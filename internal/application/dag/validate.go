package dag

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/rushteam/dagflow/internal/infrastructure/database/gen"
)

// ValidatePayload 校验 DAG payload：节点引用的 task 存在、无环、
// 嵌套 DAG 不形成自引用环。selfTaskID 是当前正在创建/编辑的任务 ID（0 表示新建）。
func ValidatePayload(ctx context.Context, queries *gen.Queries, selfTaskID int64, raw json.RawMessage) error {
	var p Payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("payload 格式错误: %w", err)
	}

	if len(p.Nodes) == 0 {
		return fmt.Errorf("DAG 至少需要一个节点")
	}

	names := make(map[string]bool, len(p.Nodes))
	for _, n := range p.Nodes {
		if n.Name == "" {
			return fmt.Errorf("节点名不能为空")
		}
		if n.TaskID <= 0 {
			return fmt.Errorf("节点 %q 的 task_id 无效", n.Name)
		}
		if names[n.Name] {
			return fmt.Errorf("节点名重复: %q", n.Name)
		}
		names[n.Name] = true
	}

	for _, n := range p.Nodes {
		task, err := queries.GetTaskByID(ctx, n.TaskID)
		if err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("节点 %q 引用的任务 #%d 不存在", n.Name, n.TaskID)
			}
			return fmt.Errorf("查询任务 #%d 失败: %w", n.TaskID, err)
		}

		if task.ID == selfTaskID {
			return fmt.Errorf("节点 %q 不能引用自身", n.Name)
		}

		if task.Kind == "dag" {
			if err := checkNestedDAG(ctx, queries, selfTaskID, task.Payload, 0); err != nil {
				return fmt.Errorf("节点 %q 嵌套检查失败: %w", n.Name, err)
			}
		}
	}

	if _, err := topoSort(p.Nodes); err != nil {
		return err
	}

	return nil
}

const maxNestDepth = 5

func checkNestedDAG(ctx context.Context, queries *gen.Queries, selfTaskID int64, raw json.RawMessage, depth int) error {
	if depth > maxNestDepth {
		return fmt.Errorf("DAG 嵌套层级超过 %d", maxNestDepth)
	}

	var p Payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil
	}

	for _, n := range p.Nodes {
		if n.TaskID == selfTaskID {
			return fmt.Errorf("检测到循环引用: 嵌套 DAG 中引用了自身")
		}
		task, err := queries.GetTaskByID(ctx, n.TaskID)
		if err != nil {
			continue
		}
		if task.Kind == "dag" {
			if err := checkNestedDAG(ctx, queries, selfTaskID, task.Payload, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}
