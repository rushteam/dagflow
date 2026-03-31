package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/rushteam/dagflow/internal/infrastructure/auth"
	"github.com/rushteam/dagflow/internal/infrastructure/database/gen"
	infrahttp "github.com/rushteam/dagflow/internal/infrastructure/http"
)

type TokenHandler struct {
	queries *gen.Queries
	jwt     *auth.JWTManager
}

func NewTokenHandler(db gen.DBTX, jwt *auth.JWTManager) *TokenHandler {
	return &TokenHandler{
		queries: gen.New(db),
		jwt:     jwt,
	}
}

func (h *TokenHandler) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(h.jwt.Middleware)
		r.Post("/api/v1/tokens", h.createToken)
		r.Get("/api/v1/tokens", h.listTokens)
		r.Delete("/api/v1/tokens/{id}", h.revokeToken)
	})
}

type createTokenRequest struct {
	Name      string `json:"name"`
	ExpiresIn string `json:"expires_in"` // "30d", "90d", "365d", "" (never)
}

type createTokenResponse struct {
	Token string        `json:"token"`
	Info  tokenResponse `json:"info"`
}

type tokenResponse struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Prefix      string  `json:"prefix"`
	CreatedBy   int64   `json:"created_by"`
	CreatorName string  `json:"creator_name,omitempty"`
	ExpiresAt   *string `json:"expires_at,omitempty"`
	LastUsedAt  *string `json:"last_used_at,omitempty"`
	Enabled     bool    `json:"enabled"`
	CreatedAt   string  `json:"created_at"`
}

func (h *TokenHandler) createToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		infrahttp.Error(w, http.StatusUnauthorized, "未登录")
		return
	}

	var req createTokenRequest
	if err := infrahttp.DecodeJSON(r, &req); err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	if req.Name == "" {
		infrahttp.Error(w, http.StatusBadRequest, "name 不能为空")
		return
	}

	raw, err := auth.GenerateAPITokenRaw()
	if err != nil {
		infrahttp.Error(w, http.StatusInternalServerError, "生成 token 失败")
		return
	}

	var expiresAt sql.NullTime
	if req.ExpiresIn != "" {
		d, perr := parseDuration(req.ExpiresIn)
		if perr != nil {
			infrahttp.Error(w, http.StatusBadRequest, "expires_in 格式无效，示例: 30d, 90d, 365d")
			return
		}
		expiresAt = sql.NullTime{Time: time.Now().Add(d), Valid: true}
	}

	prefix := raw[:11] // "tk_" + 8 hex chars

	token, err := h.queries.CreateAPIToken(r.Context(), gen.CreateAPITokenParams{
		Name:      req.Name,
		TokenHash: auth.HashToken(raw),
		Prefix:    prefix,
		CreatedBy: userID,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		infrahttp.Error(w, http.StatusInternalServerError, "创建 token 失败")
		return
	}

	infrahttp.JSON(w, http.StatusCreated, createTokenResponse{
		Token: raw,
		Info:  toTokenResponse(token),
	})
}

func (h *TokenHandler) listTokens(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListAllAPITokens(r.Context())
	if err != nil {
		infrahttp.Error(w, http.StatusInternalServerError, "查询 token 列表失败")
		return
	}

	result := make([]tokenResponse, 0, len(rows))
	for _, row := range rows {
		resp := tokenResponse{
			ID:          row.ID,
			Name:        row.Name,
			Prefix:      row.Prefix,
			CreatedBy:   row.CreatedBy,
			CreatorName: row.CreatorName,
			Enabled:     row.Enabled,
			CreatedAt:   row.CreatedAt.Format(time.RFC3339),
		}
		if row.ExpiresAt.Valid {
			s := row.ExpiresAt.Time.Format(time.RFC3339)
			resp.ExpiresAt = &s
		}
		if row.LastUsedAt.Valid {
			s := row.LastUsedAt.Time.Format(time.RFC3339)
			resp.LastUsedAt = &s
		}
		result = append(result, resp)
	}
	infrahttp.JSON(w, http.StatusOK, result)
}

func (h *TokenHandler) revokeToken(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	if err := h.queries.RevokeAPIToken(r.Context(), id); err != nil {
		infrahttp.Error(w, http.StatusInternalServerError, "撤销 token 失败")
		return
	}

	infrahttp.JSON(w, http.StatusOK, map[string]string{"message": "token 已撤销"})
}

func toTokenResponse(t gen.ApiToken) tokenResponse {
	resp := tokenResponse{
		ID:        t.ID,
		Name:      t.Name,
		Prefix:    t.Prefix,
		CreatedBy: t.CreatedBy,
		Enabled:   t.Enabled,
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
	}
	if t.ExpiresAt.Valid {
		s := t.ExpiresAt.Time.Format(time.RFC3339)
		resp.ExpiresAt = &s
	}
	if t.LastUsedAt.Valid {
		s := t.LastUsedAt.Time.Format(time.RFC3339)
		resp.LastUsedAt = &s
	}
	return resp
}

func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}
	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	var num int
	for _, c := range numStr {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		num = num*10 + int(c-'0')
	}
	switch unit {
	case 'd':
		return time.Duration(num) * 24 * time.Hour, nil
	case 'h':
		return time.Duration(num) * time.Hour, nil
	default:
		return 0, fmt.Errorf("unsupported unit: %c", unit)
	}
}
