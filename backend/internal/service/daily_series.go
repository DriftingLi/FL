// Package service 实现业务服务层。
// 本文件：按天统计序列的单一组装实现（BuildDailySeries），学员学习统计与导师阅卷统计共用。
// 把 days 钳制（7/30）、startOfDay 归零、起点、补 0 序列、activeDays 计数、total 累加全藏进 implementation，
// 调用方只保留各自 SQL 聚合出的 day→count map（ADR-0013 候选 5）。
package service

import (
	"time"

	"forklift-training/internal/clock"
)

// dailySeriesStart 返回最近 days 天的起点（把 days 钳制、startOfDay 归零、起点推导藏在一起）。
// SQL 聚合方（study_date/graded_at >= start）与 BuildDailySeries 序列共用同一起点，避免两处漂移。
func dailySeriesStart(days int) time.Time {
	if days != 7 && days != 30 {
		days = 7
	}
	end := beijingNow()
	startOfDay := end.Add(-time.Duration(end.Hour()) * time.Hour).
		Add(-time.Duration(end.Minute()) * time.Minute).
		Add(-time.Duration(end.Second()) * time.Second).
		Add(-time.Duration(end.Nanosecond()) * time.Nanosecond)
	return startOfDay.AddDate(0, 0, -(days - 1))
}

// DailySeries 按天统计序列的组装结果。
type DailySeries struct {
	Days       int
	Labels     []string
	Data       []int64
	ActiveDays int
	Total      int64
}

// BuildDailySeries 由 day→count 映射组装最近 days 天的完整序列（含无记录的天补 0）。
// days 仅允许 7 或 30，其他值统一回退为 7；时区取自 beijingNow()（Asia/Shanghai）。
// total 为 byDay 全部计数累加；labels 格式 "1/2"（月/日，无前导零）。
func BuildDailySeries(days int, byDay map[string]int64) DailySeries {
	if days != 7 && days != 30 {
		days = 7
	}
	start := dailySeriesStart(days)

	// total 累加整份 byDay（与历史语义一致：SQL 按 study_date/graded_at >= start 聚合，
	// 可能含窗口之外（未来）的计数，这些计数计入 total 但不落在 days 序列内）。
	var total int64
	for _, cnt := range byDay {
		total += cnt
	}

	labels := make([]string, 0, days)
	data := make([]int64, 0, days)
	activeDays := 0
	// start 由 beijingNow() 派生，携带 Asia/Shanghai 时区，AddDate 保留时区。
	for i := 0; i < days; i++ {
		d := start.AddDate(0, 0, i)
		key := clock.DayKey(d)
		cnt := byDay[key]
		if cnt > 0 {
			activeDays++
		}
		labels = append(labels, d.Format("1/2"))
		data = append(data, cnt)
	}

	return DailySeries{
		Days:       days,
		Labels:     labels,
		Data:       data,
		ActiveDays: activeDays,
		Total:      total,
	}
}
