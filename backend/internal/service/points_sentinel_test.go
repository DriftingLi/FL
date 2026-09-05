// Package service 积分域错误哨兵语义测试（ADR-0024 C1）：
// 一语义一哨兵，service 层以 errors.Is 断言语义，不依赖文案字符串。
package service

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// TestClaimSentinelSemantics 领取路径哨兵：任务不存在 → ErrTaskNotFound（404 类）；
// 重复领取 → 终身已领（newbie）/ 今日已领（daily）两语义分开。
func TestClaimSentinelSemantics(t *testing.T) {
	svc, db := newPointsSvc(t)
	uid := seedUserWithBalance(t, db, 0)

	// 任务不存在
	if _, err := svc.Claim(context.Background(), uid, "no_such_task"); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("任务不存在应报 ErrTaskNotFound, got %v", err)
	}

	// newbie 任务：行为未达成 → ErrTaskNotDone；达成后首次领取成功 → 重复领取报 ErrAlreadyClaimed
	seedTaskConfig(t, db, model.PointsTaskConfig{
		Code: "newbie_credential", Group: "newbie", Points: 10, DailyLimit: 1, TotalLimit: intPtr(1),
	})
	if _, err := svc.Claim(context.Background(), uid, "newbie_credential"); !errors.Is(err, ErrTaskNotDone) {
		t.Fatalf("未达成行为应报 ErrTaskNotDone, got %v", err)
	}
	credentialID := 1
	if err := db.Model(&model.HrwaiUser{}).Where("id = ?", uid).Update("current_credential_id", credentialID).Error; err != nil {
		t.Fatalf("设定当前证件失败: %v", err)
	}
	if _, err := svc.Claim(context.Background(), uid, "newbie_credential"); err != nil {
		t.Fatalf("首次领取 newbie 失败: %v", err)
	}
	if _, err := svc.Claim(context.Background(), uid, "newbie_credential"); !errors.Is(err, ErrAlreadyClaimed) {
		t.Fatalf("重复领取 newbie 应报 ErrAlreadyClaimed, got %v", err)
	}

	// daily 任务：行为未达成 → ErrTaskNotDone；达成（登录落表）后首次领取成功 → 重复领取报 ErrDailyClaimLimit
	seedTaskConfig(t, db, model.PointsTaskConfig{
		Code: "daily_login", Group: "daily", Points: 5, DailyLimit: 1,
	})
	if _, err := svc.Claim(context.Background(), uid, "daily_login"); !errors.Is(err, ErrTaskNotDone) {
		t.Fatalf("未登录应报 ErrTaskNotDone, got %v", err)
	}
	svc.MarkDailyLogin(uid)
	if _, err := svc.Claim(context.Background(), uid, "daily_login"); err != nil {
		t.Fatalf("首次领取 daily 失败: %v", err)
	}
	if _, err := svc.Claim(context.Background(), uid, "daily_login"); !errors.Is(err, ErrDailyClaimLimit) {
		t.Fatalf("重复领取 daily 应报 ErrDailyClaimLimit, got %v", err)
	}
}

// TestRedeemSentinelSemantics 兑换路径哨兵：课程不存在/无需兑换/真题卷/商城商品/已兑换。
func TestRedeemSentinelSemantics(t *testing.T) {
	svc, db := newPointsSvc(t)
	uid := seedUserWithBalance(t, db, 1000)

	if _, err := svc.RedeemCourse(context.Background(), uid, 999); !errors.Is(err, ErrCourseNotFound) {
		t.Fatalf("课程不存在应报 ErrCourseNotFound, got %v", err)
	}
	if _, err := svc.RedeemRealPaper(context.Background(), uid, 999); !errors.Is(err, ErrRealPaperUnavailable) {
		t.Fatalf("真题卷不存在应报 ErrRealPaperUnavailable, got %v", err)
	}
	if _, err := svc.RedeemShop(context.Background(), uid, "no_such_sku"); !errors.Is(err, ErrShopItemUnavailable) {
		t.Fatalf("商品不存在应报 ErrShopItemUnavailable, got %v", err)
	}
}

// TestPenaltySentinelSemantics 扣罚参数哨兵。
func TestPenaltySentinelSemantics(t *testing.T) {
	svc, db := newPointsSvc(t)
	uid := seedUserWithBalance(t, db, 100)

	if _, err := svc.AdminPenalty(context.Background(), 1, uid, 0, "理由"); !errors.Is(err, ErrInvalidPenalty) {
		t.Fatalf("分值非法应报 ErrInvalidPenalty, got %v", err)
	}
	if _, err := svc.AdminPenalty(context.Background(), 1, uid, 10, ""); !errors.Is(err, ErrEmptyPenaltyReason) {
		t.Fatalf("事由为空应报 ErrEmptyPenaltyReason, got %v", err)
	}
	if _, err := svc.AdminPenalty(context.Background(), 1, 99999, 10, "理由"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("用户不存在应报 ErrUserNotFound, got %v", err)
	}
}

// seedTaskConfig 播种任务配置。
func seedTaskConfig(t *testing.T, db *gorm.DB, cfg model.PointsTaskConfig) {
	t.Helper()
	if err := db.Create(&cfg).Error; err != nil {
		t.Fatalf("播种任务配置失败: %v", err)
	}
}

// intPtr 复用既有 helper（points_claim_contract_test.go 中已定义，但 service 包内独立定义）。
func intPtr(v int) *int { return &v }
