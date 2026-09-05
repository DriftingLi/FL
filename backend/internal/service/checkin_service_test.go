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

// newCheckInSvcAt 构造内存库 + 定格时钟服务（不带积分簿记，纯记录口径），返回 Fake 以便跨日推进。
func newCheckInSvcAt(t *testing.T, now time.Time) (*CheckInService, *clock.Fake) {
	t.Helper()
	f := clock.At(now)
	svc := NewCheckInService(testutil.NewMemoryDB(t), zap.NewNop(), f, nil)
	return svc, f
}

// newCheckInSvcWithPointsAt 构造带积分簿记的打卡服务（ADR-0028 直记发分口径）。
func newCheckInSvcWithPointsAt(t *testing.T, now time.Time) (*CheckInService, *clock.Fake, *PointsService) {
	t.Helper()
	f := clock.At(now)
	db := testutil.NewMemoryDB(t)
	points := NewPointsService(db, zap.NewNop(), f)
	svc := NewCheckInService(db, zap.NewNop(), f, points)
	return svc, f, points
}

func TestNewCheckInService_NilClockFallsBackToReal(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewCheckInService(db, zap.NewNop(), nil, nil)
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
	// 契约（ADR-0028）：逐日返回整月 31 天（未打卡日 checked=false）
	if len(cal.Days) != 31 {
		t.Fatalf("2026-08 日历应返整月 31 天, got %d", len(cal.Days))
	}
	byDate := map[string]CheckInDay{}
	for _, d := range cal.Days {
		byDate[d.Date] = d
	}
	if !byDate["2026-08-01"].Checked || !byDate["2026-08-15"].Checked {
		t.Fatalf("8-01/8-15 应已打卡: %+v", cal.Days)
	}
	if byDate["2026-08-02"].Checked {
		t.Fatalf("8-02 应未打卡: %+v", cal.Days)
	}
	// 无积分簿记（points=nil）时历史打卡 points 为 0
	if byDate["2026-08-01"].Points != 0 || byDate["2026-08-15"].Points != 0 {
		t.Fatalf("无流水历史打卡 points 应为 0: %+v", cal.Days)
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

func TestCheckInTierBonusFor(t *testing.T) {
	cases := []struct{ streak, want int }{
		{1, 0}, {2, 0}, {3, 5}, {4, 0}, {6, 0}, {7, 10}, {8, 0}, {29, 0}, {30, 50}, {31, 0},
	}
	for _, c := range cases {
		if got := CheckInTierBonusFor(c.streak); got != c.want {
			t.Fatalf("CheckInTierBonusFor(%d)=%d want %d", c.streak, got, c.want)
		}
	}
}

// TestCheckIn_AwardsBaseAndTier 打卡即发分（ADR-0028）：首签 +5；
// 连击满 3/7 天当日基础+阶梯合并一笔；同日重复/并发只发一次；断签后重新跨档再发。
func TestCheckIn_AwardsBaseAndTier(t *testing.T) {
	now := shDay(2026, 8, 3).Add(10 * time.Hour)
	svc, fake, points := newCheckInSvcWithPointsAt(t, now)
	u := testutil.SeedStudent(t, svc.db, "打卡积分生", "x")

	// day1 (8-03)：首签基础 5
	r, err := svc.CheckIn(u.ID)
	if err != nil {
		t.Fatalf("day1 checkin: %v", err)
	}
	if r.Points != 5 {
		t.Fatalf("day1 应发 5 分, got %d", r.Points)
	}
	// 同日重复：不再发分（幂等）
	r, _ = svc.CheckIn(u.ID)
	if r.Points != 0 {
		t.Fatalf("同日重复打卡不应再发分, got %d", r.Points)
	}
	// day2 (8-04)：连续第 2 天，基础 5
	fake.T = shDay(2026, 8, 4).Add(10 * time.Hour)
	r, _ = svc.CheckIn(u.ID)
	if r.Streak != 2 || r.Points != 5 {
		t.Fatalf("day2 应 streak=2 且发 5 分, got %+v", r)
	}
	// day3 (8-05)：连续满 3 → 基础 5 + 阶梯 5 = 10
	fake.T = shDay(2026, 8, 5).Add(10 * time.Hour)
	r, _ = svc.CheckIn(u.ID)
	if r.Streak != 3 || r.Points != 10 {
		t.Fatalf("day3 应 streak=3 且发 10 分, got %+v", r)
	}
	// day4-6 (8-06..08)：基础 5（8-03 首签、8-04/8-05 已在前面签到）
	for _, d := range []int{6, 7, 8} {
		fake.T = shDay(2026, 8, d).Add(10 * time.Hour)
		r, _ = svc.CheckIn(u.ID)
		if r.Points != 5 {
			t.Fatalf("8-%02d 应发 5 分, got %+v", d, r)
		}
	}
	// day7 (8-09)：连击满 7 → 5+10=15
	fake.T = shDay(2026, 8, 9).Add(10 * time.Hour)
	r, _ = svc.CheckIn(u.ID)
	if r.Streak != 7 || r.Points != 15 {
		t.Fatalf("day7 应 streak=7 且发 15 分, got %+v", r)
	}
	// 流水：7 笔正流水（7 天），累计 5+5+10+5+5+5+15 = 50
	bal, _ := points.GetBalance(u.ID)
	if bal.Balance != 50 {
		t.Fatalf("7 天累计应 50 分, got %d", bal.Balance)
	}
	var cnt int64
	if err := svc.db.Model(&model.PointsLedger{}).Where("user_id = ? AND reason = ?", u.ID, checkInReason).Count(&cnt).Error; err != nil {
		t.Fatalf("流水计数失败: %v", err)
	}
	if cnt != 7 {
		t.Fatalf("应恰 7 笔打卡流水, got %d", cnt)
	}

	// 断签：跳过 8-10，8-11 重签 streak=1 基础 5
	fake.T = shDay(2026, 8, 11).Add(10 * time.Hour)
	r, _ = svc.CheckIn(u.ID)
	if r.Streak != 1 || r.Points != 5 {
		t.Fatalf("断签重签应 streak=1 发 5 分, got %+v", r)
	}
	// 新连续段再跨 3 档（8-11,12,13）：8-13 发 10
	fake.T = shDay(2026, 8, 12).Add(10 * time.Hour)
	_, _ = svc.CheckIn(u.ID)
	fake.T = shDay(2026, 8, 13).Add(10 * time.Hour)
	r, _ = svc.CheckIn(u.ID)
	if r.Streak != 3 || r.Points != 10 {
		t.Fatalf("新连续段跨 3 档应发 10 分, got %+v", r)
	}
}

// TestCheckInCalendar_PointsFromLedger 日历逐日带实发积分：无流水的历史打卡 0，有流水按 delta。
func TestCheckInCalendar_PointsFromLedger(t *testing.T) {
	now := shDay(2026, 9, 10).Add(10 * time.Hour)
	svc, _, _ := newCheckInSvcWithPointsAt(t, now)
	u := testutil.SeedStudent(t, svc.db, "日历积分生", "x")

	// 历史打卡两天：8-01（未发分：无流水模拟旧数据）、9-01（发 10 分）
	for _, d := range []time.Time{shDay(2026, 9, 1)} {
		svc.db.Create(&model.ForumCheckIn{UserID: u.ID, CheckDate: d, CreatedAt: d})
	}
	svc.db.Create(&model.PointsLedger{
		UserID: u.ID, Delta: 10, Reason: checkInReason, RefType: checkInRefType,
		RefID: "2026-09-01", CreatedAt: shDay(2026, 9, 1),
	})
	svc.db.Create(&model.ForumCheckIn{UserID: u.ID, CheckDate: shDay(2026, 8, 1), CreatedAt: shDay(2026, 8, 1)})

	cal, err := svc.GetCheckInCalendar(u.ID, 2026, 9)
	if err != nil {
		t.Fatalf("日历查询失败: %v", err)
	}
	if len(cal.Days) != 30 {
		t.Fatalf("2026-09 应返整月 30 天, got %d", len(cal.Days))
	}
	sep1 := cal.Days[0]
	if !sep1.Checked || sep1.Points != 10 || sep1.Date != "2026-09-01" {
		t.Fatalf("9-01 应 checked 且 points=10, got %+v", sep1)
	}
	if cal.Days[1].Checked {
		t.Fatalf("9-02 应未打卡, got %+v", cal.Days[1])
	}
	cal2, _ := svc.GetCheckInCalendar(u.ID, 2026, 8)
	aug1 := cal2.Days[0]
	if len(cal2.Days) != 31 || !aug1.Checked || aug1.Points != 0 {
		t.Fatalf("无流水历史打卡 points 应 0（8-01 checked）: %+v", cal2.Days[:2])
	}
}
