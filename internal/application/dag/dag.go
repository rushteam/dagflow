package dag

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/rushteam/dagflow/internal/application/executor"
	"github.com/rushteam/dagflow/internal/infrastructure/database/gen"
)

// TaskRunner 由 scheduler 实现，dag 包通过此接口执行子任务。
type TaskRunner interface {
	RunTaskSync(ctx context.Context, taskID int64, triggerType string, parentRunID int64) error
}

// Node 描述 DAG payload 中的一个节点。
type Node struct {
	Name      string   `json:"name"`
	TaskID    int64    `json:"task_id"`
	DependsOn []string `json:"depends_on"`
}

// Payload 是 kind=dag 任务的 payload 结构。
type Payload struct {
	Nodes    []Node `json:"nodes"`
	Strategy string `json:"strategy"` // "fail_fast"（默认）| "continue_on_error"
}

// Executor 实现 dag 类型的执行器，通过延迟注入获取 TaskRunner。
type Executor struct {
	runner  atomic.Pointer[TaskRunner]
	queries *gen.Queries
}

func NewExecutor(db *sql.DB) *Executor {
	return &Executor{queries: gen.New(db)}
}

// SetRunner 注入 TaskRunner（在 scheduler 创建后调用）。
func (e *Executor) SetRunner(r TaskRunner) {
	e.runner.Store(&r)
}

func (e *Executor) getRunner() TaskRunner {
	p := e.runner.Load()
	if p == nil {
		return nil
	}
	return *p
}

// Info 返回注册到 executor.Registry 的元信息。
func (e *Executor) Info() executor.Info {
	return executor.Info{
		Name:        "dag",
		Label:       "DAG 流程",
		PayloadHint: `{"nodes":[{"name":"步骤1","task_id":1,"depends_on":[]},{"name":"步骤2","task_id":2,"depends_on":["步骤1"]}],"strategy":"fail_fast"}`,
		Fn:          e.run,
	}
}

func (e *Executor) run(ctx context.Context, payload json.RawMessage) error {
	runner := e.getRunner()
	if runner == nil {
		return fmt.Errorf("dag executor: TaskRunner not initialized")
	}

	var p Payload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("dag payload 解析失败: %w", err)
	}

	if len(p.Nodes) == 0 {
		return fmt.Errorf("dag 没有节点")
	}

	if p.Strategy == "" {
		p.Strategy = "fail_fast"
	}

	layers, err := topoSort(p.Nodes)
	if err != nil {
		return err
	}

	parentRunID := runIDFromContext(ctx)

	slog.InfoContext(ctx, "DAG 开始执行",
		"nodes", len(p.Nodes), "layers", len(layers), "strategy", p.Strategy)

	for i, layer := range layers {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("DAG 已取消: %w", err)
		}

		slog.InfoContext(ctx, "DAG 执行层",
			"layer", i, "nodes", len(layer))

		var wg sync.WaitGroup
		errs := make([]error, len(layer))

		for j, node := range layer {
			wg.Add(1)
			go func(idx int, n Node) {
				defer wg.Done()
				slog.InfoContext(ctx, "DAG 节点开始",
					"node", n.Name, "task_id", n.TaskID)
				errs[idx] = runner.RunTaskSync(ctx, n.TaskID, "dag", parentRunID)
			}(j, node)
		}
		wg.Wait()

		for j, err := range errs {
			if err != nil {
				if ctx.Err() != nil {
					return fmt.Errorf("DAG 已取消: %w", ctx.Err())
				}
				slog.ErrorContext(ctx, "DAG 节点失败",
					"node", layer[j].Name, "task_id", layer[j].TaskID, "error", err)
				if p.Strategy == "fail_fast" {
					return fmt.Errorf("DAG 节点 %q 失败: %w", layer[j].Name, err)
				}
			}
		}
	}

	slog.InfoContext(ctx, "DAG 执行完成")
	return nil
}

// topoSort 将节点按依赖关系分层（Kahn's algorithm）。
// 同一层的节点可以并行执行。
func topoSort(nodes []Node) ([][]Node, error) {
	nameMap := make(map[string]*Node, len(nodes))
	inDegree := make(map[string]int, len(nodes))
	deps := make(map[string][]string, len(nodes))

	for i := range nodes {
		n := &nodes[i]
		if _, dup := nameMap[n.Name]; dup {
			return nil, fmt.Errorf("DAG 节点名重复: %q", n.Name)
		}
		nameMap[n.Name] = n
		inDegree[n.Name] = 0
	}

	for _, n := range nodes {
		for _, dep := range n.DependsOn {
			if _, ok := nameMap[dep]; !ok {
				return nil, fmt.Errorf("DAG 节点 %q 依赖不存在的节点 %q", n.Name, dep)
			}
			deps[dep] = append(deps[dep], n.Name)
			inDegree[n.Name]++
		}
	}

	var layers [][]Node
	visited := 0

	for visited < len(nodes) {
		var layer []Node
		for name, deg := range inDegree {
			if deg == 0 {
				layer = append(layer, *nameMap[name])
			}
		}
		if len(layer) == 0 {
			return nil, fmt.Errorf("DAG 存在循环依赖")
		}
		for _, n := range layer {
			delete(inDegree, n.Name)
			for _, child := range deps[n.Name] {
				inDegree[child]--
			}
		}
		layers = append(layers, layer)
		visited += len(layer)
	}

	return layers, nil
}
