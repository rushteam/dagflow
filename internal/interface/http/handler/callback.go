package handler

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/rushteam/dagflow/internal/application/callback"
	"github.com/rushteam/dagflow/internal/infrastructure/auth"
	"github.com/rushteam/dagflow/internal/infrastructure/database/gen"
	infrahttp "github.com/rushteam/dagflow/internal/infrastructure/http"
)

type CallbackHandler struct {
	queries *gen.Queries
	jwt     *auth.JWTManager
}

func NewCallbackHandler(db gen.DBTX, jwt *auth.JWTManager) *CallbackHandler {
	return &CallbackHandler{queries: gen.New(db), jwt: jwt}
}

func (h *CallbackHandler) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(h.jwt.Middleware)
		r.Get("/api/v1/callbacks", h.list)
		r.Post("/api/v1/callbacks", h.create)
		r.Get("/api/v1/callbacks/{id}", h.get)
		r.Put("/api/v1/callbacks/{id}", h.update)
		r.Delete("/api/v1/callbacks/{id}", h.delete)
		r.Get("/api/v1/callback-vars", h.listVars)
	})
}

type callbackRequest struct {
	Name         string              `json:"name"`
	URL          string              `json:"url"`
	Events       []string            `json:"events"`
	Headers      map[string]string   `json:"headers"`
	BodyTemplate string              `json:"body_template"`
	MatchMode    string              `json:"match_mode"`
	MatchRules   callback.MatchRules `json:"match_rules"`
	Enabled      bool                `json:"enabled"`
}

type callbackResponse struct {
	ID           int64               `json:"id"`
	Name         string              `json:"name"`
	URL          string              `json:"url"`
	Events       []string            `json:"events"`
	Headers      map[string]string   `json:"headers"`
	BodyTemplate string              `json:"body_template"`
	MatchMode    string              `json:"match_mode"`
	MatchRules   callback.MatchRules `json:"match_rules"`
	Enabled      bool                `json:"enabled"`
	CreatedBy    *int64              `json:"created_by,omitempty"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
}

func toCallbackResponse(c gen.Callback) callbackResponse {
	resp := callbackResponse{
		ID:           c.ID,
		Name:         c.Name,
		URL:          c.Url,
		BodyTemplate: c.BodyTemplate,
		MatchMode:    c.MatchMode,
		Enabled:      c.Enabled,
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
	}
	if c.CreatedBy.Valid {
		resp.CreatedBy = &c.CreatedBy.Int64
	}

	var events []string
	if json.Unmarshal(c.Events, &events) == nil {
		resp.Events = events
	}
	if resp.Events == nil {
		resp.Events = []string{}
	}

	var headers map[string]string
	if json.Unmarshal(c.Headers, &headers) == nil {
		resp.Headers = headers
	}
	if resp.Headers == nil {
		resp.Headers = map[string]string{}
	}

	var mr callback.MatchRules
	if json.Unmarshal(c.MatchRules, &mr) == nil {
		resp.MatchRules = mr
	}

	return resp
}

func (h *CallbackHandler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.queries.ListCallbacks(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "查询回调列表失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
		return
	}
	result := make([]callbackResponse, 0, len(list))
	for _, c := range list {
		result = append(result, toCallbackResponse(c))
	}
	infrahttp.JSON(w, http.StatusOK, result)
}

func (h *CallbackHandler) get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	cb, err := h.queries.GetCallbackByID(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			infrahttp.Error(w, http.StatusNotFound, "回调不存在")
			return
		}
		infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
		return
	}
	infrahttp.JSON(w, http.StatusOK, toCallbackResponse(cb))
}

func (h *CallbackHandler) create(w http.ResponseWriter, r *http.Request) {
	var req callbackRequest
	if err := infrahttp.DecodeJSON(r, &req); err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.Name == "" || req.URL == "" {
		infrahttp.Error(w, http.StatusBadRequest, "name 和 url 不能为空")
		return
	}
	if req.MatchMode == "" {
		req.MatchMode = "all"
	}
	if req.MatchMode != "all" && req.MatchMode != "selected" {
		infrahttp.Error(w, http.StatusBadRequest, "match_mode 只能是 all 或 selected")
		return
	}
	if req.MatchMode == "selected" && req.MatchRules.Expr != "" {
		if err := callback.ValidateExpr(req.MatchRules.Expr); err != nil {
			infrahttp.Error(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	eventsJSON, _ := json.Marshal(defaultEvents(req.Events))
	headersJSON, _ := json.Marshal(defaultHeaders(req.Headers))
	matchRulesJSON, _ := json.Marshal(req.MatchRules)

	userID, _ := auth.UserIDFromContext(r.Context())
	var createdBy sql.NullInt64
	if userID > 0 {
		createdBy = sql.NullInt64{Int64: userID, Valid: true}
	}

	cb, err := h.queries.CreateCallback(r.Context(), gen.CreateCallbackParams{
		Name:         req.Name,
		Url:          req.URL,
		Events:       eventsJSON,
		Headers:      headersJSON,
		BodyTemplate: req.BodyTemplate,
		MatchMode:    req.MatchMode,
		MatchRules:   matchRulesJSON,
		Enabled:      req.Enabled,
		CreatedBy:    createdBy,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "创建回调失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "创建回调失败")
		return
	}
	infrahttp.JSON(w, http.StatusCreated, toCallbackResponse(cb))
}

func (h *CallbackHandler) update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	var req callbackRequest
	if err := infrahttp.DecodeJSON(r, &req); err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.Name == "" || req.URL == "" {
		infrahttp.Error(w, http.StatusBadRequest, "name 和 url 不能为空")
		return
	}
	if req.MatchMode == "" {
		req.MatchMode = "all"
	}
	if req.MatchMode == "selected" && req.MatchRules.Expr != "" {
		if err := callback.ValidateExpr(req.MatchRules.Expr); err != nil {
			infrahttp.Error(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	eventsJSON, _ := json.Marshal(defaultEvents(req.Events))
	headersJSON, _ := json.Marshal(defaultHeaders(req.Headers))
	matchRulesJSON, _ := json.Marshal(req.MatchRules)

	cb, err := h.queries.UpdateCallback(r.Context(), gen.UpdateCallbackParams{
		ID:           id,
		Name:         req.Name,
		Url:          req.URL,
		Events:       eventsJSON,
		Headers:      headersJSON,
		BodyTemplate: req.BodyTemplate,
		MatchMode:    req.MatchMode,
		MatchRules:   matchRulesJSON,
		Enabled:      req.Enabled,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			infrahttp.Error(w, http.StatusNotFound, "回调不存在")
			return
		}
		slog.ErrorContext(r.Context(), "更新回调失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "更新回调失败")
		return
	}
	infrahttp.JSON(w, http.StatusOK, toCallbackResponse(cb))
}

func (h *CallbackHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	if err := h.queries.DeleteCallback(r.Context(), id); err != nil {
		slog.ErrorContext(r.Context(), "删除回调失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "删除回调失败")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CallbackHandler) listVars(w http.ResponseWriter, _ *http.Request) {
	infrahttp.JSON(w, http.StatusOK, callback.BuiltinVars())
}

func defaultEvents(events []string) []string {
	if len(events) == 0 {
		return []string{"success", "failed", "cancelled"}
	}
	return events
}

func defaultHeaders(headers map[string]string) map[string]string {
	if headers == nil {
		return map[string]string{}
	}
	return headers
}
