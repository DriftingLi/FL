// Package service 共享工具函数。
package service

import (
	"context"
	"encoding/json"
	"strconv"
	"time"
)

// withTimeout 创建带超时的 context，封装以简化调用。
func withTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

// formatISO 将时间格式化为 ISO8601 字符串。
func formatISO(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02T15:04:05.000000")
}

// toFloat 将任意类型转为 float64。
func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	case string:
		return parseFloat(n)
	case bool:
		if n {
			return 1
		}
		return 0
	}
	return 0
}

// clampFloat 将 v 限制在 [min, max] 区间。
func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// parseFloat 解析字符串为 float64，失败返回 0。
func parseFloat(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

// parseInt 解析字符串为 int，失败返回 0。
func parseInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}

// ptrInt 返回 int 指针。
func ptrInt(v int) *int { return &v }

// derefInt 安全解引用 *int，nil 返回 0。
func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// floatPtr 从 float64 构造指针。
func floatPtr(v float64) *float64 { return &v }

// containsString 判断切片是否包含字符串。
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// jsonMarshalImpl 是 jsonMarshal 包装函数的实现，直接调用标准库 json.Marshal。
// 单独抽出实现层是为了便于在测试中替换（如需 mock JSON 序列化）。
func jsonMarshalImpl(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// jsonUnmarshalImpl 是 jsonUnmarshal 包装函数的实现，直接调用标准库 json.Unmarshal。
func jsonUnmarshalImpl(b []byte, v interface{}) error {
	return json.Unmarshal(b, v)
}
