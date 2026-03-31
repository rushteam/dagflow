package executor

import "os"

// ExpandEnv 将字符串中的 ${VAR} 替换为环境变量值。
func ExpandEnv(s string) string {
	return os.Expand(s, os.Getenv)
}

// Truncate 截断字符串到 n 字节，超出部分以 "..." 表示。
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
