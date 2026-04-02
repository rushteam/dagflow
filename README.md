# Dash — 任务调度管理平台

## 核心概念

| 概念 | 说明 |
|------|------|
| **Task** | 最小执行单元，通过 `kind` 区分类型（shell / http / dag / etl） |
| **Variable** | 任务定义的变量坑位（`key` + `default_value`），payload 中用 `${KEY}` 引用 |
| **Schedule** | 调度配置，关联一个 Task，支持 cron / once，可配置 `variable_overrides` |
| **DAG** | 一种特殊 Task（`kind=dag`），编排多个子 Task 的依赖执行 |
| **ETL** | 一种特殊 Task（`kind=etl`），从 Source 查询数据，批量写入 Sink |
| **Callback** | 任务执行完成后的 Webhook 回调，支持按任务/事件过滤和自定义 body 模板 |

## 任务类型（Kind）

### shell — Shell 命令

```json
{
  "commands": ["echo hello", "ls -la"]
}
```

### http — HTTP 请求

```json
{
  "url": "https://api.example.com/endpoint",
  "method": "POST",
  "headers": {"Content-Type": "application/json"},
  "body": "{\"key\": \"value\"}",
  "timeout": 30
}
```

HTTP 任务会自动附加追踪头：
- `X-Dash-Run-Id` — 本次运行 ID
- `X-Dash-Task-Name` — 任务名称

### dag — DAG 流程

```json
{
  "nodes": [
    {"name": "步骤1", "task_id": 1, "depends_on": []},
    {"name": "步骤2", "task_id": 2, "depends_on": ["步骤1"]}
  ],
  "strategy": "fail_fast"
}
```

`strategy`：`fail_fast`（默认，节点失败立即终止）或 `continue_on_error`。

### etl — ETL 数据同步

从 Source 查询数据，按批次流式写入 Sink。连接信息在 payload 中配置，不同任务可指向不同实例。

```json
{
  "source": {
    "type": "tga",
    "base_url": "https://tga.example.com",
    "token": "YOUR_TOKEN",
    "insecure": true,
    "timeout": 300,
    "sql": "SELECT uid, score FROM user_scores WHERE dt='2026-04-01'"
  },
  "sink": {
    "type": "redis",
    "addr": "your-redis:6379",
    "password": "",
    "db": 0,
    "command": "SET",
    "key_template": "rec:score:{{.uid}}",
    "value_field": "score",
    "ttl": 86400
  },
  "batch_size": 1000
}
```

#### Source: TGA

| 字段 | 必填 | 说明 |
|------|------|------|
| `type` | 是 | 固定 `"tga"` |
| `base_url` | 是 | TGA API 地址 |
| `token` | 是 | 鉴权 token |
| `insecure` | 否 | 跳过 TLS 证书校验（TGA 自签名证书设 `true`） |
| `timeout` | 否 | 查询超时秒数，默认 300 |
| `retries` | 否 | 失败重试次数，默认 3 |
| `sql` | 是 | 查询 SQL |

#### Source: MySQL

| 字段 | 必填 | 说明 |
|------|------|------|
| `type` | 是 | 固定 `"mysql"` |
| `dsn` | 是 | MySQL DSN，如 `user:pass@tcp(host:3306)/dbname?charset=utf8mb4` |
| `sql` | 是 | 查询 SQL |
| `timeout` | 否 | 查询超时秒数，默认 300 |

```json
{
  "source": {
    "type": "mysql",
    "dsn": "readonly:pass@tcp(mysql:3306)/analytics?charset=utf8mb4",
    "sql": "SELECT uid, score FROM user_scores WHERE date = CURDATE()"
  },
  "sink": {"type": "print"}
}
```

#### Sink: Redis

| 字段 | 必填 | 说明 |
|------|------|------|
| `type` | 是 | 固定 `"redis"` |
| `addr` | 是 | Redis 地址 |
| `password` | 否 | 密码 |
| `db` | 否 | DB 编号，默认 0 |
| `command` | 否 | `SET`（默认） / `HSET` / `ZADD` / `RPUSH` |
| `key_template` | 是 | Go template，如 `rec:{{.uid}}` |
| `ttl` | 否 | 过期秒数，0 = 不过期 |

**各命令的额外字段：**

| 命令 | 额外字段 | 说明 |
|------|----------|------|
| **SET** | `value_field` 或 `value_template` | 不填则写入整行 JSON |
| **HSET** | `field_template` + value | field 和 value 都支持模板 |
| **ZADD** | `score_field` + `member_field` | score 必须是数字字段 |
| **RPUSH** | value | 同 SET 的 value 逻辑 |

**Redis Sink 示例：**

SET — 每个 key 存一个值：
```json
{"command": "SET", "key_template": "rec:score:{{.uid}}", "value_field": "score", "ttl": 86400}
```

HSET — 一个大 hash，field 是 uid，value 是整行 JSON：
```json
{"command": "HSET", "key_template": "user:profiles", "field_template": "{{.uid}}", "ttl": 86400}
```

ZADD — 有序集合：
```json
{"command": "ZADD", "key_template": "rec:user_scores", "score_field": "score", "member_field": "uid", "ttl": 86400}
```

#### Sink: Print（调试用）

将查询结果打印到任务输出中，用于验证 Source 查询是否正确。

| 字段 | 必填 | 说明 |
|------|------|------|
| `type` | 是 | 固定 `"print"` |
| `format` | 否 | `"json"`（默认，每行一个 JSON）或 `"table"`（tab 分隔表格） |

```json
{"sink": {"type": "print", "format": "table"}}
```

#### 扩展 Source / Sink

实现 `Source` 或 `Sink` 接口，写一个 `Factory`，在 `main.go` 注册即可：

```go
etlReg.RegisterSource("my_source", mySourceFactory)
etlReg.RegisterSink("my_sink", mySinkFactory)
```

## 创建任务时同时创建调度

`POST /api/v1/tasks` 支持在创建任务的同时创建调度，通过可选的 `schedule` 字段：

```json
{
  "name": "daily_sync",
  "label": "每日数据同步",
  "kind": "etl",
  "payload": { "..." : "..." },
  "enabled": true,
  "schedule": {
    "name": "daily_sync_cron",
    "schedule_type": "cron",
    "cron_expr": "0 2 * * *",
    "variable_overrides": [
      {"key": "DATE", "type": "date", "format": "yyyyMMdd", "offset": "-1d"}
    ],
    "enabled": true
  }
}
```

- `schedule` 字段可选，不传则只创建任务
- 仅创建任务时生效，编辑任务时不会处理此字段
- 调度创建成功后，响应中会包含 `schedule_id`

## 回调（Callback）

任务执行完成后，自动触发匹配的 Webhook 回调。

| 字段 | 说明 |
|------|------|
| `url` | 回调 URL |
| `events` | 触发事件：`success` / `failed` / `cancelled`（逗号分隔） |
| `task_id` | 关联任务 ID（空则匹配所有任务） |
| `headers` | 自定义请求头（JSON 对象） |
| `body_template` | 自定义 body 模板（Go template），不填则发送默认 JSON |

**body_template 可用变量：**

| 变量 | 说明 | 示例 |
|------|------|------|
| `{{.run_id}}` | 运行 ID | 42 |
| `{{.task_id}}` | 任务 ID | 7 |
| `{{.task_name}}` | 任务名称 | daily_export |
| `{{.task_label}}` | 任务标签 | 每日导出 |
| `{{.task_kind}}` | 任务类型 | shell |
| `{{.status}}` | 执行状态 | success / failed / cancelled |
| `{{.duration_ms}}` | 耗时(ms) | 12345 |
| `{{.error}}` | 错误信息 | exit code 1 |
| `{{.output}}` | 任务输出 | done |
| `{{.finished_at}}` | 完成时间 | 2025-03-30T10:00:00Z |

## 变量系统

### 三层设计

```
任务定义坑位  →  调度/手动 决定怎么填  →  执行时替换 payload
```

| 层级 | 职责 | 数据结构 |
|------|------|----------|
| **任务（Task）** | 声明需要哪些变量 | `variables: [{key, default_value}]` |
| **调度（Schedule）** | 决定变量值的来源 | `variable_overrides: [{key, type, value/format/offset}]` |
| **手动执行** | 用户直接填写值 | `POST /run body: {variables: {key: value}}` |

### 变量覆盖类型

| type | 说明 | 示例 |
|------|------|------|
| `fixed` | 固定值 | `{key: "ENV", type: "fixed", value: "prod"}` |
| `date` | 日期函数，运行时计算 | `{key: "DATE", type: "date", format: "yyyyMMdd", offset: "-1d"}` |

### 日期格式 & 偏移量

**格式 pattern**（Java 风格）：

| Pattern | 含义 | 示例 |
|---------|------|------|
| `yyyy` | 四位年 | 2026 |
| `MM` | 两位月 | 03 |
| `dd` | 两位日 | 30 |
| `HH` | 24 小时 | 14 |
| `mm` | 分钟 | 05 |
| `ss` | 秒 | 09 |

**偏移量**：`±数字` + `单位`，如 `-1d`（昨天）、`+7d`（7 天后）、`-1M`（上月）、`-1h`（1 小时前）。

### 执行链路 — 手动触发

```
用户点击「执行」
│
├─ 任务无变量 → 直接执行
│
└─ 任务有变量 → 弹出表单（预填 default_value）
    │
    用户填写 / 确认
    │
    POST /api/v1/tasks/{id}/run  body: { variables: { DATE: "20260330", ENV: "prod" } }
    │
    └→ scheduler.RunTask(taskID, userID, vars)
        └→ runTaskAsync(taskID, vars={DATE: "20260330", ENV: "prod"})
            │
            ├─ substituteVars(payload, task.variables, vars)
            │   将 payload 中 ${DATE} → "20260330", ${ENV} → "prod"
            │
            └─ kind.Fn(ctx, 替换后的payload)
```

### 执行链路 — 调度触发

```
Cron 触发 executeSchedule(scheduleID)
│
├─ 从 DB 加载 schedule（含 variable_overrides）
│
├─ varfunc.Resolve(overrides, time.Now())
│   type=fixed  → 直接取 value
│   type=date   → 按 format+offset 计算，如 yyyyMMdd -1d → "20260329"
│   输出: vars = {DATE: "20260329", ENV: "prod"}
│
└→ runTaskAsync(taskID, vars={DATE: "20260329", ENV: "prod"})
    │
    ├─ substituteVars(payload, task.variables, vars)
    │   未配置覆盖的变量 → 回退到 task.variables 中的 default_value
    │
    └─ kind.Fn(ctx, 替换后的payload)
```

### 执行链路 — DAG 任务（变量向子任务传递）

```
触发 DAG 任务（手动传 vars 或调度计算 vars）
│
└→ runTaskAsync(dagTask, vars={DATE: "20260330"})
    │
    ├─ substituteVars: DAG 自身 payload 中的 ${DATE} 被替换
    │
    ├─ dag.WithVars(ctx, vars)    ← 将 vars 注入 context
    ├─ dag.WithRunID(ctx, runID)
    │
    └─ dagExecutor.run(ctx, payload)
        │
        │  拓扑排序 → 分层并行执行
        │
        ├─ Layer 0（无依赖的节点并行执行）
        │   ├→ runner.RunTaskSync(ctx, childTaskA)
        │   │    └→ runTaskAsync(childTaskA, vars=nil)
        │   │         ├ vars = dag.VarsFromContext(ctx)  ← 继承父 DAG 的变量
        │   │         │   → {DATE: "20260330"}
        │   │         ├ substituteVars: childA payload 中 ${DATE} 被替换
        │   │         └ kind.Fn(ctx, 替换后的payload)
        │   │
        │   └→ runner.RunTaskSync(ctx, childTaskB)
        │        └→ 同理继承 DAG 的 vars
        │
        ├─ Layer 1（依赖 Layer 0 的节点）
        │   └→ ...
        │
        └─ DAG 执行完成
```

### substituteVars 优先级

```
1. 运行时传入的 vars（手动填写 / 调度计算 / DAG 继承）  ← 最高优先级
2. 任务 variables 中定义的 default_value               ← 兜底
3. 未匹配的 ${KEY} 保持原样                            ← 不替换
```

## 架构分层（DDD）

```
interface/http/handler/    ← 接口层：HTTP handler（Task/Schedule/Callback/Auth/Token）
application/
├── executor/              ← 执行器：shell / http + Registry
├── etl/                   ← ETL executor：Source/Sink 抽象 + TGA/MySQL/Redis/Print 实现
├── dag/                   ← DAG executor：拓扑排序 + 分层并行
├── scheduler/             ← 调度引擎：cron/once + Leader Election
├── worker/                ← 执行层抽象（LocalWorker / 未来 RemoteWorker）
├── callback/              ← 回调触发器
└── varfunc/               ← 变量函数（date 计算）
infrastructure/
├── config/                ← 配置加载
├── database/              ← PostgreSQL + sqlc 生成代码
├── tga/                   ← TGA 数据查询客户端（HTTP + 多行 JSON 协议）
├── election/              ← Leader Election（pg / redis / kube / noop）
├── auth/                  ← JWT 认证
└── http/                  ← HTTP 服务器 + 路由 + SPA 托管
```
