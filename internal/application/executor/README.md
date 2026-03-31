# executor — 任务执行器

## 目录结构

```
executor/
├── executor.go     # Func / Info / Registry 核心类型
├── register.go     # RegisterBuiltin() 统一注册入口
├── util.go         # 共享工具函数 (ExpandEnv, Truncate)
├── shell.go        # Shell 命令执行器
├── http.go         # HTTP 请求执行器
└── README.md       # 本文档
```

## 核心类型

```go
// 执行函数签名：接收 context + JSON payload，返回 error
type Func func(ctx context.Context, payload json.RawMessage) error

// Kind 元信息
type Info struct {
    Name        string  // 唯一标识，如 "shell"、"http"
    Label       string  // 前端显示名，如 "Shell 命令"
    PayloadHint string  // payload JSON 示例，前端新建任务时展示
    Fn          Func    // 执行函数
}

// 注册表
type Registry struct { ... }
```

## 已有执行器

### shell — Shell 命令

顺序执行多条 shell 命令，任一失败则整体失败。

**Payload**:
```json
{
  "commands": [
    "echo 'step 1'",
    "python3 /scripts/export.py --date=$(date +%F)",
    "ls -la /tmp/output"
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `commands` | `string[]` | 是 | shell 命令列表，按顺序执行 |

### http — HTTP 请求

发送 HTTP 请求，状态码 >= 400 视为失败。支持 `${VAR}` 环境变量替换。

**Payload**:
```json
{
  "url": "https://api.example.com/train",
  "method": "POST",
  "headers": {
    "Content-Type": "application/json",
    "Authorization": "Bearer ${API_TOKEN}"
  },
  "body": "{\"model\": \"v2\", \"date\": \"2026-03-30\"}",
  "timeout": 60
}
```

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `url` | `string` | 是 | - | 请求地址，支持 `${VAR}` |
| `method` | `string` | 否 | `GET` | HTTP 方法 |
| `headers` | `map[string]string` | 否 | - | 请求头，值支持 `${VAR}` |
| `body` | `string` | 否 | - | 请求体，支持 `${VAR}` |
| `timeout` | `int` | 否 | `30` | 超时秒数 |

环境变量替换示例：`${API_TOKEN}` → 读取 Pod 环境变量 `API_TOKEN` 的值。

---

## 新增执行器指南

### 第一步：创建文件

在 `executor/` 下新建 `<kind>.go`，遵循以下模板：

```go
package executor

import (
    "context"
    "encoding/json"
    "fmt"
    "log/slog"
)

// 1. 导出 Info 变量（大写开头，首字母与文件名一致）
var MyKind = Info{
    Name:        "my_kind",                            // 唯一标识，小写+下划线
    Label:       "我的执行器",                            // 中文显示名
    PayloadHint: `{"param1": "value1", "count": 10}`,  // JSON 示例
    Fn:          myKindFn,                              // 指向执行函数
}

// 2. 定义 payload 结构体（unexported，仅本文件使用）
type myKindPayload struct {
    Param1 string `json:"param1"`
    Count  int    `json:"count"`
}

// 3. 实现执行函数（unexported）
func myKindFn(ctx context.Context, payload json.RawMessage) error {
    // 解析 payload
    var p myKindPayload
    if err := json.Unmarshal(payload, &p); err != nil {
        return fmt.Errorf("解析 my_kind payload 失败: %w", err)
    }

    // 校验必填字段
    if p.Param1 == "" {
        return fmt.Errorf("param1 不能为空")
    }

    // 执行逻辑，使用 slog 记录关键步骤
    slog.InfoContext(ctx, "开始执行 my_kind", "param1", p.Param1, "count", p.Count)

    // ... 实际业务逻辑 ...

    return nil
}
```

### 第二步：注册

在 `register.go` 中追加一行：

```go
func RegisterBuiltin(r *Registry) {
    r.Register(Shell)
    r.Register(HTTP)
    r.Register(MyKind)  // ← 加这一行
}
```

完成。后端无需改动其他任何文件，前端会自动在类型下拉中展示新 Kind，使用通用 JSON 编辑器填写 payload。

### 第三步（可选）：前端专用表单

默认情况下，新 Kind 在前端使用通用 JSON textarea 编辑 payload（会显示 PayloadHint 作为示例）。
如果需要更好的用户体验，可以在 `TaskPanel.tsx` 中添加专用表单组件：

1. 在 `TaskFormDialog` 的 payload 编辑区域添加分支：
```tsx
{currentKind === 'shell' ? (
  <ShellForm ... />
) : currentKind === 'http' ? (
  <HttpPayloadForm ... />
) : currentKind === 'my_kind' ? (
  <MyKindForm ... />       // ← 新增
) : (
  <GenericJsonTextarea />  // 通用 fallback
)}
```

2. 在 `buildPayload()` 中添加对应的构建逻辑。

## 编写约定

| 约定 | 说明 |
|---|---|
| **文件命名** | `<kind>.go`，与 `Info.Name` 一致 |
| **变量命名** | 导出变量用 PascalCase（如 `Shell`、`HTTP`、`MyKind`） |
| **函数命名** | 执行函数 unexported，`<kind>Fn`（如 `shellFn`、`httpFn`） |
| **payload 结构体** | unexported，`<kind>Payload`（如 `httpPayload`） |
| **错误信息** | 中文前缀标明 Kind，如 `"解析 http payload 失败: ..."` |
| **日志** | 使用 `slog.InfoContext(ctx, ...)`，关键步骤必须打日志 |
| **context** | 必须透传 ctx，用于超时控制和取消传播 |
| **环境变量** | 需要支持 `${VAR}` 替换的字段使用 `ExpandEnv()` |

## 共享工具函数（util.go）

| 函数 | 说明 |
|---|---|
| `ExpandEnv(s string) string` | 将 `${VAR}` 替换为环境变量值 |
| `Truncate(s string, n int) string` | 截断字符串到 n 字节，超出加 `...` |

如果新执行器需要通用工具（如重试、文件下载），在 `util.go` 中添加。
