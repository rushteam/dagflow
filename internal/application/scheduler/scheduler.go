package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/mlboy/dagflow/internal/application/dag"
	"github.com/mlboy/dagflow/internal/application/executor"
	"github.com/mlboy/dagflow/internal/application/varfunc"
	"github.com/mlboy/dagflow/internal/application/worker"
	"github.com/mlboy/dagflow/internal/infrastructure/database/gen"
)

// Scheduler 负责调度编排：决定"何时跑、跑什么"，然后委派给 Worker 执行。
// 它管理 cron 调度、运行记录持久化、取消信号，但不直接执行任务。
type Scheduler struct {
	cron    *cron.Cron
	parser  cron.Parser
	worker  worker.Worker
	kinds   *executor.Registry
	queries *gen.Queries
	db      *sql.DB

	mu      sync.Mutex
	entries map[int64]cron.EntryID // schedule.id → cron entry

	runMu   sync.Mutex
	running map[int64]context.CancelFunc // task_run.id → cancel
}

func New(db *sql.DB, kinds *executor.Registry, w worker.Worker) *Scheduler {
	parser := cron.NewParser(
		cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)
	return &Scheduler{
		parser:  parser,
		worker:  w,
		kinds:   kinds,
		queries: gen.New(db),
		db:      db,
		entries: make(map[int64]cron.EntryID),
		running: make(map[int64]context.CancelFunc),
	}
}

func (s *Scheduler) Kinds() *executor.Registry { return s.kinds }

// Start 创建新的 cron 实例，从 DB 加载所有 enabled 的调度并启动。
// 支持 Stop 后重新调用 Start（Leader 切换场景）。
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	s.cron = cron.New(cron.WithParser(s.parser), cron.WithChain(cron.Recover(cron.DefaultLogger)))
	s.entries = make(map[int64]cron.EntryID)
	s.mu.Unlock()

	schedules, err := s.queries.GetEnabledSchedules(ctx)
	if err != nil {
		return err
	}

	for _, sch := range schedules {
		if sch.ScheduleType == "once" && sch.RunAt.Valid && !sch.RunAt.Time.After(time.Now()) {
			slog.InfoContext(ctx, "补执行过期的一次性调度", "id", sch.ID, "name", sch.Name)
			go s.executeSchedule(context.Background(), sch.ID)
		} else {
			if err := s.addToCron(ctx, sch); err != nil {
				slog.ErrorContext(ctx, "加载调度失败", "id", sch.ID, "name", sch.Name, "error", err)
			}
		}
	}

	s.cron.Start()
	slog.InfoContext(ctx, "调度引擎已启动", "loaded", len(schedules))
	return nil
}

// Stop 停止 cron 引擎并等待运行中的 job 完成。
func (s *Scheduler) Stop() {
	s.mu.Lock()
	c := s.cron
	s.mu.Unlock()
	if c != nil {
		stopCtx := c.Stop()
		<-stopCtx.Done()
		slog.Info("调度引擎已停止")
	}
}

// AddSchedule 将一条调度记录加入 cron（API 创建/更新后调用）。
func (s *Scheduler) AddSchedule(ctx context.Context, sch gen.Schedule) error {
	return s.addToCron(ctx, sch)
}

// RemoveSchedule 从 cron 移除一条调度。
func (s *Scheduler) RemoveSchedule(scheduleID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if eid, ok := s.entries[scheduleID]; ok {
		if s.cron != nil {
			s.cron.Remove(eid)
		}
		delete(s.entries, scheduleID)
	}
}

// TriggerSchedule 手动立即触发一次调度。
func (s *Scheduler) TriggerSchedule(_ context.Context, scheduleID int64) {
	go s.executeSchedule(context.Background(), scheduleID)
}

// RunTask 手动执行一个任务（非调度触发），写入 task_runs。
// userID 为触发用户，0 表示系统触发。vars 为运行时变量替换值。
func (s *Scheduler) RunTask(_ context.Context, taskID int64, userID int64, vars map[string]string) {
	go s.dispatch(context.Background(), taskID, "manual", sql.NullInt64{}, sql.NullInt64{Int64: userID, Valid: userID > 0}, sql.NullInt64{}, vars)
}

// RunTaskSync 同步执行任务，供 DAG 执行器调用。
func (s *Scheduler) RunTaskSync(ctx context.Context, taskID int64, triggerType string, parentRunID int64) error {
	return s.dispatch(ctx, taskID, triggerType, sql.NullInt64{}, sql.NullInt64{}, sql.NullInt64{Int64: parentRunID, Valid: parentRunID > 0}, nil)
}

// CancelRun 取消正在运行的任务。
func (s *Scheduler) CancelRun(runID int64) bool {
	s.runMu.Lock()
	cancel, ok := s.running[runID]
	s.runMu.Unlock()
	if ok {
		cancel()
		slog.Info("任务已发送取消信号", "run_id", runID)
	}
	return ok
}

// ValidateCronExpr 校验 cron 表达式是否合法。
func (s *Scheduler) ValidateCronExpr(expr string) error {
	_, err := s.parser.Parse(expr)
	return err
}

// ---- internal: cron 管理 ----

type onceSchedule struct{ At time.Time }

func (o onceSchedule) Next(t time.Time) time.Time {
	if o.At.After(t) {
		return o.At
	}
	return time.Time{}
}

func (s *Scheduler) addToCron(ctx context.Context, sch gen.Schedule) error {
	var schedule cron.Schedule

	switch sch.ScheduleType {
	case "cron":
		if !sch.CronExpr.Valid || sch.CronExpr.String == "" {
			slog.WarnContext(ctx, "cron 调度缺少表达式，跳过", "id", sch.ID)
			return nil
		}
		parsed, err := s.parser.Parse(sch.CronExpr.String)
		if err != nil {
			return err
		}
		schedule = parsed
	case "once":
		if !sch.RunAt.Valid {
			slog.WarnContext(ctx, "一次性调度缺少 run_at，跳过", "id", sch.ID)
			return nil
		}
		schedule = onceSchedule{At: sch.RunAt.Time}
	default:
		slog.WarnContext(ctx, "未知调度类型", "id", sch.ID, "type", sch.ScheduleType)
		return nil
	}

	entryID := s.cron.Schedule(schedule, &scheduleJob{scheduler: s, scheduleID: sch.ID})

	s.mu.Lock()
	s.entries[sch.ID] = entryID
	s.mu.Unlock()

	next := schedule.Next(time.Now())
	_ = s.queries.UpdateScheduleExecution(ctx, gen.UpdateScheduleExecutionParams{
		ID:        sch.ID,
		Status:    "idle",
		LastRunAt: sch.LastRunAt,
		NextRunAt: sql.NullTime{Time: next, Valid: !next.IsZero()},
	})

	slog.InfoContext(ctx, "调度已加载",
		"id", sch.ID, "name", sch.Name, "type", sch.ScheduleType, "next_run", next)
	return nil
}

type scheduleJob struct {
	scheduler  *Scheduler
	scheduleID int64
}

func (j *scheduleJob) Run() {
	j.scheduler.executeSchedule(context.Background(), j.scheduleID)
}

// ---- internal: 调度执行 ----

func (s *Scheduler) executeSchedule(ctx context.Context, scheduleID int64) {
	sch, err := s.queries.GetScheduleByID(ctx, scheduleID)
	if err != nil {
		slog.ErrorContext(ctx, "执行调度失败：查询记录出错", "id", scheduleID, "error", err)
		return
	}

	_ = s.queries.UpdateScheduleExecution(ctx, gen.UpdateScheduleExecutionParams{
		ID:        scheduleID,
		Status:    "running",
		LastRunAt: sql.NullTime{Time: time.Now(), Valid: true},
		NextRunAt: sch.NextRunAt,
	})

	vars, varErr := varfunc.Resolve(sch.VariableOverrides, time.Now())
	if varErr != nil {
		slog.ErrorContext(ctx, "调度变量解析失败", "id", scheduleID, "error", varErr)
	}

	taskErr := s.dispatch(ctx, sch.TaskID, "schedule",
		sql.NullInt64{Int64: scheduleID, Valid: true}, sql.NullInt64{}, sql.NullInt64{}, vars)

	finalStatus := "idle"
	var nextRun sql.NullTime

	if sch.ScheduleType == "once" {
		finalStatus = "completed"
		_ = s.queries.SetScheduleEnabled(ctx, gen.SetScheduleEnabledParams{ID: scheduleID, Enabled: false})
		s.RemoveSchedule(scheduleID)
	} else if sch.CronExpr.Valid {
		parsed, perr := s.parser.Parse(sch.CronExpr.String)
		if perr == nil {
			next := parsed.Next(time.Now())
			nextRun = sql.NullTime{Time: next, Valid: true}
		}
	}
	if taskErr != nil {
		finalStatus = "failed"
	}

	_ = s.queries.UpdateScheduleExecution(ctx, gen.UpdateScheduleExecutionParams{
		ID:        scheduleID,
		Status:    finalStatus,
		LastRunAt: sql.NullTime{Time: time.Now(), Valid: true},
		NextRunAt: nextRun,
	})
}

// ---- internal: 任务分发（调度 → 执行） ----

// dispatch 是核心编排逻辑：加载任务、创建运行记录、准备上下文、委派给 Worker 执行、持久化结果。
func (s *Scheduler) dispatch(ctx context.Context, taskID int64, triggerType string, triggerID, triggeredBy, parentRunID sql.NullInt64, vars map[string]string) error {
	// ① 加载任务定义
	task, err := s.queries.GetTaskByID(ctx, taskID)
	if err != nil {
		slog.ErrorContext(ctx, "执行任务失败：查询任务出错", "task_id", taskID, "error", err)
		return err
	}

	if _, ok := s.kinds.Get(task.Kind); !ok {
		slog.ErrorContext(ctx, "执行任务失败：kind 未注册", "task_id", taskID, "kind", task.Kind)
		return fmt.Errorf("kind %q 未注册", task.Kind)
	}

	// ② 创建运行记录
	run, err := s.queries.CreateTaskRun(ctx, gen.CreateTaskRunParams{
		TaskID:      taskID,
		TriggerType: triggerType,
		TriggerID:   triggerID,
		TriggeredBy: triggeredBy,
		ParentRunID: parentRunID,
		Status:      "running",
		StartedAt:   time.Now(),
	})
	if err != nil {
		slog.ErrorContext(ctx, "创建运行记录失败", "task_id", taskID, "error", err)
	}

	// ③ 注册取消函数
	execCtx, cancel := context.WithCancel(ctx)
	if run.ID > 0 {
		s.runMu.Lock()
		s.running[run.ID] = cancel
		s.runMu.Unlock()
	}
	defer func() {
		cancel()
		if run.ID > 0 {
			s.runMu.Lock()
			delete(s.running, run.ID)
			s.runMu.Unlock()
		}
	}()

	slog.InfoContext(ctx, "开始执行任务",
		"task_id", taskID, "task", task.Name, "kind", task.Kind, "trigger", triggerType)

	// ④ 变量替换（DAG 子任务继承父 DAG 变量）
	if len(vars) == 0 {
		vars = dag.VarsFromContext(ctx)
	}
	payload := substituteVars(task.Payload, task.Variables, vars)

	// ⑤ 为 DAG 任务注入编排上下文
	if task.Kind == "dag" && run.ID > 0 {
		execCtx = dag.WithRunID(execCtx, run.ID)
		execCtx = dag.WithVars(execCtx, vars)
	}

	// ⑥ 委派给 Worker 执行
	result := s.worker.Execute(execCtx, worker.RunRequest{
		RunID:       run.ID,
		TaskID:      taskID,
		TaskName:    task.Name,
		Kind:        task.Kind,
		Payload:     payload,
		TriggerType: triggerType,
		ParentRunID: parentRunID.Int64,
		Vars:        vars,
	})

	// ⑦ 持久化运行结果
	if run.ID > 0 {
		var errMsg sql.NullString
		if result.Error != "" {
			errMsg = sql.NullString{String: result.Error, Valid: true}
		}
		var output sql.NullString
		if result.Output != "" {
			output = sql.NullString{String: result.Output, Valid: true}
		}
		_ = s.queries.FinishTaskRun(ctx, gen.FinishTaskRunParams{
			ID:         run.ID,
			FinishedAt: sql.NullTime{Time: time.Now(), Valid: true},
			Status:     result.Status,
			ErrorMsg:   errMsg,
			DurationMs: sql.NullInt64{Int64: result.DurationMs, Valid: true},
			Output:     output,
		})
	}

	if result.Status != "success" {
		return fmt.Errorf("任务 %q 执行失败: %s", task.Name, result.Error)
	}
	return nil
}

// substituteVars 将 payload 中的 ${key} 占位符替换为 vars 中的值，
// 未提供的 key 回退到变量定义中的默认值。
func substituteVars(payload json.RawMessage, varDefs json.RawMessage, vars map[string]string) json.RawMessage {
	type varDef struct {
		Key          string `json:"key"`
		DefaultValue string `json:"default_value"`
	}
	var defs []varDef
	if err := json.Unmarshal(varDefs, &defs); err != nil || len(defs) == 0 {
		return payload
	}

	s := string(payload)
	for _, d := range defs {
		val := d.DefaultValue
		if v, ok := vars[d.Key]; ok {
			val = v
		}
		s = strings.ReplaceAll(s, "${"+d.Key+"}", val)
	}
	return json.RawMessage(s)
}
