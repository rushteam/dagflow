package etl

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rushteam/dagflow/internal/infrastructure/tga"
)

type tgaSourcePayload struct {
	Type     string `json:"type"`
	BaseURL  string `json:"base_url"`
	Token    string `json:"token"`
	Insecure bool   `json:"insecure"`
	Timeout  int    `json:"timeout"` // 秒，默认 300
	Retries  int    `json:"retries"` // 默认 3
	SQL      string `json:"sql"`
}

type tgaSource struct {
	client *tga.Client
	sql    string
}

// NewTGASourceFactory 创建 TGA SourceFactory。
// 连接信息（base_url / token 等）从任务 payload 读取，每个任务可以指向不同的 TGA 实例。
func NewTGASourceFactory() SourceFactory {
	return func(taskCfg json.RawMessage) (Source, error) {
		var p tgaSourcePayload
		if err := json.Unmarshal(taskCfg, &p); err != nil {
			return nil, fmt.Errorf("解析 TGA source 配置失败: %w", err)
		}
		if p.BaseURL == "" {
			return nil, fmt.Errorf("TGA source: base_url 不能为空")
		}
		if p.SQL == "" {
			return nil, fmt.Errorf("TGA source: sql 不能为空")
		}

		timeout := time.Duration(p.Timeout) * time.Second
		if timeout <= 0 {
			timeout = 5 * time.Minute
		}

		client := tga.NewClient(tga.Config{
			BaseURL:  p.BaseURL,
			Token:    p.Token,
			Insecure: p.Insecure,
			Timeout:  timeout,
			Retries:  p.Retries,
		})

		return &tgaSource{client: client, sql: p.SQL}, nil
	}
}

func (s *tgaSource) Query(ctx context.Context, batchSize int, fn func(batch []Row) error) error {
	result, err := s.client.Query(ctx, s.sql)
	if err != nil {
		return err
	}
	if result.Code != 0 {
		return fmt.Errorf("TGA 查询返回错误: code=%d, message=%s", result.Code, result.Message)
	}

	batch := make([]Row, 0, batchSize)
	for _, row := range result.Rows {
		batch = append(batch, row)
		if len(batch) >= batchSize {
			if err := fn(batch); err != nil {
				return err
			}
			batch = make([]Row, 0, batchSize)
		}
	}
	if len(batch) > 0 {
		return fn(batch)
	}
	return nil
}
