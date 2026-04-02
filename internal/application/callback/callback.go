package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"text/template"
	"time"

	"github.com/google/cel-go/cel"

	"github.com/rushteam/dagflow/internal/application/varfunc"
	"github.com/rushteam/dagflow/internal/infrastructure/database/gen"
)

// Payload 是回调的上下文数据，同时作为默认 JSON body 和模板变量来源。
type Payload struct {
	RunID      int64  `json:"run_id"`
	TaskID     int64  `json:"task_id"`
	TaskName   string `json:"task_name"`
	TaskLabel  string `json:"task_label"`
	TaskKind   string `json:"task_kind"`
	Status     string `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
	Output     string `json:"output,omitempty"`
	FinishedAt string `json:"finished_at"`
}

// BuiltinVars 返回可用于 body_template 的内置变量列表（供前端展示）。
func BuiltinVars() []VarInfo {
	return []VarInfo{
		{Name: "run_id", Label: "运行 ID", Example: "42"},
		{Name: "task_id", Label: "任务 ID", Example: "7"},
		{Name: "task_name", Label: "任务名称", Example: "daily_export"},
		{Name: "task_label", Label: "任务标签", Example: "每日导出"},
		{Name: "task_kind", Label: "任务类型", Example: "shell"},
		{Name: "status", Label: "执行状态", Example: "success / failed / cancelled"},
		{Name: "duration_ms", Label: "耗时(ms)", Example: "12345"},
		{Name: "error", Label: "错误信息", Example: "exit code 1"},
		{Name: "output", Label: "任务输出", Example: "done"},
		{Name: "finished_at", Label: "完成时间", Example: "2025-03-30T10:00:00Z"},
	}
}

// VarInfo 描述一个内置回调变量。
type VarInfo struct {
	Name    string `json:"name"`
	Label   string `json:"label"`
	Example string `json:"example"`
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

// MatchRules 存储 CEL 表达式，match_mode="selected" 时用 Expr 求值。
type MatchRules struct {
	Expr string `json:"expr,omitempty"`
}

// celEnv 全局 CEL 环境（线程安全），暴露任务相关变量供表达式引用。
var celEnv *cel.Env

func init() {
	var err error
	celEnv, err = cel.NewEnv(
		cel.Variable("task_id", cel.IntType),
		cel.Variable("task_name", cel.StringType),
		cel.Variable("task_kind", cel.StringType),
		cel.Variable("task_label", cel.StringType),
	)
	if err != nil {
		panic(fmt.Sprintf("初始化 CEL 环境失败: %v", err))
	}
}

// ValidateExpr 校验 CEL 表达式是否合法（供 API 层调用）。
func ValidateExpr(expr string) error {
	ast, issues := celEnv.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return fmt.Errorf("表达式语法错误: %w", issues.Err())
	}
	if ast.OutputType() != cel.BoolType {
		return fmt.Errorf("表达式必须返回布尔值，当前返回 %v", ast.OutputType())
	}
	return nil
}

// FireMatched 从所有已启用的回调中筛选匹配当前任务和事件的，异步触发。
func FireMatched(callbacks []gen.Callback, p Payload) {
	for _, cb := range callbacks {
		if !matchTask(cb, p) {
			continue
		}
		if !matchEvent(cb.Events, p.Status) {
			continue
		}

		headers := parseHeaders(cb.Headers)
		bodyTemplate := cb.BodyTemplate
		go func(cbName, cbURL string) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			body := renderBody(bodyTemplate, p)

			if err := doPost(ctx, cbURL, headers, body); err != nil {
				slog.Warn("回调触发失败（已忽略）",
					"callback", cbName, "task_id", p.TaskID, "run_id", p.RunID,
					"url", cbURL, "status", p.Status, "error", err)
			} else {
				slog.Info("回调触发成功",
					"callback", cbName, "task_id", p.TaskID, "run_id", p.RunID,
					"url", cbURL, "status", p.Status)
			}
		}(cb.Name, cb.Url)
	}
}

// renderBody 根据 body_template 渲染请求体。
// 模板为空时用默认 JSON；非空时用 Go template 引擎渲染。
func renderBody(tmplStr string, p Payload) []byte {
	if tmplStr == "" {
		body, _ := json.Marshal(p)
		return body
	}

	data := payloadToData(p)
	src := varfunc.ConvertSyntax(tmplStr)

	tmpl, err := template.New("callback_body").
		Funcs(varfunc.TemplateFuncMap(time.Now())).
		Option("missingkey=zero").
		Parse(src)
	if err != nil {
		slog.Warn("回调 body_template 解析失败，使用默认 body", "error", err)
		body, _ := json.Marshal(p)
		return body
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		slog.Warn("回调 body_template 渲染失败，使用默认 body", "error", err)
		body, _ := json.Marshal(p)
		return body
	}
	return buf.Bytes()
}

func payloadToData(p Payload) map[string]string {
	return map[string]string{
		"run_id":      strconv.FormatInt(p.RunID, 10),
		"task_id":     strconv.FormatInt(p.TaskID, 10),
		"task_name":   p.TaskName,
		"task_label":  p.TaskLabel,
		"task_kind":   p.TaskKind,
		"status":      p.Status,
		"duration_ms": strconv.FormatInt(p.DurationMs, 10),
		"error":       p.Error,
		"output":      p.Output,
		"finished_at": p.FinishedAt,
	}
}

func matchTask(cb gen.Callback, p Payload) bool {
	if cb.MatchMode == "all" {
		return true
	}
	var mr MatchRules
	if err := json.Unmarshal(cb.MatchRules, &mr); err != nil || mr.Expr == "" {
		return false
	}

	ast, issues := celEnv.Compile(mr.Expr)
	if issues != nil && issues.Err() != nil {
		slog.Warn("CEL 表达式编译失败", "expr", mr.Expr, "error", issues.Err())
		return false
	}
	prg, err := celEnv.Program(ast)
	if err != nil {
		slog.Warn("CEL 程序构建失败", "expr", mr.Expr, "error", err)
		return false
	}
	out, _, err := prg.Eval(map[string]any{
		"task_id":    p.TaskID,
		"task_name":  p.TaskName,
		"task_kind":  p.TaskKind,
		"task_label": p.TaskLabel,
	})
	if err != nil {
		slog.Warn("CEL 表达式求值失败", "expr", mr.Expr, "error", err)
		return false
	}
	result, ok := out.Value().(bool)
	return ok && result
}

func matchEvent(eventsRaw json.RawMessage, status string) bool {
	var events []string
	if err := json.Unmarshal(eventsRaw, &events); err != nil || len(events) == 0 {
		return true
	}
	for _, e := range events {
		if e == status {
			return true
		}
	}
	return false
}

func parseHeaders(raw json.RawMessage) map[string]string {
	var h map[string]string
	if json.Unmarshal(raw, &h) != nil {
		return nil
	}
	return h
}

func doPost(ctx context.Context, url string, headers map[string]string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建回调请求: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Dash-Callback/1.0")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送回调请求: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("回调返回 HTTP %d", resp.StatusCode)
	}
	return nil
}
