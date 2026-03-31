package handler

import (
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/mlboy/dagflow/internal/infrastructure/auth"
	"github.com/mlboy/dagflow/internal/infrastructure/database/gen"
	infrahttp "github.com/mlboy/dagflow/internal/infrastructure/http"

	"github.com/go-chi/chi/v5"
)

type AuthHandler struct {
	queries *gen.Queries
	jwt     *auth.JWTManager
}

func NewAuthHandler(db gen.DBTX, jwt *auth.JWTManager) *AuthHandler {
	return &AuthHandler{
		queries: gen.New(db),
		jwt:     jwt,
	}
}

func (h *AuthHandler) RegisterRoutes(r chi.Router) {
	r.Post("/api/v1/auth/register", h.register)
	r.Post("/api/v1/auth/login", h.login)

	r.Group(func(r chi.Router) {
		r.Use(h.jwt.Middleware)
		r.Get("/api/v1/auth/me", h.me)
	})
}

type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
}

func (h *AuthHandler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := infrahttp.DecodeJSON(r, &req); err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	if req.Username == "" || req.Password == "" {
		infrahttp.Error(w, http.StatusBadRequest, "用户名和密码不能为空")
		return
	}
	if len(req.Password) < 6 {
		infrahttp.Error(w, http.StatusBadRequest, "密码长度至少 6 位")
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("bcrypt hash 失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
		return
	}

	nickname := req.Nickname
	if nickname == "" {
		nickname = req.Username
	}

	ctx := r.Context()
	user, err := h.queries.CreateUser(ctx, gen.CreateUserParams{
		Username:     req.Username,
		PasswordHash: string(hashed),
		Nickname:     nickname,
	})
	if err != nil {
		if isDuplicateKeyError(err) {
			infrahttp.Error(w, http.StatusConflict, "用户名已存在")
			return
		}
		slog.ErrorContext(ctx, "创建用户失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "创建用户失败")
		return
	}

	token, err := h.jwt.GenerateToken(user.ID, user.Username)
	if err != nil {
		slog.ErrorContext(ctx, "生成 token 失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
		return
	}

	infrahttp.JSON(w, http.StatusCreated, map[string]any{
		"token": token,
		"user":  toUserResponse(user),
	})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := infrahttp.DecodeJSON(r, &req); err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	ctx := r.Context()
	user, err := h.queries.GetUserByUsername(ctx, req.Username)
	if err != nil {
		if err == sql.ErrNoRows {
			infrahttp.Error(w, http.StatusUnauthorized, "用户名或密码错误")
			return
		}
		slog.ErrorContext(ctx, "查询用户失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		infrahttp.Error(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	_ = h.queries.UpdateUserLastLogin(ctx, user.ID)

	token, err := h.jwt.GenerateToken(user.ID, user.Username)
	if err != nil {
		slog.ErrorContext(ctx, "生成 token 失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
		return
	}

	infrahttp.JSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  toUserResponse(user),
	})
}

func (h *AuthHandler) me(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		infrahttp.Error(w, http.StatusUnauthorized, "未登录")
		return
	}

	user, err := h.queries.GetUserByID(r.Context(), userID)
	if err != nil {
		if err == sql.ErrNoRows {
			infrahttp.Error(w, http.StatusNotFound, "用户不存在")
			return
		}
		infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
		return
	}

	infrahttp.JSON(w, http.StatusOK, toUserResponse(user))
}

type userResponse struct {
	ID        int64      `json:"id"`
	Username  string     `json:"username"`
	Nickname  string     `json:"nickname"`
	Role      string     `json:"role"`
	CreatedAt time.Time  `json:"created_at"`
	LastLogin *time.Time `json:"last_login,omitempty"`
}

func toUserResponse(u gen.User) userResponse {
	resp := userResponse{
		ID:        u.ID,
		Username:  u.Username,
		Nickname:  u.Nickname,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
	}
	if u.LastLoginAt.Valid {
		resp.LastLogin = &u.LastLoginAt.Time
	}
	return resp
}

func isDuplicateKeyError(err error) bool {
	return err != nil && containsString(err.Error(), "duplicate key", "unique constraint", "UNIQUE constraint")
}

func containsString(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
