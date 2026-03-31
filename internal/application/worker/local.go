package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/rushteam/dagflow/internal/application/executor"
)

// LocalWorker 在当前进程内执行任务。
// 它只依赖 executor.Registry，不关心调度、持久化等上层逻辑。
type LocalWorker struct {
	kinds *executor.Registry
}

func NewLocalWorker(kinds *executor.Registry) *LocalWorker {
	return &LocalWorker{kinds: kinds}
}

func (w *LocalWorker) Execute(ctx context.Context, req RunRequest) RunResult {
	kind, ok := w.kinds.Get(req.Kind)
	if !ok {
		return RunResult{
			Status: "failed",
			Error:  fmt.Sprintf("kind %q 未注册", req.Kind),
		}
	}

	start := time.Now()

	execCtx, outputCollector := executor.WithOutput(ctx)
	taskErr := kind.Fn(execCtx, req.Payload)

	elapsed := time.Since(start)

	result := RunResult{
		Status:     "success",
		Output:     outputCollector.String(),
		DurationMs: elapsed.Milliseconds(),
	}

	if taskErr != nil {
		if execCtx.Err() == context.Canceled {
			result.Status = "cancelled"
			result.Error = "任务被手动取消"
			slog.InfoContext(ctx, "任务已取消",
				"task_id", req.TaskID, "task", req.TaskName, "duration", elapsed)
		} else {
			result.Status = "failed"
			result.Error = taskErr.Error()
			slog.ErrorContext(ctx, "任务执行失败",
				"task_id", req.TaskID, "task", req.TaskName, "error", taskErr, "duration", elapsed)
		}
	} else {
		slog.InfoContext(ctx, "任务执行成功",
			"task_id", req.TaskID, "task", req.TaskName, "duration", elapsed)
	}

	return result
}
