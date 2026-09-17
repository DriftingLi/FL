// #1098 行为用例：扣罚即通知回归积分域后属**强一致族**（ADR-0056 §4）——
// 站内信写失败则扣罚整体不生效（流水不存在、余额不变），管理端可见 ErrPenaltyNotifyFailed。
//
// 发信失败注入方式：删掉 notifications 表——发信在真实链路（同事务第二笔写）上失败，
// 不引入测试专用 seam，也不依赖 mock。
package service

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// TestAdminPenaltyNotifyFailureRollsBack 通知写失败 → 流水不存在、余额不变。
func TestAdminPenaltyNotifyFailureRollsBack(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	uid := seedUserWithBalance(t, db, 100)
	if err := db.Migrator().DropTable(&model.Notification{}); err != nil {
		t.Fatalf("注入发信失败（删 notifications 表）: %v", err)
	}
	svc := NewPointsService(db, zap.NewNop(), nil, NewNotificationService(db, zap.NewNop()))

	deducted, err := svc.AdminPenalty(context.Background(), 1, uid, 30, "违规")
	if !errors.Is(err, ErrPenaltyNotifyFailed) {
		t.Fatalf("发信失败应报 ErrPenaltyNotifyFailed, got %v", err)
	}
	if deducted != 0 {
		t.Fatalf("扣罚整体不生效：deducted = %d, 期望 0", deducted)
	}
	if got := ledgerCount(t, db, "user_id = ? AND reason = ?", uid, "admin_penalty"); got != 0 {
		t.Fatalf("通知写失败时扣罚流水应不存在, got %d 行", got)
	}
	if got := userBalance(t, db, uid); got != 100 {
		t.Fatalf("通知写失败时余额应不变（100）, got %d", got)
	}
}

// TestAdminPenaltyNotifyCommittedWithLedger 扣罚成功：流水与站内信同事务落库。
func TestAdminPenaltyNotifyCommittedWithLedger(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	uid := seedUserWithBalance(t, db, 100)
	svc := NewPointsService(db, zap.NewNop(), nil, NewNotificationService(db, zap.NewNop()))

	deducted, err := svc.AdminPenalty(context.Background(), 1, uid, 30, "违规操作")
	if err != nil || deducted != 30 {
		t.Fatalf("扣罚应成功扣 30: deducted=%d err=%v", deducted, err)
	}
	if got := userBalance(t, db, uid); got != 70 {
		t.Fatalf("余额应为 70, got %d", got)
	}
	if got := ledgerCount(t, db, "user_id = ? AND reason = ?", uid, "admin_penalty"); got != 1 {
		t.Fatalf("扣罚流水应恰一行, got %d", got)
	}
	var n model.Notification
	if err := db.Where("user_id = ?", uid).First(&n).Error; err != nil {
		t.Fatalf("同事务站内信应存在: %v", err)
	}
}

// TestAdminPenaltyZeroBalanceStillNotifies 余额不足（截断到 0）：不写流水但仍告知学员——
// 保留 #1098 之前 api 层「成功即发信」的既有语义（扣 0 分也发）。
func TestAdminPenaltyZeroBalanceStillNotifies(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	uid := seedUserWithBalance(t, db, 0)
	svc := NewPointsService(db, zap.NewNop(), nil, NewNotificationService(db, zap.NewNop()))

	deducted, err := svc.AdminPenalty(context.Background(), 1, uid, 20, "违规")
	if err != nil || deducted != 0 {
		t.Fatalf("余额 0 罚分应返回 0 且不报错: deducted=%d err=%v", deducted, err)
	}
	if got := ledgerCount(t, db, "user_id = ? AND reason = ?", uid, "admin_penalty"); got != 0 {
		t.Fatalf("截断到 0 不应写流水, got %d 行", got)
	}
	var n model.Notification
	if err := db.Where("user_id = ?", uid).First(&n).Error; err != nil {
		t.Fatalf("截断到 0 仍应发通知（既有语义）: %v", err)
	}
}
