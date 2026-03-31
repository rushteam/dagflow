// Package worker 定义任务执行抽象。
//
// Scheduler（调度层）负责"何时跑、跑什么"，Worker（执行层）负责"怎么跑"。
// 当前 LocalWorker 在进程内执行；未来可替换为 RemoteWorker 分发到消息队列，
// Scheduler 代码无需改动。
package worker

import (
	"context"
	"encoding/json"
)

// RunRequest 包含执行一个任务所需的全部信息。
// 所有字段均为值类型或不可变引用，可安全跨 goroutine / 跨进程序列化。
type RunRequest struct {
	RunID       int64
	TaskID      int64
	TaskName    string
	Kind        string
	Payload     json.RawMessage
	TriggerType string
	ParentRunID int64
	Vars        map[string]string
}

// RunResult 是任务执行的结果。
type RunResult struct {
	Status     string // "success" | "failed" | "cancelled"
	Error      string
	Output     string
	DurationMs int64
}

// Worker 抽象任务执行。
//
// 实现约定：
//   - ctx 携带取消信号，实现方必须响应 ctx.Done()
//   - Execute 是同步调用，返回时任务已结束
//   - 实现方不负责持久化运行记录，由 Scheduler 统一管理
type Worker interface {
	Execute(ctx context.Context, req RunRequest) RunResult
}
