# Dash TODO

## 待办

### 调度任务队列化（Scheduler + Worker 分离）

**现状**：Leader Pod 同时负责调度决策和任务执行，所有任务在单 Pod 上运行。

**触发条件**：当出现单 Pod CPU/内存瓶颈，或任务执行时间长导致调度延迟时考虑实施。

**目标架构**：

```
现在:  Leader Pod = 调度决策 + 执行任务（全部）
改进:  Leader Pod = 调度决策 + 发任务到队列
       所有 Pod  = 从队列消费执行（包括 Leader）
```

#### 方案 A：PG 队列（SKIP LOCKED）

零额外基础设施，复用现有 PG。

```sql
CREATE TABLE task_queue (
    id         BIGSERIAL PRIMARY KEY,
    task_id    BIGINT NOT NULL REFERENCES tasks(id),
    schedule_id BIGINT NOT NULL REFERENCES schedules(id),
    payload    JSONB NOT NULL DEFAULT '{}',
    status     VARCHAR(16) NOT NULL DEFAULT 'pending',  -- pending / running / done / failed
    claimed_by VARCHAR(128),                             -- pod id
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    claimed_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);
CREATE INDEX idx_task_queue_pending ON task_queue (status) WHERE status = 'pending';
```

- **投递**：Leader 到点后 `INSERT INTO task_queue`
- **消费**：所有 Pod 轮询

```sql
-- 原子抢占一条待执行任务
UPDATE task_queue
SET status = 'running', claimed_by = $1, claimed_at = NOW()
WHERE id = (
    SELECT id FROM task_queue
    WHERE status = 'pending'
    ORDER BY created_at
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING *;
```

- **优势**：无新依赖，事务保证，天然持久化
- **劣势**：高并发下轮询有 DB 压力，延迟取决于轮询间隔

#### 方案 B：Redis List 队列

适合已有 Redis 的场景（如已启用 Redis Leader Election）。

- **投递**：`LPUSH task_queue <json_message>`
- **消费**：所有 Pod `BRPOP task_queue 5`（阻塞等待，无轮询开销）
- **确认**：消费后写执行结果到 PG

```
投递消息格式：
{
  "schedule_id": 1,
  "task_id": 5,
  "kind": "shell",
  "payload": {"commands": ["echo hello"]},
  "enqueued_at": "2026-03-30T18:00:00Z"
}
```

- **优势**：BRPOP 零轮询，低延迟，高吞吐
- **劣势**：消息可能丢失（Redis 非持久队列），需要额外可靠性机制（如 RPOPLPUSH + 超时回收）

#### 方案 C：Asynq（推荐长期方案）

基于 Redis 的 Go 任务队列库（[hibiken/asynq](https://github.com/hibiken/asynq)），提供开箱即用的：

- 可靠投递（RPOPLPUSH 模式）
- 自动重试 + 死信队列
- 优先级队列
- 任务去重
- Web Dashboard（asynqmon）
- 定时任务内置支持

```go
// 投递（Leader scheduler.go 中 executeSchedule 改为）
client.Enqueue(asynq.NewTask("shell", payload), asynq.Queue("default"))

// 消费（所有 Pod 启动 worker）
srv := asynq.NewServer(redis, asynq.Config{Concurrency: 10})
mux := asynq.NewServeMux()
mux.HandleFunc("shell", handleShellTask)
srv.Run(mux)
```

- **优势**：生产级可靠性，零样板代码，自带监控
- **劣势**：依赖 Redis，引入新框架

#### 实施建议

| 阶段 | 方案 | 适用场景 |
|---|---|---|
| 现在 | 不改（单 Leader 执行） | 任务少、执行快 |
| 中期 | 方案 A（PG SKIP LOCKED） | 需要分摊负载，但不想加 Redis |
| 长期 | 方案 C（Asynq） | 任务量大、需要重试/优先级/监控 |

#### 改动范围

核心改动仅一处：`scheduler.go` 中 `executeSchedule` 方法——从直接调用 `kindInfo.Executor(ctx, task.Payload)` 改为向队列投递消息。所有 Pod 额外启动 worker 消费循环。其余代码（Kind 注册、调度加载、Leader Election）无需变动。
