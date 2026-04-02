package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

// Func 根据 payload 执行特定类型的任务。
// 每个 Kind 对应一个 Func 实现。
type Func func(ctx context.Context, payload json.RawMessage) error

// Info 描述一种任务类型（Kind）的元信息 + 执行函数。
type Info struct {
	Name        string // 唯一标识，如 "shell"、"http"
	Label       string // 显示名，如 "Shell 命令"
	PayloadHint string // payload JSON 示例，供前端展示
	Fn          Func   // 执行函数
}

// OutputCollector 线程安全的执行输出收集器，通过 context 传递给 executor。
type OutputCollector struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

const maxOutputSize = 256 * 1024 // 256KB

// Write 追加输出内容（带大小限制）。
func (c *OutputCollector) Write(s string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.buf.Len() >= maxOutputSize {
		return
	}
	c.buf.WriteString(s)
}

// Writef 格式化追加输出内容。
func (c *OutputCollector) Writef(format string, args ...any) {
	c.Write(fmt.Sprintf(format, args...))
}

// String 返回收集到的全部输出。
func (c *OutputCollector) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

// RunMeta 通过 context 传递给 executor 的运行元数据。
type RunMeta struct {
	RunID    int64
	TaskName string
}

type runMetaCtxKey struct{}

// WithRunMeta 将运行元数据写入 context。
func WithRunMeta(ctx context.Context, m RunMeta) context.Context {
	return context.WithValue(ctx, runMetaCtxKey{}, m)
}

// GetRunMeta 从 context 中取回运行元数据；不存在时返回零值。
func GetRunMeta(ctx context.Context) (RunMeta, bool) {
	m, ok := ctx.Value(runMetaCtxKey{}).(RunMeta)
	return m, ok
}

type outputCtxKey struct{}

// WithOutput 创建带 OutputCollector 的 context。
func WithOutput(ctx context.Context) (context.Context, *OutputCollector) {
	c := &OutputCollector{}
	return context.WithValue(ctx, outputCtxKey{}, c), c
}

// AppendOutput 向 context 中的 OutputCollector 追加内容。
// 如果 context 中没有 collector 则静默忽略。
func AppendOutput(ctx context.Context, s string) {
	if c, ok := ctx.Value(outputCtxKey{}).(*OutputCollector); ok {
		c.Write(s)
	}
}

// Registry 管理所有可用的 Kind。
type Registry struct {
	mu    sync.RWMutex
	kinds map[string]Info
}

func NewRegistry() *Registry {
	return &Registry{kinds: make(map[string]Info)}
}

// Register 注册一种 Kind。重复注册会覆盖。
func (r *Registry) Register(info Info) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.kinds[info.Name] = info
}

// Get 按 name 查找 Kind。
func (r *Registry) Get(name string) (Info, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	k, ok := r.kinds[name]
	return k, ok
}

// List 返回所有已注册 Kind，按 Name 排序。
func (r *Registry) List() []Info {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Info, 0, len(r.kinds))
	for _, k := range r.kinds {
		result = append(result, k)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}
