# Dash — 任务调度管理平台

## 核心概念

| 概念 | 说明 |
|------|------|
| **Task** | 最小执行单元，通过 `kind` 区分类型（shell / http / dag） |
| **Variable** | 任务定义的变量坑位（`key` + `default_value`），payload 中用 `${KEY}` 引用 |
| **Schedule** | 调度配置，关联一个 Task，支持 cron / once，可配置 `variable_overrides` |
| **DAG** | 一种特殊 Task（`kind=dag`），编排多个子 Task 的依赖执行 |

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
