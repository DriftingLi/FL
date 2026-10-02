package timefmt

import (
	"testing"
	"time"
)

func TestFormatISO(t *testing.T) {
	// 零时间返回空字符串
	if got := FormatISO(time.Time{}); got != "" {
		t.Errorf("零时间期望 ''，得到 %q", got)
	}
	// 非零时间返回 RFC3339，**带显式偏移、且墙钟为业务时区（Asia/Shanghai）**（ADR-0043）
	// 14:30 UTC = 北京 22:30 —— 输出必须是北京墙钟 + +08:00，不能是 14:30。
	ts := time.Date(2026, 6, 26, 14, 30, 0, 0, time.UTC)
	if got := FormatISO(ts); got != "2026-06-26T22:30:00.000000+08:00" {
		t.Errorf("期望 '2026-06-26T22:30:00.000000+08:00'，得到 %q", got)
	}
	// 带微秒（500000 纳秒 = 500 微秒）：00:00:00.0005 UTC = 北京 08:00:00.0005
	ts2 := time.Date(2026, 1, 1, 0, 0, 0, 500000, time.UTC)
	if got := FormatISO(ts2); got != "2026-01-01T08:00:00.000500+08:00" {
		t.Errorf("期望 '2026-01-01T08:00:00.000500+08:00'，得到 %q", got)
	}
	// 已是北京时区的输入原样输出（不做二次换算）
	shanghai := time.FixedZone("CST", 8*3600)
	ts3 := time.Date(2026, 9, 11, 20, 16, 52, 0, shanghai)
	if got := FormatISO(ts3); got != "2026-09-11T20:16:52.000000+08:00" {
		t.Errorf("期望 '2026-09-11T20:16:52.000000+08:00'，得到 %q", got)
	}
	// 跨日边界：北京 07:00（= UTC 前一日 23:00）必须落在**当日**，不能退到前一天
	ts4 := time.Date(2026, 9, 10, 23, 0, 0, 0, time.UTC)
	if got := FormatISO(ts4); got != "2026-09-11T07:00:00.000000+08:00" {
		t.Errorf("期望 '2026-09-11T07:00:00.000000+08:00'，得到 %q", got)
	}
	// 偏移恒定 → 字典序即时间序（student_service 等处按字符串排序，依赖此性质）
	early := FormatISO(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	late := FormatISO(time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC))
	if !(early < late) {
		t.Errorf("字典序失效：%q 应 < %q", early, late)
	}
}

func TestFormatTimePtr(t *testing.T) {
	// nil 指针返回 nil
	if got := FormatTimePtr(nil); got != nil {
		t.Errorf("FormatTimePtr(nil) 期望 nil，得到 %v", got)
	}
	// 非零时间返回按 FormatISO 格式化的字符串指针
	ts := time.Date(2026, 6, 26, 14, 30, 0, 0, time.UTC)
	p := FormatTimePtr(&ts)
	if p == nil {
		t.Fatal("FormatTimePtr(非零时间) 不应返回 nil")
	}
	if *p != "2026-06-26T22:30:00.000000+08:00" {
		t.Errorf("期望 '2026-06-26T22:30:00.000000+08:00'，得到 %q", *p)
	}
	// 零时间返回空字符串指针（沿用 FormatISO 契约）
	zero := time.Time{}
	pz := FormatTimePtr(&zero)
	if pz == nil || *pz != "" {
		t.Errorf("FormatTimePtr(零时间) 期望空字符串指针，得到 %v", pz)
	}
}
