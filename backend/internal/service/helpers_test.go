package service

import (
	"testing"
	"time"
)

func TestFormatISO(t *testing.T) {
	// 零时间返回空字符串
	if got := formatISO(time.Time{}); got != "" {
		t.Errorf("零时间期望 ''，得到 %q", got)
	}
	// 非零时间返回 RFC3339，**带显式偏移、且墙钟为业务时区（Asia/Shanghai）**（ADR-0043）
	// 14:30 UTC = 北京 22:30 —— 输出必须是北京墙钟 + +08:00，不能是 14:30。
	ts := time.Date(2026, 6, 26, 14, 30, 0, 0, time.UTC)
	if got := formatISO(ts); got != "2026-06-26T22:30:00.000000+08:00" {
		t.Errorf("期望 '2026-06-26T22:30:00.000000+08:00'，得到 %q", got)
	}
	// 带微秒（500000 纳秒 = 500 微秒）：00:00:00.0005 UTC = 北京 08:00:00.0005
	ts2 := time.Date(2026, 1, 1, 0, 0, 0, 500000, time.UTC)
	if got := formatISO(ts2); got != "2026-01-01T08:00:00.000500+08:00" {
		t.Errorf("期望 '2026-01-01T08:00:00.000500+08:00'，得到 %q", got)
	}
	// 已是北京时区的输入原样输出（不做二次换算）
	shanghai := time.FixedZone("CST", 8*3600)
	ts3 := time.Date(2026, 9, 11, 20, 16, 52, 0, shanghai)
	if got := formatISO(ts3); got != "2026-09-11T20:16:52.000000+08:00" {
		t.Errorf("期望 '2026-09-11T20:16:52.000000+08:00'，得到 %q", got)
	}
	// 跨日边界：北京 07:00（= UTC 前一日 23:00）必须落在**当日**，不能退到前一天
	ts4 := time.Date(2026, 9, 10, 23, 0, 0, 0, time.UTC)
	if got := formatISO(ts4); got != "2026-09-11T07:00:00.000000+08:00" {
		t.Errorf("期望 '2026-09-11T07:00:00.000000+08:00'，得到 %q", got)
	}
	// 偏移恒定 → 字典序即时间序（student_service 等处按字符串排序，依赖此性质）
	early := formatISO(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	late := formatISO(time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC))
	if !(early < late) {
		t.Errorf("字典序失效：%q 应 < %q", early, late)
	}
}

func TestToFloat(t *testing.T) {
	tests := []struct {
		input any
		want  float64
	}{
		{float64(3.14), 3.14},
		{float32(2.5), 2.5},
		{int(42), 42},
		{int64(100), 100},
		{int32(7), 7},
		{"3.14", 3.14},
		{"42", 42},
		{true, 1},
		{false, 0},
		{"invalid", 0},
		{nil, 0},
		{[]int{1, 2}, 0}, // 不支持的类型
	}
	for _, tt := range tests {
		if got := toFloat(tt.input); got != tt.want {
			t.Errorf("toFloat(%v) = %v，期望 %v", tt.input, got, tt.want)
		}
	}
}

func TestClampFloat(t *testing.T) {
	if got := clampFloat(5, 0, 10); got != 5 {
		t.Errorf("clampFloat(5,0,10) = %v，期望 5", got)
	}
	if got := clampFloat(-1, 0, 10); got != 0 {
		t.Errorf("clampFloat(-1,0,10) = %v，期望 0", got)
	}
	if got := clampFloat(15, 0, 10); got != 10 {
		t.Errorf("clampFloat(15,0,10) = %v，期望 10", got)
	}
	if got := clampFloat(0, 0, 10); got != 0 {
		t.Errorf("clampFloat(0,0,10) = %v，期望 0", got)
	}
	if got := clampFloat(10, 0, 10); got != 10 {
		t.Errorf("clampFloat(10,0,10) = %v，期望 10", got)
	}
}

func TestParseFloat(t *testing.T) {
	if got, err := parseFloat("3.14"); err != nil || got != 3.14 {
		t.Errorf("parseFloat('3.14') = %v, err=%v", got, err)
	}
	if got, err := parseFloat("-5.5"); err != nil || got != -5.5 {
		t.Errorf("parseFloat('-5.5') = %v, err=%v", got, err)
	}
	for _, s := range []string{"invalid", "", "abc12"} {
		if got, err := parseFloat(s); err == nil {
			t.Errorf("parseFloat(%q) 期望报错，得到 %v", s, got)
		}
	}
}

func TestParseInt(t *testing.T) {
	if got, err := parseInt("42"); err != nil || got != 42 {
		t.Errorf("parseInt('42') = %v, err=%v", got, err)
	}
	if got, err := parseInt("-7"); err != nil || got != -7 {
		t.Errorf("parseInt('-7') = %v, err=%v", got, err)
	}
	for _, s := range []string{"invalid", "", "3.14"} {
		if got, err := parseInt(s); err == nil {
			t.Errorf("parseInt(%q) 期望报错，得到 %v", s, got)
		}
	}
}

func TestPtrInt(t *testing.T) {
	p := ptrInt(42)
	if p == nil || *p != 42 {
		t.Errorf("ptrInt(42) 失败")
	}
}

func TestFloatPtr(t *testing.T) {
	p := floatPtr(3.14)
	if p == nil || *p != 3.14 {
		t.Errorf("floatPtr(3.14) 失败")
	}
}

func TestContainsString(t *testing.T) {
	slice := []string{"apple", "banana", "cherry"}
	if !containsString(slice, "banana") {
		t.Error("应包含 'banana'")
	}
	if containsString(slice, "grape") {
		t.Error("不应包含 'grape'")
	}
	if containsString([]string{}, "x") {
		t.Error("空切片应返回 false")
	}
	if !containsString(slice, "apple") {
		t.Error("应包含 'apple'")
	}
}

func TestOrDefault(t *testing.T) {
	if got := orDefault("hello", "default"); got != "hello" {
		t.Errorf("orDefault('hello','default') = %v", got)
	}
	if got := orDefault("", "default"); got != "default" {
		t.Errorf("orDefault('','default') = %v，期望 'default'", got)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello world", 5); got != "hello" {
		t.Errorf("truncate('hello world',5) = %q，期望 'hello'", got)
	}
	if got := truncate("hi", 10); got != "hi" {
		t.Errorf("truncate('hi',10) = %q，期望 'hi'", got)
	}
	if got := truncate("", 5); got != "" {
		t.Errorf("truncate('',5) = %q", got)
	}
}

func TestFormatTimePtr(t *testing.T) {
	// nil 指针返回 nil
	if got := formatTimePtr(nil); got != nil {
		t.Errorf("formatTimePtr(nil) 期望 nil，得到 %v", got)
	}
	// 非零时间返回按 formatISO 格式化的字符串指针
	ts := time.Date(2026, 6, 26, 14, 30, 0, 0, time.UTC)
	p := formatTimePtr(&ts)
	if p == nil {
		t.Fatal("formatTimePtr(非零时间) 不应返回 nil")
	}
	if *p != "2026-06-26T22:30:00.000000+08:00" {
		t.Errorf("期望 '2026-06-26T22:30:00.000000+08:00'，得到 %q", *p)
	}
	// 零时间返回空字符串指针（沿用 formatISO 契约）
	zero := time.Time{}
	pz := formatTimePtr(&zero)
	if pz == nil || *pz != "" {
		t.Errorf("formatTimePtr(零时间) 期望空字符串指针，得到 %v", pz)
	}
}

func TestWithTimeout(t *testing.T) {
	ctx, cancel := withTimeout(5 * time.Second)
	defer cancel()
	if ctx == nil {
		t.Fatal("context 不应为 nil")
	}
	_, ok := ctx.Deadline()
	if !ok {
		t.Error("context 应有截止时间")
	}
}

func TestBeijingNow(t *testing.T) {
	now := beijingNow()
	if now.IsZero() {
		t.Error("beijingNow 不应返回零值")
	}
	// 北京时间应为 UTC+8
	_, offset := now.Zone()
	if offset != 8*3600 {
		t.Errorf("北京时间偏移应为 28800 秒(UTC+8)，得到 %d", offset)
	}
}
