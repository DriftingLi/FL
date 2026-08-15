// Package service 共享工具函数。
package service

import (
	"context"
	"slices"
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
		// 宽容语义：字符串解析失败回退 0。
		f, _ := parseFloat(n)
		return f
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

// parseFloat 解析字符串为 float64，失败返回 strconv 原生错误。
func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// parseInt 解析字符串为 int，失败返回 strconv 原生错误。
func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// ptrInt 返回 int 指针。
func ptrInt(v int) *int { return &v }

// floatPtr 从 float64 构造指针。
func floatPtr(v float64) *float64 { return &v }

// containsString 判断切片是否包含字符串。
func containsString(slice []string, s string) bool {
	return slices.Contains(slice, s)
}

// formatTimePtr 格式化时间指针，nil 返回 nil。
func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := formatISO(*t)
	return &s
}
