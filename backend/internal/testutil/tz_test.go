package testutil

import (
	"testing"
	"time"

	"forklift-training/internal/clock"
)

// TestLocalIsBusinessLocation 钉住 #2204 的修法：测试进程的 time.Local 必须是业务时区。
//
// 为什么值得单独守住：这条不是「风格」，而是日界计数能否判对的前提。sqlite 把 time.Time 存成
// 带偏移量的字面量、按字符串比较；只要 Local 退回 runner 的 UTC，clock.DayStart 的 +08 边界与
// 行时刻的 +00:00 表达就会在「上海 00:00–08:00」这 8 小时里把 `created_at >= todayStart` 判反，
// forum 日额度与 points 任务进度三条契约断言随之无差别假红（机制与取证见 tz.go 的文件头）。
//
// 判据用 clock.Location() 而不是硬编码 "Asia/Shanghai"：时区政策的单点在 clock 包，本处只问
// 「两边是否同一口径」。Windows 上 LoadLocation 返回的 *time.Location 名字可能是 "CST"，
// 故比偏移量而非字符串。
func TestLocalIsBusinessLocation(t *testing.T) {
	// 取一个「上海与 UTC 不同日」的时刻做对照，避免偏移量巧合相等时假绿。
	probe := time.Date(2026, 11, 2, 0, 30, 0, 0, clock.Location())
	want := clock.DayStart(probe)

	if got, wantLocal := time.Date(2026, 11, 2, 0, 30, 0, 0, time.Local), probe.In(time.Local); !got.Equal(wantLocal) {
		t.Fatalf("time.Local 不是业务时区：同一墙上时刻解析出 %v，业务时区应为 %v", got, wantLocal)
	}
	if got := want.In(time.Local); !got.Equal(want) {
		t.Fatalf("日界在 time.Local 下落在 %v，业务时区下应为 %v", got, want)
	}
	if name := time.Local.String(); name != clock.Location().String() {
		t.Logf("提示：time.Local=%q 与 clock.Location()=%q 名字不同，偏移一致即通过", name, clock.Location())
	}
}
