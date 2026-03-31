package handler

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/rushteam/dagflow/internal/infrastructure/auth"
	"github.com/rushteam/dagflow/internal/infrastructure/database/gen"
	infrahttp "github.com/rushteam/dagflow/internal/infrastructure/http"

	"github.com/go-chi/chi/v5"
)

type UserHandler struct {
	queries *gen.Queries
	jwt     *auth.JWTManager
}

func NewUserHandler(db gen.DBTX, jwt *auth.JWTManager) *UserHandler {
	return &UserHandler{
		queries: gen.New(db),
		jwt:     jwt,
	}
}

func (h *UserHandler) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(h.jwt.Middleware)
		r.Get("/api/v1/users", h.listUsers)
	})
}

func (h *UserHandler) listUsers(w http.ResponseWriter, r *http.Request) {
	_, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		infrahttp.Error(w, http.StatusUnauthorized, "未登录")
		return
	}

	ctx := r.Context()
	users, err := h.queries.ListUsers(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			infrahttp.JSON(w, http.StatusOK, []userResponse{})
			return
		}
		slog.ErrorContext(ctx, "查询用户列表失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
		return
	}

	result := make([]userResponse, 0, len(users))
	for _, u := range users {
		result = append(result, toUserResponse(u))
	}

	infrahttp.JSON(w, http.StatusOK, result)
}
