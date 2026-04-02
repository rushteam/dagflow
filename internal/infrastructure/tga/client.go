// Package tga 提供 TGA（数据查询平台）的 HTTP 客户端。
//
// TGA API 以 form-encoded POST 接受 SQL 查询，响应为多行 JSON：
// 第一行是 meta（return_code / return_message），后续每行一个 JSON 对象。
//
// 属于 DDD 基础设施层，不包含任何业务逻辑。
package tga

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config 是 TGA 客户端的连接配置。
type Config struct {
	BaseURL  string        `yaml:"base_url"`
	Token    string        `yaml:"token"`
	Timeout  time.Duration `yaml:"timeout"`
	Insecure bool          `yaml:"insecure"` // 跳过 TLS 证书校验
	Retries  int           `yaml:"retries"`
}

// Row 是一行查询结果（字段名 → 值）。
// 数字类型为 json.Number 以保留大整数精度。
type Row = map[string]any

// QueryResult 封装 TGA 查询结果。
type QueryResult struct {
	Code    int    // return_code，0 表示成功
	Message string // return_message
	Rows    []Row
}

// Client 是 TGA SQL 查询的 HTTP 客户端。线程安全，可共享。
type Client struct {
	baseURL    string
	token      string
	retries    int
	httpClient *http.Client
}

// NewClient 根据配置创建 TGA 客户端。
func NewClient(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Minute
	}
	retries := cfg.Retries
	if retries <= 0 {
		retries = 3
	}

	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   30 * time.Second,
		ResponseHeaderTimeout: timeout,
	}
	if cfg.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // TGA 使用自签名证书
	}

	return &Client{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		token:   cfg.Token,
		retries: retries,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
	}
}

// Query 执行 SQL 查询，自动重试。返回的 Row 中数字值为 json.Number。
func (c *Client) Query(ctx context.Context, sql string) (*QueryResult, error) {
	reqURL := fmt.Sprintf("%s/querySql?token=%s", c.baseURL, c.token)

	formData := url.Values{}
	formData.Set("sql", sql)
	formData.Set("format", "json_object")
	encoded := formData.Encode()

	var lastErr error
	for attempt := 1; attempt <= c.retries; attempt++ {
		result, err := c.doQuery(ctx, reqURL, encoded)
		if err == nil {
			return result, nil
		}
		lastErr = err

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if attempt < c.retries {
			wait := time.Duration(attempt*2) * time.Second
			slog.WarnContext(ctx, "TGA 查询失败，重试中",
				"attempt", attempt, "max", c.retries, "backoff", wait, "error", err)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
		}
	}
	return nil, fmt.Errorf("TGA 查询失败(重试%d次): %w", c.retries, lastErr)
}

func (c *Client) doQuery(ctx context.Context, reqURL, formBody string) (*QueryResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(formBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 512))
	}

	return parseResponse(body)
}

// parseResponse 解析 TGA 多行 JSON 响应。
// 第一行: {"return_code":0,"return_message":"ok",...}
// 后续行: 每行一个 JSON 对象（数据行）
func parseResponse(body []byte) (*QueryResult, error) {
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("响应为空")
	}

	var meta struct {
		Code    int    `json:"return_code"`
		Message string `json:"return_message"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &meta); err != nil {
		return nil, fmt.Errorf("解析 meta 行失败: %w", err)
	}

	result := &QueryResult{
		Code:    meta.Code,
		Message: meta.Message,
	}

	if meta.Code != 0 {
		return result, nil
	}

	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var row Row
		dec := json.NewDecoder(strings.NewReader(line))
		dec.UseNumber()
		if err := dec.Decode(&row); err != nil {
			return nil, fmt.Errorf("解析数据行 %d 失败: %w", i, err)
		}
		result.Rows = append(result.Rows, row)
	}

	return result, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
