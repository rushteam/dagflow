package etl

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/rushteam/dagflow/internal/application/executor"
)

const defaultBatchSize = 500

type etlPayload struct {
	Source    json.RawMessage `json:"source"`
	Sink      json.RawMessage `json:"sink"`
	BatchSize int             `json:"batch_size"`
}

// Executor 实现 "etl" Kind，将 Source 数据流式写入 Sink。
type Executor struct {
	registry *Registry
}

func NewExecutor(reg *Registry) *Executor {
	return &Executor{registry: reg}
}

// Info 返回注册到 executor.Registry 的元信息。
func (e *Executor) Info() executor.Info {
	return executor.Info{
		Name:  "etl",
		Label: "ETL 数据同步",
		PayloadHint: `{"source":{"type":"tga","base_url":"https://tga.example.com","token":"TOKEN","insecure":true,"sql":"SELECT uid,score FROM ..."},"sink":{"type":"redis","addr":"localhost:6379","command":"SET","key_template":"rec:{{.uid}}","value_field":"score","ttl":3600},"batch_size":500}`,
		Fn:    e.run,
	}
}

func extractType(raw json.RawMessage) (string, error) {
	var peek struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &peek); err != nil {
		return "", fmt.Errorf("解析 type 字段失败: %w", err)
	}
	if peek.Type == "" {
		return "", fmt.Errorf("缺少 type 字段")
	}
	return peek.Type, nil
}

func (e *Executor) run(ctx context.Context, payload json.RawMessage) error {
	var p etlPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("解析 etl payload 失败: %w", err)
	}

	batchSize := p.BatchSize
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}

	srcType, err := extractType(p.Source)
	if err != nil {
		return fmt.Errorf("source: %w", err)
	}
	src, err := e.registry.NewSource(srcType, p.Source)
	if err != nil {
		return fmt.Errorf("创建 source(%s) 失败: %w", srcType, err)
	}

	sinkType, err := extractType(p.Sink)
	if err != nil {
		return fmt.Errorf("sink: %w", err)
	}
	sink, err := e.registry.NewSink(sinkType, p.Sink)
	if err != nil {
		return fmt.Errorf("创建 sink(%s) 失败: %w", sinkType, err)
	}
	defer sink.Close()

	slog.InfoContext(ctx, "ETL 开始",
		"source", srcType, "sink", sinkType, "batch_size", batchSize)
	executor.AppendOutput(ctx, fmt.Sprintf("ETL: %s → %s (batch=%d)\n", srcType, sinkType, batchSize))

	var totalRows, totalBatches int

	err = src.Query(ctx, batchSize, func(batch []Row) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		totalBatches++
		totalRows += len(batch)

		if err := sink.Write(ctx, batch); err != nil {
			return fmt.Errorf("sink.Write batch#%d 失败: %w", totalBatches, err)
		}

		executor.AppendOutput(ctx, fmt.Sprintf("  batch#%d: %d rows (cumulative: %d)\n",
			totalBatches, len(batch), totalRows))
		return nil
	})
	if err != nil {
		return fmt.Errorf("ETL 执行失败: %w", err)
	}

	slog.InfoContext(ctx, "ETL 完成", "total_rows", totalRows, "batches", totalBatches)
	executor.AppendOutput(ctx, fmt.Sprintf("ETL 完成: %d rows, %d batches\n", totalRows, totalBatches))
	return nil
}
