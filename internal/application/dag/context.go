package dag

import "context"

type ctxKey struct{}
type varsCtxKey struct{}

// WithRunID 将 DAG 父 run ID 注入到 context 中。
func WithRunID(ctx context.Context, runID int64) context.Context {
	return context.WithValue(ctx, ctxKey{}, runID)
}

// runIDFromContext 从 context 中提取 DAG 父 run ID。
func runIDFromContext(ctx context.Context) int64 {
	v, _ := ctx.Value(ctxKey{}).(int64)
	return v
}

// WithVars 将 DAG 级别的变量注入到 context，供子任务继承。
func WithVars(ctx context.Context, vars map[string]string) context.Context {
	if len(vars) == 0 {
		return ctx
	}
	return context.WithValue(ctx, varsCtxKey{}, vars)
}

// VarsFromContext 从 context 中提取 DAG 传递的变量。
func VarsFromContext(ctx context.Context) map[string]string {
	v, _ := ctx.Value(varsCtxKey{}).(map[string]string)
	return v
}
