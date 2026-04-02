package handler

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/rushteam/dagflow/internal/application/dag"
	"github.com/rushteam/dagflow/internal/application/executor"
	appscheduler "github.com/rushteam/dagflow/internal/application/scheduler"
	"github.com/rushteam/dagflow/internal/infrastructure/auth"
	"github.com/rushteam/dagflow/internal/infrastructure/database/gen"
	infrahttp "github.com/rushteam/dagflow/internal/infrastructure/http"
)

type TaskHandler struct {
	queries   *gen.Queries
	jwt       *auth.JWTManager
	kinds     *executor.Registry
	scheduler *appscheduler.Scheduler
}

func NewTaskHandler(db gen.DBTX, jwt *auth.JWTManager, kinds *executor.Registry, sched *appscheduler.Scheduler) *TaskHandler {
	return &TaskHandler{
		queries:   gen.New(db),
		jwt:       jwt,
		kinds:     kinds,
		scheduler: sched,
	}
}

func (h *TaskHandler) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(h.jwt.Middleware)
		r.Get("/api/v1/kinds", h.listKinds)
		r.Get("/api/v1/tasks", h.listTasks)
		r.Post("/api/v1/tasks", h.createTask)
		r.Get("/api/v1/tasks/{id}", h.getTask)
		r.Put("/api/v1/tasks/{id}", h.updateTask)
		r.Delete("/api/v1/tasks/{id}", h.deleteTask)
		r.Post("/api/v1/tasks/{id}/run", h.runTask)
		r.Get("/api/v1/tasks/{id}/runs", h.listTaskRuns)
		r.Get("/api/v1/task-runs", h.listAllTaskRuns)
		r.Get("/api/v1/task-runs/{id}", h.getTaskRunDetail)
		r.Post("/api/v1/task-runs/{id}/cancel", h.cancelTaskRun)
		r.Get("/api/v1/task-runs/{id}/children", h.listChildRuns)
	})
}

// ---- kinds ----

type kindResponse struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	PayloadHint string `json:"payload_hint"`
}

func (h *TaskHandler) listKinds(w http.ResponseWriter, _ *http.Request) {
	kinds := h.kinds.List()
	result := make([]kindResponse, 0, len(kinds))
	for _, k := range kinds {
		result = append(result, kindResponse{
			Name:        k.Name,
			Label:       k.Label,
			PayloadHint: k.PayloadHint,
		})
	}
	infrahttp.JSON(w, http.StatusOK, result)
}

// ---- tasks CRUD ----

type taskSchedulePayload struct {
	Name              string          `json:"name"`
	ScheduleType      string          `json:"schedule_type"`
	CronExpr          string          `json:"cron_expr"`
	RunAt             string          `json:"run_at"`
	VariableOverrides json.RawMessage `json:"variable_overrides"`
	Enabled           bool            `json:"enabled"`
}

type taskRequest struct {
	Name      string               `json:"name"`
	Label     string               `json:"label"`
	Kind      string               `json:"kind"`
	Payload   json.RawMessage      `json:"payload"`
	Variables json.RawMessage      `json:"variables"`
	Enabled   bool                 `json:"enabled"`
	Schedule  *taskSchedulePayload `json:"schedule,omitempty"`
}

type taskResponse struct {
	ID         int64           `json:"id"`
	Name       string          `json:"name"`
	Label      string          `json:"label"`
	Kind       string          `json:"kind"`
	Payload    json.RawMessage `json:"payload"`
	Variables  json.RawMessage `json:"variables"`
	Enabled    bool            `json:"enabled"`
	CreatedBy  *int64          `json:"created_by,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	ScheduleID *int64          `json:"schedule_id,omitempty"`
}

func taskScheduleToScheduleRequest(p *taskSchedulePayload, taskID int64) scheduleRequest {
	return scheduleRequest{
		Name:              strings.TrimSpace(p.Name),
		TaskID:            taskID,
		ScheduleType:      strings.TrimSpace(p.ScheduleType),
		CronExpr:          strings.TrimSpace(p.CronExpr),
		RunAt:             strings.TrimSpace(p.RunAt),
		VariableOverrides: p.VariableOverrides,
		Enabled:           p.Enabled,
	}
}

func toTaskResponse(t gen.Task) taskResponse {
	resp := taskResponse{
		ID:        t.ID,
		Name:      t.Name,
		Label:     t.Label,
		Kind:      t.Kind,
		Payload:   t.Payload,
		Variables: t.Variables,
		Enabled:   t.Enabled,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
	if t.CreatedBy.Valid {
		resp.CreatedBy = &t.CreatedBy.Int64
	}
	return resp
}

func (h *TaskHandler) listTasks(w http.ResponseWriter, r *http.Request) {
	list, err := h.queries.ListTasks(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "查询任务列表失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
		return
	}
	result := make([]taskResponse, 0, len(list))
	for _, t := range list {
		result = append(result, toTaskResponse(t))
	}
	infrahttp.JSON(w, http.StatusOK, result)
}

func (h *TaskHandler) getTask(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	task, err := h.queries.GetTaskByID(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			infrahttp.Error(w, http.StatusNotFound, "任务不存在")
			return
		}
		infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
		return
	}
	infrahttp.JSON(w, http.StatusOK, toTaskResponse(task))
}

func (h *TaskHandler) createTask(w http.ResponseWriter, r *http.Request) {
	var req taskRequest
	if err := infrahttp.DecodeJSON(r, &req); err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	if req.Name == "" || req.Kind == "" {
		infrahttp.Error(w, http.StatusBadRequest, "name 和 kind 不能为空")
		return
	}

	if _, ok := h.kinds.Get(req.Kind); !ok {
		infrahttp.Error(w, http.StatusBadRequest, "不支持的 kind: "+req.Kind)
		return
	}

	payload := req.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	if req.Kind == "dag" {
		if err := dag.ValidatePayload(r.Context(), h.queries, 0, payload); err != nil {
			infrahttp.Error(w, http.StatusBadRequest, "DAG 校验失败: "+err.Error())
			return
		}
	}

	if req.Schedule != nil && strings.TrimSpace(req.Schedule.Name) != "" {
		sr := taskScheduleToScheduleRequest(req.Schedule, 0)
		if _, err := scheduleCreateParamsFromRequest(sr, h.scheduler); err != nil {
			infrahttp.Error(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	variables := req.Variables
	if len(variables) == 0 {
		variables = json.RawMessage(`[]`)
	}

	userID, _ := auth.UserIDFromContext(r.Context())
	var createdBy sql.NullInt64
	if userID > 0 {
		createdBy = sql.NullInt64{Int64: userID, Valid: true}
	}

	task, err := h.queries.CreateTask(r.Context(), gen.CreateTaskParams{
		Name:      req.Name,
		Label:     req.Label,
		Kind:      req.Kind,
		Payload:   payload,
		Variables: variables,
		Enabled:   req.Enabled,
		CreatedBy: createdBy,
		Callback:  json.RawMessage(`null`),
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "创建任务失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "创建任务失败")
		return
	}

	ctx := r.Context()
	resp := toTaskResponse(task)
	if req.Schedule != nil && strings.TrimSpace(req.Schedule.Name) != "" {
		sr := taskScheduleToScheduleRequest(req.Schedule, task.ID)
		params, err := scheduleCreateParamsFromRequest(sr, h.scheduler)
		if err != nil {
			slog.ErrorContext(ctx, "组装调度参数失败", "error", err)
			infrahttp.Error(w, http.StatusInternalServerError, "创建调度失败")
			return
		}
		if userID > 0 {
			params.CreatedBy = sql.NullInt64{Int64: userID, Valid: true}
		}
		sch, err := h.queries.CreateSchedule(ctx, params)
		if err != nil {
			slog.ErrorContext(ctx, "创建调度失败", "error", err)
			infrahttp.Error(w, http.StatusInternalServerError, "任务已创建，但调度创建失败")
			return
		}
		if sch.Enabled {
			if err := h.scheduler.AddSchedule(ctx, sch); err != nil {
				slog.ErrorContext(ctx, "加入调度引擎失败", "id", sch.ID, "error", err)
			}
		}
		sid := sch.ID
		resp.ScheduleID = &sid
	}

	infrahttp.JSON(w, http.StatusCreated, resp)
}

func (h *TaskHandler) updateTask(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	var req taskRequest
	if err := infrahttp.DecodeJSON(r, &req); err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	if req.Kind != "" {
		if _, ok := h.kinds.Get(req.Kind); !ok {
			infrahttp.Error(w, http.StatusBadRequest, "不支持的 kind: "+req.Kind)
			return
		}
	}

	payload := req.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	if req.Kind == "dag" {
		if err := dag.ValidatePayload(r.Context(), h.queries, id, payload); err != nil {
			infrahttp.Error(w, http.StatusBadRequest, "DAG 校验失败: "+err.Error())
			return
		}
	}

	variables := req.Variables
	if len(variables) == 0 {
		variables = json.RawMessage(`[]`)
	}

	task, err := h.queries.UpdateTask(r.Context(), gen.UpdateTaskParams{
		ID:        id,
		Name:      req.Name,
		Label:     req.Label,
		Kind:      req.Kind,
		Payload:   payload,
		Variables: variables,
		Enabled:   req.Enabled,
		Callback:  json.RawMessage(`null`),
	})
	if err != nil {
		if err == sql.ErrNoRows {
			infrahttp.Error(w, http.StatusNotFound, "任务不存在")
			return
		}
		slog.ErrorContext(r.Context(), "更新任务失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "更新任务失败")
		return
	}

	infrahttp.JSON(w, http.StatusOK, toTaskResponse(task))
}

func (h *TaskHandler) deleteTask(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	if err := h.queries.DeleteTask(r.Context(), id); err != nil {
		slog.ErrorContext(r.Context(), "删除任务失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "删除任务失败")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---- run / runs ----

type runTaskRequest struct {
	Variables map[string]string `json:"variables"`
}

func (h *TaskHandler) runTask(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	var req runTaskRequest
	_ = infrahttp.DecodeJSON(r, &req)

	task, err := h.queries.GetTaskByID(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			infrahttp.Error(w, http.StatusNotFound, "任务不存在")
			return
		}
		infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
		return
	}

	if _, ok := h.kinds.Get(task.Kind); !ok {
		infrahttp.Error(w, http.StatusBadRequest, "任务 kind 未注册: "+task.Kind)
		return
	}

	userID, _ := auth.UserIDFromContext(r.Context())
	h.scheduler.RunTask(r.Context(), id, userID, req.Variables)

	infrahttp.JSON(w, http.StatusOK, map[string]string{"message": "任务已触发"})
}

type taskRunResponse struct {
	ID          int64   `json:"id"`
	TaskID      int64   `json:"task_id"`
	TriggerType string  `json:"trigger_type"`
	TriggerID   *int64  `json:"trigger_id,omitempty"`
	TriggeredBy *int64  `json:"triggered_by,omitempty"`
	Status      string  `json:"status"`
	StartedAt   string  `json:"started_at"`
	FinishedAt  *string `json:"finished_at,omitempty"`
	DurationMs  *int64  `json:"duration_ms,omitempty"`
	ErrorMsg    *string `json:"error_msg,omitempty"`
	Output      *string `json:"output,omitempty"`
}

func toTaskRunResponse(r gen.TaskRun) taskRunResponse {
	resp := taskRunResponse{
		ID:          r.ID,
		TaskID:      r.TaskID,
		TriggerType: r.TriggerType,
		Status:      r.Status,
		StartedAt:   r.StartedAt.Format(time.RFC3339),
	}
	if r.TriggerID.Valid {
		resp.TriggerID = &r.TriggerID.Int64
	}
	if r.TriggeredBy.Valid {
		resp.TriggeredBy = &r.TriggeredBy.Int64
	}
	if r.FinishedAt.Valid {
		s := r.FinishedAt.Time.Format(time.RFC3339)
		resp.FinishedAt = &s
	}
	if r.DurationMs.Valid {
		resp.DurationMs = &r.DurationMs.Int64
	}
	if r.ErrorMsg.Valid {
		resp.ErrorMsg = &r.ErrorMsg.String
	}
	if r.Output.Valid {
		resp.Output = &r.Output.String
	}
	return resp
}

func (h *TaskHandler) listTaskRuns(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	runs, err := h.queries.ListTaskRunsByTaskID(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "查询运行日志失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
		return
	}

	result := make([]taskRunResponse, 0, len(runs))
	for _, run := range runs {
		result = append(result, toTaskRunResponse(run))
	}
	infrahttp.JSON(w, http.StatusOK, result)
}

// ---- all runs (paged) ----

type allTaskRunResponse struct {
	taskRunResponse
	TaskName  string `json:"task_name"`
	TaskLabel string `json:"task_label"`
	TaskKind  string `json:"task_kind"`
}

type pagedTaskRunsResponse struct {
	Total int64                `json:"total"`
	Page  int                  `json:"page"`
	Size  int                  `json:"size"`
	Items []allTaskRunResponse `json:"items"`
}

func (h *TaskHandler) listAllTaskRuns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(q.Get("size"))
	if size < 1 || size > 100 {
		size = 15
	}
	offset := (page - 1) * size

	filterArgs := buildRunFilters(q)

	total, err := h.queries.CountTaskRuns(r.Context(), gen.CountTaskRunsParams{
		TaskName:  filterArgs.TaskName,
		TaskLabel: filterArgs.TaskLabel,
		RunID:     filterArgs.RunID,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "统计运行日志总数失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
		return
	}

	rows, err := h.queries.ListTaskRunsPaged(r.Context(), gen.ListTaskRunsPagedParams{
		Limit:     int32(size),
		Offset:    int32(offset),
		TaskName:  filterArgs.TaskName,
		TaskLabel: filterArgs.TaskLabel,
		RunID:     filterArgs.RunID,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "查询运行日志分页失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
		return
	}

	items := make([]allTaskRunResponse, 0, len(rows))
	for _, row := range rows {
		resp := allTaskRunResponse{
			taskRunResponse: taskRunResponse{
				ID:          row.ID,
				TaskID:      row.TaskID,
				TriggerType: row.TriggerType,
				Status:      row.Status,
				StartedAt:   row.StartedAt.Format(time.RFC3339),
			},
			TaskName:  row.TaskName,
			TaskLabel: row.TaskLabel,
			TaskKind:  row.TaskKind,
		}
		if row.TriggerID.Valid {
			resp.TriggerID = &row.TriggerID.Int64
		}
		if row.TriggeredBy.Valid {
			resp.TriggeredBy = &row.TriggeredBy.Int64
		}
		if row.FinishedAt.Valid {
			s := row.FinishedAt.Time.Format(time.RFC3339)
			resp.FinishedAt = &s
		}
		if row.DurationMs.Valid {
			resp.DurationMs = &row.DurationMs.Int64
		}
		if row.ErrorMsg.Valid {
			resp.ErrorMsg = &row.ErrorMsg.String
		}
		items = append(items, resp)
	}

	infrahttp.JSON(w, http.StatusOK, pagedTaskRunsResponse{
		Total: total,
		Page:  page,
		Size:  size,
		Items: items,
	})
}

type runFilterArgs struct {
	TaskName  sql.NullString
	TaskLabel sql.NullString
	RunID     sql.NullInt64
}

func buildRunFilters(q interface{ Get(string) string }) runFilterArgs {
	var f runFilterArgs
	if v := q.Get("task_name"); v != "" {
		f.TaskName = sql.NullString{String: v, Valid: true}
	}
	if v := q.Get("task_label"); v != "" {
		f.TaskLabel = sql.NullString{String: v, Valid: true}
	}
	if v := q.Get("run_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.RunID = sql.NullInt64{Int64: id, Valid: true}
		}
	}
	return f
}

// ---- run detail ----

func (h *TaskHandler) getTaskRunDetail(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	row, err := h.queries.GetTaskRunDetail(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			infrahttp.Error(w, http.StatusNotFound, "运行记录不存在")
			return
		}
		infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
		return
	}

	resp := allTaskRunResponse{
		taskRunResponse: taskRunResponse{
			ID:          row.ID,
			TaskID:      row.TaskID,
			TriggerType: row.TriggerType,
			Status:      row.Status,
			StartedAt:   row.StartedAt.Format(time.RFC3339),
		},
		TaskName:  row.TaskName,
		TaskLabel: row.TaskLabel,
		TaskKind:  row.TaskKind,
	}
	if row.TriggerID.Valid {
		resp.TriggerID = &row.TriggerID.Int64
	}
	if row.TriggeredBy.Valid {
		resp.TriggeredBy = &row.TriggeredBy.Int64
	}
	if row.FinishedAt.Valid {
		s := row.FinishedAt.Time.Format(time.RFC3339)
		resp.FinishedAt = &s
	}
	if row.DurationMs.Valid {
		resp.DurationMs = &row.DurationMs.Int64
	}
	if row.ErrorMsg.Valid {
		resp.ErrorMsg = &row.ErrorMsg.String
	}
	if row.Output.Valid {
		resp.Output = &row.Output.String
	}
	infrahttp.JSON(w, http.StatusOK, resp)
}

// ---- cancel run ----

func (h *TaskHandler) cancelTaskRun(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	run, err := h.queries.GetTaskRunByID(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			infrahttp.Error(w, http.StatusNotFound, "运行记录不存在")
			return
		}
		infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
		return
	}

	if run.Status != "running" {
		infrahttp.Error(w, http.StatusBadRequest, "只能取消运行中的任务")
		return
	}

	if !h.scheduler.CancelRun(id) {
		infrahttp.Error(w, http.StatusBadRequest, "任务不在当前实例运行中（可能已结束）")
		return
	}

	infrahttp.JSON(w, http.StatusOK, map[string]string{"message": "已发送取消信号"})
}

// ---- child runs (DAG) ----

type childRunResponse struct {
	allTaskRunResponse
	ParentRunID *int64 `json:"parent_run_id,omitempty"`
}

func (h *TaskHandler) listChildRuns(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		infrahttp.Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	rows, err := h.queries.ListChildRuns(r.Context(), sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		slog.ErrorContext(r.Context(), "查询 DAG 子任务失败", "error", err)
		infrahttp.Error(w, http.StatusInternalServerError, "内部错误")
		return
	}

	result := make([]childRunResponse, 0, len(rows))
	for _, row := range rows {
		resp := childRunResponse{
			allTaskRunResponse: allTaskRunResponse{
				taskRunResponse: taskRunResponse{
					ID:          row.ID,
					TaskID:      row.TaskID,
					TriggerType: row.TriggerType,
					Status:      row.Status,
					StartedAt:   row.StartedAt.Format(time.RFC3339),
				},
				TaskName:  row.TaskName,
				TaskLabel: row.TaskLabel,
				TaskKind:  row.TaskKind,
			},
		}
		if row.TriggerID.Valid {
			resp.TriggerID = &row.TriggerID.Int64
		}
		if row.TriggeredBy.Valid {
			resp.TriggeredBy = &row.TriggeredBy.Int64
		}
		if row.ParentRunID.Valid {
			resp.ParentRunID = &row.ParentRunID.Int64
		}
		if row.FinishedAt.Valid {
			s := row.FinishedAt.Time.Format(time.RFC3339)
			resp.FinishedAt = &s
		}
		if row.DurationMs.Valid {
			resp.DurationMs = &row.DurationMs.Int64
		}
		if row.ErrorMsg.Valid {
			resp.ErrorMsg = &row.ErrorMsg.String
		}
		result = append(result, resp)
	}
	infrahttp.JSON(w, http.StatusOK, result)
}
