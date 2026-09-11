// Package service 共享工具函数。
package service

import (
	"context"
	"slices"
	"strconv"
	"time"

	"forklift-training/internal/clock"
)

// withTimeout 创建带超时的 context，封装以简化调用。
func withTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

// formatISO 时间 → API 契约串（ADR-0043）：**业务时区（Asia/Shanghai）墙钟 + 显式偏移 + 微秒定长**。
// 例：`2026-09-11T20:16:52.203824+08:00`。
//
// 三条约束，改动前务必读完：
//
//  1. **偏移量不可省**。历史实现输出**裸 UTC**（`2026-09-11T12:16:52.203824`，无任何标记），
//     而 JS `new Date()` 对不带时区标记的日期时间按**本地时区**解析——东八区用户读到的时刻
//     恒比真实值早 8 小时。列表里的绝对日期看不出异常，一旦渲染成相对时间（「8小时前」）
//     立刻暴露；日期边界处（北京 07:00 = UTC 前一日 23:00）还会显示错日期。
//  2. **用业务时区而非 UTC**。移动端论坛按 `substring` 直接截取展示（`formatDateStr` /
//     `formatDateTimeStr`），它不做任何时区换算——只有发出北京墙钟，那两处才显示正确。
//     此处若改回 `t.UTC()`，Web 仍正确但移动端会整体差 8 小时。
//  3. **偏移必须恒定**（中国无夏令时，Asia/Shanghai 恒 +08:00）。`student_service` 等处按
//     字符串**字典序**当时间序排序；偏移一旦逐值浮动，字典序即失效。
//
// 契约的完整说明见 docs/adr/ADR-0043；对外表述见 API.md「时间格式」。
func formatISO(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(clock.Location()).Format("2006-01-02T15:04:05.000000Z07:00")
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
