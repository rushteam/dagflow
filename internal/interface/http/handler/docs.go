package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegisterDocsRoute 注册 API 文档页面，无需鉴权。
func RegisterDocsRoute(r chi.Router) {
	r.Get("/docs", serveDocs)
	r.Get("/docs/", serveDocs)
}

func serveDocs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(docsHTML))
}

const docsHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Dash API 文档</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;color:#1a1a1a;background:#f7f8fa;line-height:1.6}
.layout{display:flex;min-height:100vh}
.sidebar{width:240px;background:#fff;border-right:1px solid #e8e8e8;padding:20px 0;position:fixed;top:0;left:0;height:100vh;overflow-y:auto}
.sidebar h1{font-size:18px;padding:0 20px 16px;border-bottom:1px solid #f0f0f0;margin-bottom:12px}
.sidebar h1 span{font-size:11px;color:#999;font-weight:400;display:block;margin-top:2px}
.nav-group{margin-bottom:8px}
.nav-group-title{font-size:11px;font-weight:600;color:#999;text-transform:uppercase;letter-spacing:.5px;padding:8px 20px 4px}
.nav-link{display:block;padding:5px 20px;font-size:13px;color:#555;text-decoration:none;transition:all .15s}
.nav-link:hover{color:#1677ff;background:#f5f8ff}
.main{margin-left:240px;flex:1;padding:32px 40px;max-width:960px}
h2{font-size:22px;font-weight:700;margin:40px 0 8px;padding-top:20px;border-top:1px solid #f0f0f0;color:#111}
h2:first-child{border-top:none;margin-top:0}
h3{font-size:15px;font-weight:600;margin:24px 0 8px;color:#333}
p{margin:6px 0;font-size:14px;color:#444}
.badge{display:inline-block;font-size:11px;font-weight:700;padding:2px 8px;border-radius:4px;font-family:monospace;margin-right:6px;min-width:52px;text-align:center}
.get{background:#e6f4ff;color:#1677ff}
.post{background:#f6ffed;color:#389e0d}
.put{background:#fff7e6;color:#d46b08}
.delete{background:#fff2f0;color:#cf1322}
.endpoint{margin:12px 0 16px;padding:14px 16px;background:#fff;border:1px solid #e8e8e8;border-radius:8px}
.endpoint-header{display:flex;align-items:center;gap:8px;font-family:'SF Mono',Consolas,monospace;font-size:14px;font-weight:500}
.endpoint-desc{font-size:13px;color:#666;margin-top:6px}
.auth-tag{font-size:10px;padding:1px 6px;border-radius:3px;background:#f5f5f5;color:#888;margin-left:auto;font-family:sans-serif;font-weight:400}
pre{background:#1a1a2e;color:#e0e0e0;padding:14px 16px;border-radius:8px;font-size:13px;line-height:1.5;overflow-x:auto;margin:8px 0}
code{font-family:'SF Mono',Consolas,monospace;font-size:13px}
.inline-code{background:#f5f5f5;padding:1px 6px;border-radius:3px;font-size:12px;color:#c41a16}
table{width:100%;border-collapse:collapse;margin:8px 0;font-size:13px}
th{text-align:left;padding:8px 12px;background:#fafafa;border-bottom:1px solid #e8e8e8;font-weight:600;color:#666}
td{padding:8px 12px;border-bottom:1px solid #f0f0f0;color:#333}
.note{padding:10px 14px;background:#e6f4ff;border-left:3px solid #1677ff;border-radius:0 6px 6px 0;font-size:13px;color:#333;margin:12px 0}
</style>
</head>
<body>
<div class="layout">
<nav class="sidebar">
  <h1>Dash API <span>DagFlow</span></h1>
  <div class="nav-group">
    <div class="nav-group-title">概览</div>
    <a class="nav-link" href="#auth">鉴权方式</a>
    <a class="nav-link" href="#errors">错误格式</a>
  </div>
  <div class="nav-group">
    <div class="nav-group-title">认证</div>
    <a class="nav-link" href="#login">登录</a>
    <a class="nav-link" href="#me">当前用户</a>
  </div>
  <div class="nav-group">
    <div class="nav-group-title">API Token</div>
    <a class="nav-link" href="#token-create">创建 Token</a>
    <a class="nav-link" href="#token-list">列出 Token</a>
    <a class="nav-link" href="#token-revoke">撤销 Token</a>
  </div>
  <div class="nav-group">
    <div class="nav-group-title">任务</div>
    <a class="nav-link" href="#task-list">任务列表</a>
    <a class="nav-link" href="#task-create">创建任务</a>
    <a class="nav-link" href="#task-get">获取任务</a>
    <a class="nav-link" href="#task-update">更新任务</a>
    <a class="nav-link" href="#task-delete">删除任务</a>
    <a class="nav-link" href="#task-run">执行任务</a>
    <a class="nav-link" href="#task-kinds">任务类型</a>
  </div>
  <div class="nav-group">
    <div class="nav-group-title">运行记录</div>
    <a class="nav-link" href="#runs-list">运行列表</a>
    <a class="nav-link" href="#runs-detail">运行详情</a>
    <a class="nav-link" href="#runs-cancel">取消运行</a>
    <a class="nav-link" href="#runs-children">DAG 子任务</a>
  </div>
  <div class="nav-group">
    <div class="nav-group-title">调度</div>
    <a class="nav-link" href="#sched-list">调度列表</a>
    <a class="nav-link" href="#sched-create">创建调度</a>
    <a class="nav-link" href="#sched-trigger">手动触发</a>
  </div>
  <div class="nav-group">
    <div class="nav-group-title">回调</div>
    <a class="nav-link" href="#cb-list">回调列表</a>
    <a class="nav-link" href="#cb-create">创建回调</a>
    <a class="nav-link" href="#cb-update">更新回调</a>
    <a class="nav-link" href="#cb-delete">删除回调</a>
    <a class="nav-link" href="#cb-vars">模板变量</a>
  </div>
</nav>

<main class="main">

<h2 id="auth">鉴权方式</h2>
<p>所有 API（除 <code class="inline-code">/docs</code>、<code class="inline-code">/health</code>、<code class="inline-code">/api/v1/auth/login</code>）均需鉴权。支持两种方式：</p>

<h3>方式一：JWT（Web UI 使用）</h3>
<p>通过登录接口获取 JWT token：</p>
<pre>Authorization: Bearer eyJhbGciOiJIUzI1NiIs...</pre>

<h3>方式二：API Token（程序调用推荐）</h3>
<p>通过 Token 管理接口创建，前缀为 <code class="inline-code">tk_</code>，长期有效：</p>
<pre>Authorization: Bearer tk_a1b2c3d4e5f6...</pre>

<div class="note">API Token 创建后明文仅返回一次，请妥善保存。</div>

<h2 id="errors">错误格式</h2>
<p>所有错误响应遵循统一格式：</p>
<pre>{"error": "错误描述信息"}</pre>

<!-- ====== 认证 ====== -->

<h2 id="login">登录</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge post">POST</span> /api/v1/auth/login</div>
  <div class="endpoint-desc">使用用户名密码登录，返回 JWT token。</div>
</div>
<h3>请求体</h3>
<pre>{"username": "admin", "password": "123456"}</pre>
<h3>响应</h3>
<pre>{"token": "eyJ...", "user": {"id": 1, "username": "admin", "role": "admin"}}</pre>

<h2 id="me">当前用户</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge get">GET</span> /api/v1/auth/me <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">获取当前登录用户信息。</div>
</div>

<!-- ====== Token ====== -->

<h2 id="token-create">创建 API Token</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge post">POST</span> /api/v1/tokens <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">创建一个 API Token，明文仅在此次响应中返回。</div>
</div>
<h3>请求体</h3>
<table>
  <tr><th>字段</th><th>类型</th><th>必填</th><th>说明</th></tr>
  <tr><td>name</td><td>string</td><td>是</td><td>Token 名称（如 "CI Pipeline"）</td></tr>
  <tr><td>expires_in</td><td>string</td><td>否</td><td>过期时间，如 "30d"、"90d"、"365d"；不传则永不过期</td></tr>
</table>
<h3>响应</h3>
<pre>{
  "token": "tk_a1b2c3d4e5f6...",
  "info": {"id": 1, "name": "CI Pipeline", "prefix": "tk_a1b2c3d4", ...}
}</pre>

<h2 id="token-list">列出 Token</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge get">GET</span> /api/v1/tokens <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">列出所有 API Token（仅显示前缀，不返回明文）。</div>
</div>

<h2 id="token-revoke">撤销 Token</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge delete">DELETE</span> /api/v1/tokens/{id} <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">撤销指定 Token，撤销后立即失效。</div>
</div>

<!-- ====== 任务 ====== -->

<h2 id="task-list">任务列表</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge get">GET</span> /api/v1/tasks <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">获取所有任务。</div>
</div>

<h2 id="task-create">创建任务</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge post">POST</span> /api/v1/tasks <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">创建新任务。</div>
</div>
<h3>请求体</h3>
<table>
  <tr><th>字段</th><th>类型</th><th>必填</th><th>说明</th></tr>
  <tr><td>name</td><td>string</td><td>是</td><td>任务唯一标识</td></tr>
  <tr><td>label</td><td>string</td><td>否</td><td>显示名称</td></tr>
  <tr><td>kind</td><td>string</td><td>是</td><td>任务类型：shell / http / dag / etl</td></tr>
  <tr><td>payload</td><td>object</td><td>是</td><td>任务参数，结构取决于 kind</td></tr>
  <tr><td>variables</td><td>array</td><td>否</td><td>变量定义 [{"key":"DATE","default_value":"20260101"}]</td></tr>
  <tr><td>enabled</td><td>boolean</td><td>否</td><td>是否启用，默认 true</td></tr>
  <tr><td>schedule</td><td>object</td><td>否</td><td>同时创建调度（仅创建时有效），见下方说明</td></tr>
</table>

<h3>schedule 字段（可选，创建时同时创建调度）</h3>
<table>
  <tr><th>字段</th><th>类型</th><th>必填</th><th>说明</th></tr>
  <tr><td>name</td><td>string</td><td>是</td><td>调度名称</td></tr>
  <tr><td>schedule_type</td><td>string</td><td>是</td><td>"cron" 或 "once"</td></tr>
  <tr><td>cron_expr</td><td>string</td><td>cron 时</td><td>Cron 表达式</td></tr>
  <tr><td>run_at</td><td>string</td><td>once 时</td><td>ISO 8601 时间</td></tr>
  <tr><td>variable_overrides</td><td>array</td><td>否</td><td>变量覆盖配置</td></tr>
  <tr><td>enabled</td><td>boolean</td><td>否</td><td>是否启用</td></tr>
</table>
<p>创建成功后响应会包含 <code class="inline-code">schedule_id</code> 字段。</p>

<h3>Shell Payload</h3>
<pre>{"commands": ["echo hello", "ls -la"]}</pre>

<h3>HTTP Payload</h3>
<pre>{
  "url": "https://api.example.com/run",
  "method": "POST",
  "headers": {"Authorization": "Bearer xxx"},
  "body": "{\"key\":\"value\"}",
  "timeout": 30
}</pre>
<p>HTTP 任务会自动附加 <code class="inline-code">X-Dash-Run-Id</code> 和 <code class="inline-code">X-Dash-Task-Name</code> 追踪头。</p>

<h3>DAG Payload</h3>
<pre>{
  "nodes": [
    {"name": "step1", "task_id": 1, "depends_on": []},
    {"name": "step2", "task_id": 2, "depends_on": ["step1"]}
  ],
  "strategy": "fail_fast"
}</pre>

<h3>ETL Payload</h3>
<p>从 Source 查询数据，按批次写入 Sink。连接信息在 payload 中配置，不同任务可指向不同实例。</p>
<pre>{
  "source": {
    "type": "tga",
    "base_url": "https://tga.example.com",
    "token": "TOKEN",
    "insecure": true,
    "timeout": 300,
    "sql": "SELECT uid, score FROM ..."
  },
  "sink": {
    "type": "redis",
    "addr": "redis:6379",
    "command": "SET",
    "key_template": "rec:{{.uid}}",
    "value_field": "score",
    "ttl": 86400
  },
  "batch_size": 1000
}</pre>
<p>Source 支持 <code class="inline-code">tga</code>（TGA SQL 查询）和 <code class="inline-code">mysql</code>（MySQL 查询）。Sink 支持 <code class="inline-code">redis</code>（SET/HSET/ZADD/RPUSH）和 <code class="inline-code">print</code>（输出到任务日志，调试用）。</p>

<h3>MySQL Source</h3>
<pre>{
  "source": {
    "type": "mysql",
    "dsn": "user:pass@tcp(host:3306)/db?charset=utf8mb4",
    "sql": "SELECT uid, score FROM user_scores",
    "timeout": 300
  },
  "sink": {"type": "print", "format": "table"},
  "batch_size": 500
}</pre>

<h3>Print Sink（调试）</h3>
<p>将查询结果打印到任务输出中，<code class="inline-code">format</code> 可选 <code class="inline-code">json</code>（默认）或 <code class="inline-code">table</code>（tab 分隔表格）。</p>

<h3>示例：创建任务并同时创建调度</h3>
<pre>curl -X POST /api/v1/tasks \
  -H "Authorization: Bearer tk_xxx" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "daily_sync",
  "kind": "etl",
  "payload": {"source":{"type":"tga","base_url":"...","sql":"..."},"sink":{"type":"redis","addr":"...","command":"SET","key_template":"k:{{.id}}","value_field":"v"}},
  "enabled": true,
  "schedule": {
    "name": "daily_sync_cron",
    "schedule_type": "cron",
    "cron_expr": "0 2 * * *",
    "enabled": true
  }
}'</pre>

<h2 id="task-get">获取任务</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge get">GET</span> /api/v1/tasks/{id} <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">获取单个任务详情。</div>
</div>

<h2 id="task-update">更新任务</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge put">PUT</span> /api/v1/tasks/{id} <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">更新任务，请求体与创建相同。</div>
</div>

<h2 id="task-delete">删除任务</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge delete">DELETE</span> /api/v1/tasks/{id} <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">删除任务。关联的调度将无法执行。</div>
</div>

<h2 id="task-run">执行任务</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge post">POST</span> /api/v1/tasks/{id}/run <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">手动触发执行，异步返回。</div>
</div>
<h3>请求体（可选）</h3>
<pre>{"variables": {"DATE": "20260330", "ENV": "prod"}}</pre>
<p>如果任务定义了变量（<code class="inline-code">${KEY}</code>），在此传入运行时值。</p>

<h2 id="task-kinds">任务类型</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge get">GET</span> /api/v1/kinds <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">列出系统支持的任务类型及 payload 示例。</div>
</div>

<!-- ====== 运行记录 ====== -->

<h2 id="runs-list">运行列表（分页）</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge get">GET</span> /api/v1/task-runs <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">分页查询所有任务运行记录。</div>
</div>
<h3>查询参数</h3>
<table>
  <tr><th>参数</th><th>说明</th></tr>
  <tr><td>page</td><td>页码（默认 1）</td></tr>
  <tr><td>size</td><td>每页条数（默认 15，最大 100）</td></tr>
  <tr><td>task_name</td><td>按任务标识筛选</td></tr>
  <tr><td>task_label</td><td>按任务名称筛选</td></tr>
  <tr><td>run_id</td><td>按运行 ID 精确查找</td></tr>
</table>

<h2 id="runs-detail">运行详情</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge get">GET</span> /api/v1/task-runs/{id} <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">获取单次运行的完整信息，包括执行输出。</div>
</div>

<h2 id="runs-cancel">取消运行</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge post">POST</span> /api/v1/task-runs/{id}/cancel <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">取消正在运行的任务。仅 status=running 的任务可取消。</div>
</div>

<h2 id="runs-children">DAG 子任务</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge get">GET</span> /api/v1/task-runs/{id}/children <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">查询 DAG 任务的子任务运行记录。</div>
</div>

<!-- ====== 调度 ====== -->

<h2 id="sched-list">调度列表</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge get">GET</span> /api/v1/schedules <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">获取所有调度配置。</div>
</div>

<h2 id="sched-create">创建调度</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge post">POST</span> /api/v1/schedules <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">创建定时调度或一次性调度。</div>
</div>
<h3>请求体</h3>
<table>
  <tr><th>字段</th><th>类型</th><th>必填</th><th>说明</th></tr>
  <tr><td>name</td><td>string</td><td>是</td><td>调度名称</td></tr>
  <tr><td>task_id</td><td>number</td><td>是</td><td>关联任务 ID</td></tr>
  <tr><td>schedule_type</td><td>string</td><td>是</td><td>"cron" 或 "once"</td></tr>
  <tr><td>cron_expr</td><td>string</td><td>cron 时</td><td>Cron 表达式（支持秒），如 "0 30 2 * * *"</td></tr>
  <tr><td>run_at</td><td>string</td><td>once 时</td><td>ISO 8601 时间</td></tr>
  <tr><td>variable_overrides</td><td>array</td><td>否</td><td>变量覆盖配置</td></tr>
  <tr><td>enabled</td><td>boolean</td><td>否</td><td>是否启用</td></tr>
</table>

<h2 id="sched-trigger">手动触发调度</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge post">POST</span> /api/v1/schedules/{id}/trigger <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">立即触发一次调度执行。</div>
</div>

<!-- ====== 回调 ====== -->

<h2 id="cb-list">回调列表</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge get">GET</span> /api/v1/callbacks <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">获取所有回调配置。</div>
</div>

<h2 id="cb-create">创建回调</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge post">POST</span> /api/v1/callbacks <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">创建 Webhook 回调，任务执行完成后自动触发。</div>
</div>
<h3>请求体</h3>
<table>
  <tr><th>字段</th><th>类型</th><th>必填</th><th>说明</th></tr>
  <tr><td>name</td><td>string</td><td>是</td><td>回调名称</td></tr>
  <tr><td>url</td><td>string</td><td>是</td><td>Webhook URL</td></tr>
  <tr><td>events</td><td>array</td><td>是</td><td>触发事件：["success", "failed", "cancelled"]</td></tr>
  <tr><td>match_mode</td><td>string</td><td>否</td><td>"all"（默认，匹配所有任务）或 "selected"</td></tr>
  <tr><td>task_ids</td><td>array</td><td>否</td><td>match_mode=selected 时，指定任务 ID 列表</td></tr>
  <tr><td>headers</td><td>object</td><td>否</td><td>自定义请求头</td></tr>
  <tr><td>body_template</td><td>string</td><td>否</td><td>自定义 body（Go template），不填则发送默认 JSON</td></tr>
  <tr><td>enabled</td><td>boolean</td><td>否</td><td>是否启用</td></tr>
</table>
<h3>示例</h3>
<pre>{
  "name": "飞书通知",
  "url": "https://open.feishu.cn/open-apis/bot/v2/hook/xxx",
  "events": ["failed"],
  "match_mode": "all",
  "headers": {"Content-Type": "application/json"},
  "body_template": "{\"msg_type\":\"text\",\"content\":{\"text\":\"任务 {{.task_name}} 失败: {{.error}}\"}}",
  "enabled": true
}</pre>

<h2 id="cb-update">更新回调</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge put">PUT</span> /api/v1/callbacks/{id} <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">更新回调配置，请求体与创建相同。</div>
</div>

<h2 id="cb-delete">删除回调</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge delete">DELETE</span> /api/v1/callbacks/{id} <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">删除回调。</div>
</div>

<h2 id="cb-vars">回调模板变量</h2>
<div class="endpoint">
  <div class="endpoint-header"><span class="badge get">GET</span> /api/v1/callback-vars <span class="auth-tag">需鉴权</span></div>
  <div class="endpoint-desc">列出 body_template 中可用的内置变量。</div>
</div>
<h3>可用变量</h3>
<table>
  <tr><th>变量</th><th>说明</th><th>示例</th></tr>
  <tr><td>{{.run_id}}</td><td>运行 ID</td><td>42</td></tr>
  <tr><td>{{.task_id}}</td><td>任务 ID</td><td>7</td></tr>
  <tr><td>{{.task_name}}</td><td>任务标识</td><td>daily_export</td></tr>
  <tr><td>{{.task_label}}</td><td>任务标签</td><td>每日导出</td></tr>
  <tr><td>{{.task_kind}}</td><td>任务类型</td><td>shell</td></tr>
  <tr><td>{{.status}}</td><td>执行状态</td><td>success / failed / cancelled</td></tr>
  <tr><td>{{.duration_ms}}</td><td>耗时(ms)</td><td>12345</td></tr>
  <tr><td>{{.error}}</td><td>错误信息</td><td>exit code 1</td></tr>
  <tr><td>{{.output}}</td><td>任务输出</td><td>done</td></tr>
  <tr><td>{{.finished_at}}</td><td>完成时间</td><td>2026-04-01T10:00:00Z</td></tr>
</table>

</main>
</div>
</body>
</html>`
