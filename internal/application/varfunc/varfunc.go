package varfunc

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Override 定义调度级别的变量覆盖，支持固定值和函数。
//
//	type=fixed → 使用 Value 字段
//	type=date  → 使用 Format + Offset 在运行时计算
type Override struct {
	Key    string `json:"key"`
	Type   string `json:"type"`
	Value  string `json:"value,omitempty"`
	Format string `json:"format,omitempty"`
	Offset string `json:"offset,omitempty"`
}

// Resolve 将 overrides 解析为 key→value 字典。
// now 为基准时间（通常 time.Now()），用于日期函数计算。
func Resolve(overrides json.RawMessage, now time.Time) (map[string]string, error) {
	var items []Override
	if len(overrides) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(overrides, &items); err != nil {
		return nil, nil
	}
	if len(items) == 0 {
		return nil, nil
	}

	result := make(map[string]string, len(items))
	for _, item := range items {
		switch item.Type {
		case "fixed":
			result[item.Key] = item.Value
		case "date":
			t, err := applyOffset(now, item.Offset)
			if err != nil {
				return nil, fmt.Errorf("变量 %s 偏移量 %q 无效: %w", item.Key, item.Offset, err)
			}
			result[item.Key] = t.Format(javaToGoLayout(item.Format))
		default:
			result[item.Key] = item.Value
		}
	}
	return result, nil
}

// applyOffset 解析偏移量字符串并应用到时间上。
// 支持格式: "0d", "-1d", "+7d", "-1h", "-1M"（大写 M = 月）
func applyOffset(t time.Time, offset string) (time.Time, error) {
	offset = strings.TrimSpace(offset)
	if offset == "" || offset == "0" || offset == "0d" {
		return t, nil
	}

	if len(offset) < 2 {
		return t, fmt.Errorf("格式无效")
	}

	unit := offset[len(offset)-1:]
	numStr := offset[:len(offset)-1]
	if numStr == "" || numStr == "+" {
		numStr += "0"
	}
	n, err := strconv.Atoi(numStr)
	if err != nil {
		return t, fmt.Errorf("数字部分无效: %s", numStr)
	}

	switch unit {
	case "d":
		return t.AddDate(0, 0, n), nil
	case "h":
		return t.Add(time.Duration(n) * time.Hour), nil
	case "m":
		return t.Add(time.Duration(n) * time.Minute), nil
	case "M":
		return t.AddDate(0, n, 0), nil
	case "y":
		return t.AddDate(n, 0, 0), nil
	default:
		return t, fmt.Errorf("未知单位: %s", unit)
	}
}

// javaToGoLayout 将 Java 风格的日期格式转换为 Go 的 reference time layout。
func javaToGoLayout(pattern string) string {
	r := strings.NewReplacer(
		"yyyy", "2006",
		"yy", "06",
		"MM", "01",
		"dd", "02",
		"HH", "15",
		"mm", "04",
		"ss", "05",
	)
	return r.Replace(pattern)
}
