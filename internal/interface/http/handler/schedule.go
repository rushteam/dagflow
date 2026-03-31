package handler

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/rushteam/dagflow/internal/application/scheduler"
	"github.com/rushteam/dagflow/internal/infrastructure/auth"
	"github.com/rushteam/dagflow/internal/infrastructure/database/gen"
	infrahttp "github.com/rushteam/dagflow/internal/infrastructure/http"
)

type ScheduleHandler struct {
	queries   *gen.Queries
	jwt       *auth.JWTManager
	scheduler *scheduler.Scheduler
}

func NewScheduleHandler(db gen.DBTX, jwt *auth.JWTManager, sched *scheduler.Scheduler) *ScheduleHandler {
	return &ScheduleHandler{
		queries:   gen.New(db),
		jwt:       jwt,
		scheduler: sched,
	}
}

func (h *ScheduleHandler) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(h.jwt.Middleware)
		r.Get("/api/v1/schedules", h.listSchedules)
		r.Post("/api/v1/schedules", h.createSchedule)
		r.Get("/api/v1/schedules/{id}", h.getSchedule)
		r.Put("/api/v1/schedules/{id}", h.updateSchedule)
		r.Delete("/api/v1/schedules/{id}", h.deleteSchedule)
		r.Post("/api/v1/schedules/{id}/trigger", h.triggerSchedule)
		r.Get("/api/v1/schedules/{id}/logs", h.getScheduleLogs)
	})
}

// ---- request / response ----

type scheduleRequest struct {
	Name              string          `json:"name"`
	TaskID            int64           `json:"task_id"`
	ScheduleType      string          `json:"schedule_type"`
	CronExpr          string          `json:"cron_expr"`
	RunAt             string          `json:"run_at"`
	VariableOverrides json.RawMessage `json:"variable_overrides"`
	Enabled           bool            `json:"enabled"`
}

type scheduleResponse struct {
	ID                int64           `json:"id"`
	Name              string          `json:"name"`
	TaskID            int64           `json:"task_id"`
	ScheduleType      string          `json:"schedule_type"`
	CronExpr          string          `json:"cron_expr,omitempty"`
	RunAt             *time.Time      `json:"run_at,omitempty"`
	VariableOverrides json.RawMessage `json:"variable_overrides"`
	Enabled           bool            `json:"enabled"`
	Status            string          `json:"status"`
	LastRunAt         *time.Time      `json:"last_run_at,omitempty"`
	NextRunAt         *time.Time      `json:"next_run_at,omitempty"`
	CreatedBy         *int64          `json:"created_by,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
}

func toScheduleResponse(s gen.Schedule) scheduleResponse {
	resp := scheduleResponse{
		ID:                s.ID,
		Name:              s.Name,
		TaskID:            s.TaskID,
		ScheduleType:      s.ScheduleType,
		VariableOverrides: s.VariableOverrides,
		Enabled:           s.Enabled,
		Status:            s.Status,
		CreatedAt:         s.CreatedAt,
	}
	if s.CronExpr.Valid {
		resp.CronExpr = s.CronExpr.String
	}
	if s.RunAt.Valid {
		resp.RunAt = &s.RunAt.Time
	}
	if s.LastRunAt.Valid {
		resp.LastRunAt = &s.LastRunAt.Time
	}
	if s.NextRunAt.Valid {
		resp.NextRunAt = &s.NextRunAt.Time
	}
	if s.CreatedBy.Valid {
		resp.CreatedBy = &s.CreatedBy.Int64
	}
	return resp
}

type logResponse struct {
	ID         int64      `json:"id"`
	ScheduleID int64      `json:"schedule_id"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Status     string     `json:"status"`
	ErrorMsg   string     `json:"error_msg,omitempty"`
	DurationMs *int64     `json:"duration_ms,omitempty"`
}

func toLogResponse(l gen.ScheduleLog) logResponse {
	resp := logResponse{
		ID:         l.ID,
		ScheduleID: l.ScheduleID,
		StartedAt:  l.StartedAt,
		Status:     l.Status,
	}
	if l.FinishedAt.Valid {
		resp.FinishedAt = &l.FinishedAt.Time
	}
	if l.ErrorMsg.Valid {
		resp.ErrorMsg = l.ErrorMsg.String
	}
	if l.DurationMs.Valid {
		resp.DurationMs = &l.DurationMs.Int64
	}
	return resp
}

// ---- handlers ----

func (h *ScheduleHandler) listSchedules(w http.ResponseWriter, r *http.Request) {
	list, err := h.queries.ListSchedules(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "查询调度列表失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
		return
	}
	result := make([]scheduleResponse, 0, len(list))
	for _, s := range list {
		result = append(result, toScheduleResponse(s))
	}
	infrahttp.JSON(w, http.StatusOK, result)
}

func (h *ScheduleHandler) getSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	sch, err := h.queries.GetScheduleByID(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			infrahttp.Error(w, http.StatusNotFound, "调度不存在")
			return
		}
		infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
		return
	}
	infrahttp.JSON(w, http.StatusOK, toScheduleResponse(sch))
}

func (h *ScheduleHandler) createSchedule(w http.ResponseWriter, r *http.Request) {
	var req scheduleRequest
	if err := infrahttp.DecodeJSON(r, &req); err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	if req.Name == "" || req.TaskID == 0 {
		infrahttp.Error(w, http.StatusBadRequest, "name 和 task_id 不能为空")
		return
	}

	if _, err := h.queries.GetTaskByID(r.Context(), req.TaskID); err != nil {
		if err == sql.ErrNoRows {
			infrahttp.Error(w, http.StatusBadRequest, "任务不存在")
			return
		}
		infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
		return
	}

	if req.ScheduleType != "cron" && req.ScheduleType != "once" {
		infrahttp.Error(w, http.StatusBadRequest, "schedule_type 仅支持 cron 或 once")
		return
	}

	overrides := req.VariableOverrides
	if len(overrides) == 0 {
		overrides = json.RawMessage(`[]`)
	}

	params := gen.CreateScheduleParams{
		Name:              req.Name,
		TaskID:            req.TaskID,
		ScheduleType:      req.ScheduleType,
		VariableOverrides: overrides,
		Enabled:           req.Enabled,
	}

	if req.ScheduleType == "cron" {
		if req.CronExpr == "" {
			infrahttp.Error(w, http.StatusBadRequest, "cron 类型须提供 cron_expr")
			return
		}
		if err := h.scheduler.ValidateCronExpr(req.CronExpr); err != nil {
			infrahttp.Error(w, http.StatusBadRequest, "cron 表达式无效: "+err.Error())
			return
		}
		params.CronExpr = sql.NullString{String: req.CronExpr, Valid: true}
	} else {
		if req.RunAt == "" {
			infrahttp.Error(w, http.StatusBadRequest, "once 类型须提供 run_at")
			return
		}
		t, err := time.Parse(time.RFC3339, req.RunAt)
		if err != nil {
			infrahttp.Error(w, http.StatusBadRequest, "run_at 格式无效（需 RFC3339）")
			return
		}
		params.RunAt = sql.NullTime{Time: t, Valid: true}
		params.NextRunAt = sql.NullTime{Time: t, Valid: true}
	}

	userID, _ := auth.UserIDFromContext(r.Context())
	if userID > 0 {
		params.CreatedBy = sql.NullInt64{Int64: userID, Valid: true}
	}

	ctx := r.Context()
	sch, err := h.queries.CreateSchedule(ctx, params)
	if err != nil {
		slog.ErrorContext(ctx, "创建调度失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "创建调度失败")
		return
	}

	if sch.Enabled {
		if err := h.scheduler.AddSchedule(ctx, sch); err != nil {
			slog.ErrorContext(ctx, "加入调度引擎失败", "id", sch.ID, "error", err)
		}
	}

	infrahttp.JSON(w, http.StatusCreated, toScheduleResponse(sch))
}

func (h *ScheduleHandler) updateSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	var req scheduleRequest
	if err := infrahttp.DecodeJSON(r, &req); err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	if req.TaskID > 0 {
		if _, err := h.queries.GetTaskByID(r.Context(), req.TaskID); err != nil {
			if err == sql.ErrNoRows {
				infrahttp.Error(w, http.StatusBadRequest, "任务不存在")
				return
			}
			infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
			return
		}
	}

	overrides := req.VariableOverrides
	if len(overrides) == 0 {
		overrides = json.RawMessage(`[]`)
	}

	params := gen.UpdateScheduleParams{
		ID:                id,
		Name:              req.Name,
		TaskID:            req.TaskID,
		ScheduleType:      req.ScheduleType,
		VariableOverrides: overrides,
		Enabled:           req.Enabled,
	}

	if req.ScheduleType == "cron" && req.CronExpr != "" {
		if err := h.scheduler.ValidateCronExpr(req.CronExpr); err != nil {
			infrahttp.Error(w, http.StatusBadRequest, "cron 表达式无效: "+err.Error())
			return
		}
		params.CronExpr = sql.NullString{String: req.CronExpr, Valid: true}
	}
	if req.ScheduleType == "once" && req.RunAt != "" {
		t, err := time.Parse(time.RFC3339, req.RunAt)
		if err != nil {
			infrahttp.Error(w, http.StatusBadRequest, "run_at 格式无效（需 RFC3339）")
			return
		}
		params.RunAt = sql.NullTime{Time: t, Valid: true}
		params.NextRunAt = sql.NullTime{Time: t, Valid: true}
	}

	ctx := r.Context()
	sch, err := h.queries.UpdateSchedule(ctx, params)
	if err != nil {
		if err == sql.ErrNoRows {
			infrahttp.Error(w, http.StatusNotFound, "调度不存在")
			return
		}
		slog.ErrorContext(ctx, "更新调度失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "更新调度失败")
		return
	}

	h.scheduler.RemoveSchedule(id)
	if sch.Enabled {
		if err := h.scheduler.AddSchedule(ctx, sch); err != nil {
			slog.ErrorContext(ctx, "重新加入调度引擎失败", "id", id, "error", err)
		}
	}

	infrahttp.JSON(w, http.StatusOK, toScheduleResponse(sch))
}

func (h *ScheduleHandler) deleteSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	h.scheduler.RemoveSchedule(id)

	if err := h.queries.DeleteSchedule(r.Context(), id); err != nil {
		slog.ErrorContext(r.Context(), "删除调度失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "删除调度失败")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ScheduleHandler) triggerSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	if _, err := h.queries.GetScheduleByID(r.Context(), id); err != nil {
		if err == sql.ErrNoRows {
			infrahttp.Error(w, http.StatusNotFound, "调度不存在")
			return
		}
		infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
		return
	}

	h.scheduler.TriggerSchedule(r.Context(), id)
	infrahttp.JSON(w, http.StatusOK, map[string]string{"message": "已触发执行"})
}

func (h *ScheduleHandler) getScheduleLogs(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	logs, err := h.queries.ListScheduleLogs(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "查询执行日志失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
		return
	}
	result := make([]logResponse, 0, len(logs))
	for _, l := range logs {
		result = append(result, toLogResponse(l))
	}
	infrahttp.JSON(w, http.StatusOK, result)
}

func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}
