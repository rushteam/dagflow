package varfunc

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
)

// TemplateFuncMap 返回可注入 text/template 的函数表。
// 闭包捕获 now 作为时间基准。
func TemplateFuncMap(now time.Time) template.FuncMap {
	return template.FuncMap{
		"dateFormat": func(format, offset string) string {
			t, _ := applyOffset(now, offset)
			if format == "" {
				format = "yyyyMMdd"
			}
			return t.Format(javaToGoLayout(format))
		},
		"timestamp": func(args ...string) string {
			offset := ""
			if len(args) > 0 {
				offset = args[0]
			}
			t, _ := applyOffset(now, offset)
			return strconv.FormatInt(t.Unix(), 10)
		},
		"timestampMs": func(args ...string) string {
			offset := ""
			if len(args) > 0 {
				offset = args[0]
			}
			t, _ := applyOffset(now, offset)
			return strconv.FormatInt(t.UnixMilli(), 10)
		},
		"uuid": func() string {
			return uuid.NewString()
		},
	}
}

// ResolveOverrides 将 schedule 的 variable_overrides JSON 解析为 key→value 字典。
// override 的 value 本身支持模板表达式，如 ${dateFormat(yyyyMMdd, -1d)}。
func ResolveOverrides(raw json.RawMessage, now time.Time) map[string]string {
	type kv struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	var items []kv
	if json.Unmarshal(raw, &items) != nil || len(items) == 0 {
		return nil
	}

	funcMap := TemplateFuncMap(now)
	result := make(map[string]string, len(items))
	for _, item := range items {
		result[item.Key] = renderExpr(item.Value, funcMap)
	}
	return result
}

// renderExpr 对单个字符串执行模板渲染（解析其中的函数调用）。
func renderExpr(s string, funcMap template.FuncMap) string {
	if !strings.Contains(s, "${") && !strings.Contains(s, "{{") {
		return s
	}
	src := ConvertSyntax(s)
	tmpl, err := template.New("").Funcs(funcMap).Option("missingkey=zero").Parse(src)
	if err != nil {
		return s
	}
	var buf bytes.Buffer
	if tmpl.Execute(&buf, nil) != nil {
		return s
	}
	return buf.String()
}

// RenderPayload 用 Go template 引擎渲染 payload。
//
// 支持两种语法（可混用）：
//
//	${KEY}                        → 变量引用
//	${funcName(arg1, arg2)}       → 内联函数调用
//	{{.KEY}} / {{funcName ...}}   → 原生 Go template 语法
func RenderPayload(payload json.RawMessage, varDefs json.RawMessage, vars map[string]string, now time.Time) json.RawMessage {
	data := buildTemplateData(varDefs, vars)
	src := ConvertSyntax(string(payload))

	tmpl, err := template.New("payload").
		Funcs(TemplateFuncMap(now)).
		Option("missingkey=zero").
		Parse(src)
	if err != nil {
		return payload
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return payload
	}
	return json.RawMessage(buf.Bytes())
}

func buildTemplateData(varDefs json.RawMessage, vars map[string]string) map[string]string {
	type varDef struct {
		Key          string `json:"key"`
		DefaultValue string `json:"default_value"`
	}

	data := make(map[string]string)
	var defs []varDef
	if json.Unmarshal(varDefs, &defs) == nil {
		for _, d := range defs {
			data[d.Key] = d.DefaultValue
		}
	}
	for k, v := range vars {
		data[k] = v
	}
	return data
}

// ---- Syntax Conversion ----

var reVarRef = regexp.MustCompile(`\$\{([^}]+)\}`)
var reFuncCall = regexp.MustCompile(`^(\w+)\(([^)]*)\)$`)

// ConvertSyntax 将 ${...} 语法转换为 Go template {{...}} 语法。
func ConvertSyntax(s string) string {
	return reVarRef.ReplaceAllStringFunc(s, func(match string) string {
		inner := match[2 : len(match)-1]

		if m := reFuncCall.FindStringSubmatch(inner); m != nil {
			funcName := m[1]
			argsStr := strings.TrimSpace(m[2])
			if argsStr == "" {
				return "{{" + funcName + "}}"
			}
			parts := strings.Split(argsStr, ",")
			var goArgs []string
			for _, p := range parts {
				goArgs = append(goArgs, `"`+strings.TrimSpace(p)+`"`)
			}
			return "{{" + funcName + " " + strings.Join(goArgs, " ") + "}}"
		}

		return "{{." + strings.TrimSpace(inner) + "}}"
	})
}

// ---- Offset & Layout Helpers ----

func applyOffset(t time.Time, offset string) (time.Time, error) {
	offset = strings.TrimSpace(offset)
	if offset == "" || offset == "0" || offset == "0d" {
		return t, nil
	}
	if len(offset) < 2 {
		return t, nil
	}

	unit := offset[len(offset)-1:]
	numStr := offset[:len(offset)-1]
	if numStr == "" || numStr == "+" {
		numStr += "0"
	}
	n, err := strconv.Atoi(numStr)
	if err != nil {
		return t, err
	}

	switch unit {
	case "d":
		return t.AddDate(0, 0, n), nil
	case "h":
		return t.Add(time.Duration(n) * time.Hour), nil
	case "m":
		return t.Add(time.Duration(n) * time.Minute), nil
	case "M":
		return t.AddDate(0, n, 0), nil
	case "y":
		return t.AddDate(n, 0, 0), nil
	default:
		return t, nil
	}
}

func javaToGoLayout(pattern string) string {
	r := strings.NewReplacer(
		"yyyy", "2006",
		"yy", "06",
		"MM", "01",
		"dd", "02",
		"HH", "15",
		"mm", "04",
		"ss", "05",
	)
	return r.Replace(pattern)
}
