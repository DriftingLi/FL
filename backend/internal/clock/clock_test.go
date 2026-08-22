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
