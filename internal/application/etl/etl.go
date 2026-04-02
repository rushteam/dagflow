package etl

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Row 是一行数据记录，字段名 → 值。
type Row = map[string]any

// Source 从外部系统读取数据，按批次流式回调。
type Source interface {
	Query(ctx context.Context, batchSize int, fn func(batch []Row) error) error
}

// Sink 将批量数据写入外部系统。
type Sink interface {
	Write(ctx context.Context, batch []Row) error
	Close() error
}

// SourceFactory 根据任务级 payload 创建 Source 实例。
type SourceFactory func(cfg json.RawMessage) (Source, error)

// SinkFactory 根据任务级 payload 创建 Sink 实例。
type SinkFactory func(cfg json.RawMessage) (Sink, error)

// Registry 管理 Source / Sink 工厂。
type Registry struct {
	mu      sync.RWMutex
	sources map[string]SourceFactory
	sinks   map[string]SinkFactory
}

func NewRegistry() *Registry {
	return &Registry{
		sources: make(map[string]SourceFactory),
		sinks:   make(map[string]SinkFactory),
	}
}

func (r *Registry) RegisterSource(name string, f SourceFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sources[name] = f
}

func (r *Registry) RegisterSink(name string, f SinkFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sinks[name] = f
}

func (r *Registry) NewSource(name string, cfg json.RawMessage) (Source, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.sources[name]
	if !ok {
		return nil, fmt.Errorf("未知 source 类型: %q", name)
	}
	return f(cfg)
}

func (r *Registry) NewSink(name string, cfg json.RawMessage) (Sink, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.sinks[name]
	if !ok {
		return nil, fmt.Errorf("未知 sink 类型: %q", name)
	}
	return f(cfg)
}
