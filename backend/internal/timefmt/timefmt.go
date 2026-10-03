// Package timefmt 是 API 契约时间格式的**单点**（ADR-0043）：业务时区墙钟 + 显式偏移 + 微秒定长。
//
// 为什么是独立叶子包：域包（internal/<域>）与残余的 internal/core 都要输出同一套时间串，
// 而残余 internal/core 里仍有引用域包的代码（站内信写入等）。格式函数若留在
// internal/core，域包 import 它就会与该引用形成 import 环，故收到只依赖
// internal/clock 与标准库的叶子里。
package timefmt

import (
	"time"

	"forklift-training/internal/clock"
)

// FormatISO 时间 → API 契约串（ADR-0043）：**业务时区（Asia/Shanghai）墙钟 + 显式偏移 + 微秒定长**。
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
func FormatISO(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(clock.Location()).Format("2006-01-02T15:04:05.000000Z07:00")
}

// FormatTimePtr 格式化时间指针，nil 返回 nil。
func FormatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := FormatISO(*t)
	return &s
}
