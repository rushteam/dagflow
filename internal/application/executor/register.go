package executor

// RegisterBuiltin 注册所有内置执行器。
// 新增执行器时在此处追加一行即可。
func RegisterBuiltin(r *Registry) {
	r.Register(Shell)
	r.Register(HTTP)
}
