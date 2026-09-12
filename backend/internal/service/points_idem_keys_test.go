// Package service 实现业务服务层。
// 本文件：积分幂等键构造器快照测试（#608）。逐字钉住全部键格式——键即
// points_entry_idem 占坑主键，格式漂移会让历史键失配（双发/双扣），任何变更
// 都必须是有意的口径决策并同步 CONTEXT.md 登记（ADR-0023）。
package service

import (
	"testing"
	"time"

	"forklift-training/internal/clock"
)

func TestPointsIdemKeyFormats(t *testing.T) {
	day := time.Date(2026, 9, 5, 10, 30, 0, 0, clock.Location())
	cases := []struct {
		name string
		got  string
		want string
	}{
		// ===== 直记键 =====
		{"redeem", RedeemIdemKey("course:42"), "redeem:course:42"},
		{"ai_tokens", AITokensIdemKey("req-abc-123"), "ai_tokens:req-abc-123"},
		{"accepted_bonus", AcceptedBonusIdemKey(101), "accepted_bonus:101"},
		{"accept_action", AcceptActionIdemKey(101), "accept_action:101"},
		{"contribution_approved", ContributionApprovedIdemKey(7), "contribution_approved:7"},
		{"contribution_tier", ContributionTierIdemKey(7, 50), "contribution_tier:7:50"},
		{"checkin", CheckInIdemKey(3, day), "checkin:3:2026-09-05"},
		// ===== 回收键 =====
		{"rollback", ForumRollbackIdemKey(101), "rollback:101"},
		{"contribution_rollback", ContributionRollbackIdemKey(7), "contribution_rollback:7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("idem key %s = %q, want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}

func TestCheckInIdemKeyUsesShanghaiDay(t *testing.T) {
	// UTC 2026-09-05 17:00 = 上海 2026-09-06 01:00：键内日期必须按业务时区归一，
	// 与打卡流水 ref_id（shanghaiDayStr）同口径。
	utcEvening := time.Date(2026, 9, 5, 17, 0, 0, 0, time.UTC)
	if got, want := CheckInIdemKey(3, utcEvening), "checkin:3:2026-09-06"; got != want {
		t.Fatalf("CheckInIdemKey crossing UTC day = %q, want %q", got, want)
	}
}
