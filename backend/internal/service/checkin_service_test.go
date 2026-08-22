// Package service 每日打卡服务测试：时间经 internal/clock Fake 定格（spec #296），
// 覆盖签到幂等、日历 BETWEEN、连击起点、排行榜 tie-break 与 Me 名次合并、400 天截断窗口。
package service

import (
	"testing"
	"time"

	"go.uber.org/zap"

	"forklift-training/internal/clock"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// shDay 业务时区（Asia/Shanghai）某日 00:00。
func shDay(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, clock.Location())
}

// newCheckInSvcAt 构造内存库 + 定格时钟服务，返回 Fake 以便跨日推进。
func newCheckInSvcAt(t *testing.T, now time.Time) (*CheckInService, *clock.Fake) {
	t.Helper()
	f := clock.At(now)
	svc := NewCheckInService(testutil.NewMemoryDB(t), zap.NewNop(), f)
	return svc, f
}

func TestNewCheckInService_NilClockFallsBackToReal(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewCheckInService(db, zap.NewNop(), nil)
	if svc == nil || svc.clk == nil {
		t.Fatal("nil Clock 应回退生产实钟，svc/clk 不应为 nil")
	}
}

func TestComputeStreakMetrics(t *testing.T) {
	now := shDay(2026, 8, 15)
	cases := []struct {
		name       string
		dates      []time.Time
		wantStreak int
		wantToday  bool
		wantTotal  int
	}{
		{"空集合", nil, 0, false, 0},
		{"仅今日", []time.Time{now}, 1, true, 1},
		{"今日+昨日+前日连续", []time.Time{now, now.AddDate(0, 0, -1), now.AddDate(0, 0, -2)}, 3, true, 3},
		{"仅昨日（今日未签）", []time.Time{now.AddDate(0, 0, -1)}, 1, false, 1},
		{"缺昨日/今日均为空且昨日缺失→断开", []time.Time{now.AddDate(0, 0, -2), now.AddDate(0, 0, -3)}, 0, false, 2},
		{"乱序+重复去重", []time.Time{now.AddDate(0, 0, -1), now, now, now.AddDate(0, 0, -2)}, 3, true, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			streak, total, todayChecked := ComputeStreakMetrics(c.dates, now)
			if streak != c.wantStreak || todayChecked != c.wantToday || total != c.wantTotal {
				t.Fatalf("streak=%d total=%d today=%v want %d/%d/%v", streak, total, todayChecked, c.wantStreak, c.wantTotal, c.wantToday)
			}
		})
	}
}

func TestCheckIn_IdempotentAndCrossDay(t *testing.T) {
	day1 := shDay(2026, 8, 10).Add(10 * time.Hour)
	svc, fake := newCheckInSvcAt(t, day1)

	u := testutil.SeedStudent(t, svc.db, "打卡生", "x")

	r, _ := svc.CheckIn(u.ID)
	if r == nil || !r.TodayChecked || r.Streak != 1 || r.Total != 1 {
		t.Fatalf("day1 首签 streak/total/todayChecked 应 1/1/true, got %+v", r)
	}
	// 同日幂等：不再新增行
	r, _ = svc.CheckIn(u.ID)
	if r.Total != 1 {
		t.Fatalf("同日重签 total 应保持 1, got %+v", r)
	}

	// 连续次日
	fake.T = shDay(2026, 8, 11).Add(10 * time.Hour)
	r, _ = svc.CheckIn(u.ID)
	if r.Streak != 2 || r.Total != 2 || !r.TodayChecked {
		t.Fatalf("day2 连续 streak/total 应 2/2, got %+v", r)
	}

	// 跳日（8-13，跳过 8-12）：连击重置
	fake.T = shDay(2026, 8, 13).Add(10 * time.Hour)
	r, _ = svc.CheckIn(u.ID)
	if r.Streak != 1 || r.Total != 3 {
		t.Fatalf("跳日后 streak 应重置为 1 total=3, got %+v", r)
	}
}

func TestGetCheckInCalendar_BETWEEN(t *testing.T) {
	svc, _ := newCheckInSvcAt(t, shDay(2026, 8, 20))
	u := testutil.SeedStudent(t, svc.db, "日历生", "x")

	for _, d := range []time.Time{shDay(2026, 8, 1), shDay(2026, 8, 15), shDay(2026, 7, 31)} {
		svc.db.Create(&model.ForumCheckIn{UserID: u.ID, CheckDate: d, CreatedAt: d})
	}

	cal, err := svc.GetCheckInCalendar(u.ID, 2026, 8)
	if err != nil {
		t.Fatalf("日历查询失败: %v", err)
	}
	if len(cal.Dates) != 2 {
		t.Fatalf("2026-08 日历应恰 2 天, got %v", cal.Dates)
	}
	if cal.Dates[0] != "2026-08-01" || cal.Dates[1] != "2026-08-15" {
		t.Fatalf("日历日期与期望不符: %v", cal.Dates)
	}
	// total 为全生命周期计数（不受月窗口限制），含 7-31
	if cal.Total != 3 {
		t.Fatalf("日历 total 应为全量 3, got %d", cal.Total)
	}
}

func TestGetCheckInRank_TieBreakAndMe(t *testing.T) {
	svc, _ := newCheckInSvcAt(t, shDay(2026, 8, 20))

	userB := testutil.SeedStudent(t, svc.db, "用户B", "x") // total 3, 最早满贯→排前
	userA := testutil.SeedStudent(t, svc.db, "用户A", "x") // total 3, 较晚满贯→排后
	userC := testutil.SeedStudent(t, svc.db, "用户C", "x") // total 1

	// A: 8-14..16（最后一天 16），B: 8-12..14（最后一天 14），C: 8-20
	for _, d := range []time.Time{shDay(2026, 8, 14), shDay(2026, 8, 15), shDay(2026, 8, 16)} {
		svc.db.Create(&model.ForumCheckIn{UserID: userA.ID, CheckDate: d, CreatedAt: d})
	}
	for _, d := range []time.Time{shDay(2026, 8, 12), shDay(2026, 8, 13), shDay(2026, 8, 14)} {
		svc.db.Create(&model.ForumCheckIn{UserID: userB.ID, CheckDate: d, CreatedAt: d})
	}
	svc.db.Create(&model.ForumCheckIn{UserID: userC.ID, CheckDate: shDay(2026, 8, 20), CreatedAt: shDay(2026, 8, 20)})

	rank, err := svc.GetCheckInRank(userC.ID, 1, 20)
	if err != nil {
		t.Fatalf("排行榜查询失败: %v", err)
	}
	if rank.Total != 3 || len(rank.Items) != 3 {
		t.Fatalf("排行榜总人数/条目应 3, got %d/%d", rank.Total, len(rank.Items))
	}
	// tie-break：同 total 下 last_date ASC，故 B 排 1、A 排 2
	if rank.Items[0].User.UserID != userB.ID || rank.Items[1].User.UserID != userA.ID {
		t.Fatalf("tie-break 排序与期望不符（B 前于 A）: %+v", rank.Items)
	}
	if rank.Items[0].Rank != 1 || rank.Items[1].Rank != 2 {
		t.Fatalf("排名序号应 1/2, got %d/%d", rank.Items[0].Rank, rank.Items[1].Rank)
	}
	// Me：请求方 C total=1 排第 3
	if rank.Me == nil || rank.Me.Rank != 3 {
		t.Fatalf("C 的 Me.Rank 应为 3, got %+v", rank.Me)
	}
	// Me 不在当页仍正确：pageSize=2 时 B/A 占前二，A 的 Me 仍为 2
	rank2, _ := svc.GetCheckInRank(userA.ID, 1, 2)
	if rank2.Me == nil || rank2.Me.Rank != 2 {
		t.Fatalf("A 的 Me.Rank 应为 2（同 total 下较晚 last_date）, got %+v", rank2.Me)
	}
}

func TestStreakWindow400_CapsAndTotalFull(t *testing.T) {
	now := shDay(2026, 8, 20).Add(10 * time.Hour)
	svc, _ := newCheckInSvcAt(t, now)
	u := testutil.SeedStudent(t, svc.db, "窗口生", "x")

	// 连续 500 天（远超 400 天窗口）：streak 截断为 401（窗口含 cutoff 当天），total 保持全量 500
	start := now.AddDate(0, 0, -499)
	for i := 0; i < 500; i++ {
		d := startOfShanghaiDay(start.AddDate(0, 0, i))
		svc.db.Create(&model.ForumCheckIn{UserID: u.ID, CheckDate: d, CreatedAt: d})
	}
	streak, todayChecked := svc.streakInWindow(u.ID, now)
	if streak != 401 || !todayChecked {
		t.Fatalf("500 天连续 streakInWindow 应截断为 401 且 todayChecked=true, got %d/%v", streak, todayChecked)
	}
	// total 为全生命周期，不受窗口截断
	_, total, _ := svc.checkInStats(u.ID, now)
	if total != 500 {
		t.Fatalf("total 应为全量 500, got %d", total)
	}
	// 孤立远古记录不影响 streak：仅今日 + 401 天前一天（中间断开）→ streak=1
	svc2, _ := newCheckInSvcAt(t, now)
	u2 := testutil.SeedStudent(t, svc2.db, "孤立生", "x")
	svc2.db.Create(&model.ForumCheckIn{UserID: u2.ID, CheckDate: shDay(2026, 8, 20), CreatedAt: shDay(2026, 8, 20)})
	svc2.db.Create(&model.ForumCheckIn{UserID: u2.ID, CheckDate: startOfShanghaiDay(now.AddDate(0, 0, -401)), CreatedAt: now.AddDate(0, 0, -401)})
	streak2, _ := svc2.streakInWindow(u2.ID, now)
	if streak2 != 1 {
		t.Fatalf("远古孤立记录不应抬高 streak, got %d", streak2)
	}
}
