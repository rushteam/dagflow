package etl

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rushteam/dagflow/internal/application/executor"
)

type printSinkPayload struct {
	Type   string `json:"type"`
	Format string `json:"format"` // "json"（默认）| "table"
}

type printSink struct {
	format  string
	headers []string // table 模式下缓存列名
}

// NewPrintSinkFactory 创建 Print SinkFactory。
// 将数据打印到任务输出中，用于调试和验证 Source 查询结果。
func NewPrintSinkFactory() SinkFactory {
	return func(taskCfg json.RawMessage) (Sink, error) {
		var p printSinkPayload
		if err := json.Unmarshal(taskCfg, &p); err != nil {
			return nil, fmt.Errorf("解析 print sink 配置失败: %w", err)
		}
		f := p.Format
		if f == "" {
			f = "json"
		}
		return &printSink{format: f}, nil
	}
}

func (s *printSink) Write(ctx context.Context, batch []Row) error {
	switch s.format {
	case "table":
		return s.writeTable(ctx, batch)
	default:
		return s.writeJSON(ctx, batch)
	}
}

func (s *printSink) Close() error { return nil }

func (s *printSink) writeJSON(ctx context.Context, batch []Row) error {
	for _, row := range batch {
		b, err := json.Marshal(row)
		if err != nil {
			return err
		}
		executor.AppendOutput(ctx, string(b)+"\n")
	}
	return nil
}

func (s *printSink) writeTable(ctx context.Context, batch []Row) error {
	if len(batch) == 0 {
		return nil
	}

	if s.headers == nil {
		s.headers = sortedKeys(batch[0])
		executor.AppendOutput(ctx, strings.Join(s.headers, "\t")+"\n")
	}

	for _, row := range batch {
		vals := make([]string, len(s.headers))
		for i, h := range s.headers {
			vals[i] = fmt.Sprint(row[h])
		}
		executor.AppendOutput(ctx, strings.Join(vals, "\t")+"\n")
	}
	return nil
}

func sortedKeys(m Row) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}
