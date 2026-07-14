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
	// 非零时间返回 ISO8601 格式
	ts := time.Date(2026, 6, 26, 14, 30, 0, 0, time.UTC)
	if got := formatISO(ts); got != "2026-06-26T14:30:00.000000" {
		t.Errorf("期望 '2026-06-26T14:30:00.000000'，得到 %q", got)
	}
	// 带微秒（500000 纳秒 = 500 微秒）
	ts2 := time.Date(2026, 1, 1, 0, 0, 0, 500000, time.UTC)
	if got := formatISO(ts2); got != "2026-01-01T00:00:00.000500" {
		t.Errorf("期望 '2026-01-01T00:00:00.000500'，得到 %q", got)
	}
}

func TestToFloat(t *testing.T) {
	tests := []struct {
		input interface{}
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
	if got := parseFloat("3.14"); got != 3.14 {
		t.Errorf("parseFloat('3.14') = %v", got)
	}
	if got := parseFloat("invalid"); got != 0 {
		t.Errorf("parseFloat('invalid') = %v，期望 0", got)
	}
	if got := parseFloat(""); got != 0 {
		t.Errorf("parseFloat('') = %v，期望 0", got)
	}
	if got := parseFloat("-5.5"); got != -5.5 {
		t.Errorf("parseFloat('-5.5') = %v", got)
	}
}

func TestParseInt(t *testing.T) {
	if got := parseInt("42"); got != 42 {
		t.Errorf("parseInt('42') = %v", got)
	}
	if got := parseInt("invalid"); got != 0 {
		t.Errorf("parseInt('invalid') = %v，期望 0", got)
	}
	if got := parseInt(""); got != 0 {
		t.Errorf("parseInt('') = %v，期望 0", got)
	}
	if got := parseInt("-7"); got != -7 {
		t.Errorf("parseInt('-7') = %v", got)
	}
}

func TestPtrAndDerefInt(t *testing.T) {
	p := ptrInt(42)
	if p == nil || *p != 42 {
		t.Errorf("ptrInt(42) 失败")
	}
	if got := derefInt(p); got != 42 {
		t.Errorf("derefInt(ptrInt(42)) = %v", got)
	}
	if got := derefInt(nil); got != 0 {
		t.Errorf("derefInt(nil) = %v，期望 0", got)
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
