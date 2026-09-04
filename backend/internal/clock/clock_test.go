package clock

import (
	"testing"
	"time"
)

func TestRealNowInBusinessLocation(t *testing.T) {
	got := Real().Now()
	if got.Location() != Location() {
		t.Fatalf("Real 时钟应返回 Asia/Shanghai 时区, got %v", got.Location())
	}
	if diff := time.Since(got); diff < -time.Minute || diff > time.Minute {
		t.Fatalf("Real 时钟偏差过大: %v", diff)
	}
}

func TestLocationIsAsiaShanghai(t *testing.T) {
	if got := Location().String(); got != "Asia/Shanghai" && got != "CST" {
		t.Fatalf("业务时区应为 Asia/Shanghai（或退化 CST）, got %q", got)
	}
}

func TestPackageNowUsesBusinessLocation(t *testing.T) {
	if got := Now(); got.Location() != Location() {
		t.Fatalf("包级 Now 应返回业务时区, got %v", got.Location())
	}
}

func TestFakeAtFrozen(t *testing.T) {
	frozen := time.Date(2026, 8, 20, 15, 4, 5, 0, Location())
	f := At(frozen)
	for i := 0; i < 3; i++ {
		if got := f.Now(); !got.Equal(frozen) {
			t.Fatalf("Fake 应定格, want %v got %v", frozen, got)
		}
		time.Sleep(time.Millisecond)
	}
	// Fake 可变推进，供跨日场景复用同一实例。
	f.T = frozen.AddDate(0, 0, 1)
	if got := f.Now(); got.Equal(frozen) {
		t.Fatalf("推进 Fake 后应返回新时刻, got %v", got)
	}
}

func TestDayStart(t *testing.T) {
	// 业务时区内的非 0 点时刻 → 当日 00:00。
	noon := time.Date(2026, 8, 20, 15, 4, 5, 0, Location())
	want := time.Date(2026, 8, 20, 0, 0, 0, 0, Location())
	if got := DayStart(noon); !got.Equal(want) {
		t.Fatalf("DayStart 应返回上海当日 00:00, want %v got %v", want, got)
	}
	// UTC 入参（上海已是次日）也要按业务时区归一。
	utcEvening := time.Date(2026, 8, 20, 0, 30, 0, 0, time.UTC) // 上海 08:30
	if got := DayStart(utcEvening); !got.Equal(want) {
		t.Fatalf("DayStart 应把 UTC 入参归一为上海自然日, want %v got %v", want, got)
	}
	// 午前时刻 → 同为当日 00:00（不漂移到前一天）。
	early := time.Date(2026, 8, 20, 0, 1, 0, 0, Location())
	if got := DayStart(early); !got.Equal(want) {
		t.Fatalf("DayStart 午前瞬间应仍归当日, want %v got %v", want, got)
	}
	if got := DayStart(noon); got.Location() != Location() {
		t.Fatalf("DayStart 应保留业务时区, got %v", got.Location())
	}
}

func TestDayKey(t *testing.T) {
	noon := time.Date(2026, 8, 20, 15, 4, 5, 0, Location())
	if got := DayKey(noon); got != "2026-08-20" {
		t.Fatalf("DayKey 应输出 2006-01-02, got %q", got)
	}
	// UTC 入参归一。
	utcEvening := time.Date(2026, 8, 20, 0, 30, 0, 0, time.UTC)
	if got := DayKey(utcEvening); got != "2026-08-20" {
		t.Fatalf("DayKey 应把 UTC 入参归一为上海日期, got %q", got)
	}
}
