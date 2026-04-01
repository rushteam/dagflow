package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/rushteam/dagflow/internal/infrastructure/database/gen"
)

// Payload 是回调 POST 请求的 JSON body。
type Payload struct {
	RunID      int64  `json:"run_id"`
	TaskID     int64  `json:"task_id"`
	TaskName   string `json:"task_name"`
	Status     string `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
	Output     string `json:"output,omitempty"`
	FinishedAt string `json:"finished_at"`
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

// FireMatched 从所有已启用的回调中筛选匹配当前任务和事件的，异步触发。
// 回调失败仅记录日志，不影响任务结果。
func FireMatched(callbacks []gen.Callback, taskID int64, p Payload) {
	for _, cb := range callbacks {
		if !matchTask(cb, taskID) {
			continue
		}
		if !matchEvent(cb.Events, p.Status) {
			continue
		}

		headers := parseHeaders(cb.Headers)
		go func(cbName, cbURL string) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := doPost(ctx, cbURL, headers, p); err != nil {
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

func matchTask(cb gen.Callback, taskID int64) bool {
	if cb.MatchMode == "all" {
		return true
	}
	var ids []int64
	if err := json.Unmarshal(cb.TaskIds, &ids); err != nil {
		return false
	}
	for _, id := range ids {
		if id == taskID {
			return true
		}
	}
	return false
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

func doPost(ctx context.Context, url string, headers map[string]string, p Payload) error {
	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("序列化回调 body: %w", err)
	}

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
