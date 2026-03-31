package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

var HTTP = Info{
	Name:  "http",
	Label: "HTTP 请求",
	PayloadHint: `{"url": "https://api.example.com/endpoint", "method": "POST", ` +
		`"headers": {"Content-Type": "application/json"}, "body": "{\"key\": \"value\"}", "timeout": 30}`,
	Fn: httpFn,
}

type httpPayload struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	Timeout int               `json:"timeout"` // 秒，默认 30
}

func httpFn(ctx context.Context, payload json.RawMessage) error {
	var p httpPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("解析 http payload 失败: %w", err)
	}
	if p.URL == "" {
		return fmt.Errorf("url 不能为空")
	}
	if p.Method == "" {
		p.Method = "GET"
	}
	timeout := 30
	if p.Timeout > 0 {
		timeout = p.Timeout
	}

	url := ExpandEnv(p.URL)
	body := ExpandEnv(p.Body)
	headers := make(map[string]string, len(p.Headers))
	for k, v := range p.Headers {
		headers[k] = ExpandEnv(v)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(p.Method), url, reqBody)
	if err != nil {
		return fmt.Errorf("创建 HTTP 请求失败: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	slog.InfoContext(ctx, "发送 HTTP 请求", "method", req.Method, "url", url)
	AppendOutput(ctx, fmt.Sprintf("%s %s\n", req.Method, url))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	respText := strings.TrimSpace(string(respBody))

	slog.InfoContext(ctx, "HTTP 响应",
		"status", resp.StatusCode, "body_preview", Truncate(respText, 512))

	AppendOutput(ctx, fmt.Sprintf("HTTP %d\n%s\n", resp.StatusCode, Truncate(respText, 4096)))

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, Truncate(respText, 256))
	}
	return nil
}
