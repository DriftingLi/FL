// Package service 积分簿记核心测试（ADR-0023）：占坑幂等、守卫扣减、
// 兑换并发双花、AI 扣费稳定键、管理员罚分封底 0。
package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func newPointsSvc(t *testing.T) (*PointsService, *gorm.DB) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	return NewPointsService(db, zap.NewNop(), nil), db
}

// seedUserWithBalance 插入带余额的测试用户，返回用户 ID。
func seedUserWithBalance(t *testing.T, db *gorm.DB, balance int) int {
	t.Helper()
	u := testutil.SeedStudent(t, db, "pts_user", "x")
	if err := db.Model(&model.HrwaiUser{}).Where("id = ?", u.ID).UpdateColumn("points_balance", balance).Error; err != nil {
		t.Fatalf("设置余额失败: %v", err)
	}
	return u.ID
}

func userBalance(t *testing.T, db *gorm.DB, userID int) int {
	t.Helper()
	var u model.HrwaiUser
	if err := db.First(&u, userID).Error; err != nil {
		t.Fatalf("加载用户失败: %v", err)
	}
	return u.PointsBalance
}

func ledgerCount(t *testing.T, db *gorm.DB, query string, args ...any) int64 {
	t.Helper()
	var cnt int64
	if err := db.Model(&model.PointsLedger{}).Where(query, args...).Count(&cnt).Error; err != nil {
		t.Fatalf("统计流水失败: %v", err)
	}
	return cnt
}

// TestApplyTxIdemOccupyRollback 占坑同键二次提交：第二次返回 ErrPointsProcessed，
// 且整笔事务回滚（同事务内其他写入一并撤销）。
func TestApplyTxIdemOccupyRollback(t *testing.T) {
	_, db := newPointsSvc(t)
	uid := seedUserWithBalance(t, db, 100)

	// 首次提交：占坑 + 流水 + 余额
	err := db.Transaction(func(tx *gorm.DB) error {
		_, err := ApplyTx(tx, PointsEntry{UserID: uid, Delta: 10, Reason: "daily_checkin", RefType: "task", RefID: "daily_checkin", IdemKey: "event:first"})
		return err
	})
	if err != nil {
		t.Fatalf("首次提交失败: %v", err)
	}
	if got := userBalance(t, db, uid); got != 110 {
		t.Fatalf("首次提交后余额应为 110, got %d", got)
	}

	// 同键二次提交：事务闭包里的其他写入（对冲流水）必须整笔回滚
	err = db.Transaction(func(tx *gorm.DB) error {
		marker := model.PointsLedger{UserID: uid, Delta: -1, Reason: "rollback", RefType: "test", RefID: "marker"}
		if err := tx.Create(&marker).Error; err != nil {
			return err
		}
		_, err := ApplyTx(tx, PointsEntry{UserID: uid, Delta: -5, Reason: "daily_checkin", RefType: "task", RefID: "daily_checkin", IdemKey: "event:first"})
		if !errors.Is(err, ErrPointsProcessed) {
			t.Errorf("同键二次提交应返回 ErrPointsProcessed, got %v", err)
		}
		return err // 冲突 → 整笔事务回滚
	})
	if err == nil {
		t.Fatal("二次提交事务应回滚返回错误")
	}
	if got := ledgerCount(t, db, "user_id = ? AND reason = ?", uid, "rollback"); got != 0 {
		t.Fatalf("事务内对冲流水应随整笔回滚撤销, got %d 行", got)
	}
	if got := userBalance(t, db, uid); got != 110 {
		t.Fatalf("余额不得被二次提交改动, got %d", got)
	}
}

// TestApplyTxRedeemGuard 守卫扣减：余额不足时 0 行受影响 → 报「积分不足」整笔回滚，
// 无流水、余额不动（并发双花窗口在守卫处收口，RedeemShop 缺陷的机制级断言）。
func TestApplyTxRedeemGuard(t *testing.T) {
	_, db := newPointsSvc(t)
	uid := seedUserWithBalance(t, db, 50)

	err := db.Transaction(func(tx *gorm.DB) error {
		_, err := ApplyTx(tx, PointsEntry{UserID: uid, Delta: -100, Reason: "redeem_x", RefType: "shop", RefID: "x"})
		return err
	})
	if err == nil || err.Error() != "积分不足" {
		t.Fatalf("超扣应报「积分不足」, got %v", err)
	}
	if got := userBalance(t, db, uid); got != 50 {
		t.Fatalf("余额不得被击穿, got %d", got)
	}
	if got := ledgerCount(t, db, "user_id = ?", uid); got != 0 {
		t.Fatalf("失败扣减不得留下流水, got %d 行", got)
	}
}

// TestApplyTxFloorZero 封底 0：扣减量按余额截断；余额为 0 时仅占坑行（无 0 分流水）。
func TestApplyTxFloorZero(t *testing.T) {
	_, db := newPointsSvc(t)
	uid := seedUserWithBalance(t, db, 30)

	// 扣 100 → 实扣 30，余额钳 0
	var applied bool
	err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		applied, err = ApplyTx(tx, PointsEntry{UserID: uid, Delta: -100, Reason: "rollback", RefType: "forum_topic", RefID: "1", FloorZero: true})
		return err
	})
	if err != nil || !applied {
		t.Fatalf("封底 0 扣减应成功: applied=%v err=%v", applied, err)
	}
	if got := userBalance(t, db, uid); got != 0 {
		t.Fatalf("余额应钳 0, got %d", got)
	}
	if got := ledgerCount(t, db, "user_id = ? AND delta = ?", uid, -30); got != 1 {
		t.Fatalf("流水应按余额截断为 -30, got %d 行", got)
	}

	// 余额 0 再扣：占坑行落库、无流水（Delta:0 违反 CHECK 的路径已消除）
	err = db.Transaction(func(tx *gorm.DB) error {
		_, err := ApplyTx(tx, PointsEntry{UserID: uid, Delta: -40, Reason: "rollback", RefType: "forum_topic", RefID: "2", IdemKey: "rollback:2", FloorZero: true})
		return err
	})
	if err != nil {
		t.Fatalf("余额 0 的 settle 应成功（仅占坑）: %v", err)
	}
	if got := ledgerCount(t, db, "user_id = ? AND ref_id = ?", uid, "2"); got != 0 {
		t.Fatalf("余额 0 不得写流水, got %d 行", got)
	}
	var idemCnt int64
	if err := db.Model(&model.PointsEntryIdem{}).Where("idem_key = ?", "rollback:2").Count(&idemCnt).Error; err != nil || idemCnt != 1 {
		t.Fatalf("占坑行应存在: cnt=%d err=%v", idemCnt, err)
	}
}

// TestRedeemShopConcurrentDoubleSpend 兑换并发双花守卫：余额恰好一次兑换时，
// 并发重复兑换恰有一笔成功，余额不击穿、流水与权益各一行。
func TestRedeemShopConcurrentDoubleSpend(t *testing.T) {
	db := testutil.NewFileDB(t) // :memory: 每连接独立库，并发场景须文件库
	svc := NewPointsService(db, zap.NewNop(), nil)
	uid := seedUserWithBalance(t, db, 300)
	if err := db.Create(&model.PointsShopItem{SKU: "unlock_gold", Title: "金牌", Price: 300, Enabled: true}).Error; err != nil {
		t.Fatalf("建商城项失败: %v", err)
	}

	const attempts = 4
	results := make(chan error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.RedeemShop(context.Background(), uid, "unlock_gold")
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("并发兑换应恰有一笔成功, got %d", success)
	}
	if got := userBalance(t, db, uid); got != 0 {
		t.Fatalf("余额应恰好扣完（不击穿为负）, got %d", got)
	}
	if got := ledgerCount(t, db, "user_id = ? AND ref_type = ?", uid, "shop"); got != 1 {
		t.Fatalf("兑换流水应恰一行, got %d", got)
	}
	var entCnt int64
	if err := db.Model(&model.UserEntitlement{}).Where("user_id = ? AND sku = ?", uid, "unlock_gold").Count(&entCnt).Error; err != nil || entCnt != 1 {
		t.Fatalf("权益应恰一条: cnt=%d err=%v", entCnt, err)
	}
}

// TestDeductAIStableRequestIdempotent AI 扣费稳定键：同一 requestID 重复扣费只扣一次。
func TestDeductAIStableRequestIdempotent(t *testing.T) {
	svc, db := newPointsSvc(t)
	uid := seedUserWithBalance(t, db, 100)

	points1, bal1, err := svc.DeductAI(context.Background(), uid, "req-stable-1", 1000, "")
	if err != nil {
		t.Fatalf("首次扣费失败: %v", err)
	}
	if bal1 != 100-points1 {
		t.Fatalf("首次扣费后余额不符: bal=%d", bal1)
	}
	points2, bal2, err := svc.DeductAI(context.Background(), uid, "req-stable-1", 1000, "")
	if err != nil {
		t.Fatalf("同键重复扣费不应报错: %v", err)
	}
	if points2 != points1 || bal2 != bal1 {
		t.Fatalf("同键重复扣费应幂等返回: p1=%d p2=%d bal1=%d bal2=%d", points1, points2, bal1, bal2)
	}
	if got := ledgerCount(t, db, "user_id = ? AND reason = ?", uid, "ai_tokens"); got != 1 {
		t.Fatalf("ai_tokens 流水应恰一行, got %d", got)
	}
	var idemCnt int64
	if err := db.Model(&model.PointsEntryIdem{}).Where("idem_key = ?", "ai_tokens:req-stable-1").Count(&idemCnt).Error; err != nil || idemCnt != 1 {
		t.Fatalf("ai_tokens 占坑行应存在: cnt=%d err=%v", idemCnt, err)
	}
}

// TestClaimThroughApplyTx 任务领取走簿记核心：占坑 + 流水 + 余额一次成型，重复领取拒绝。
func TestClaimThroughApplyTx(t *testing.T) {
	svc, db := newPointsSvc(t)
	uid := seedUserWithBalance(t, db, 0)
	if err := db.Create(&model.PointsTaskConfig{Code: "daily_checkin", Title: "每日打卡", Group: "daily", Points: 5, DailyLimit: 1, EventType: "check_in"}).Error; err != nil {
		t.Fatalf("建任务配置失败: %v", err)
	}
	ctx := context.Background()
	if _, err := svc.Claim(ctx, uid, "daily_checkin"); err != nil {
		t.Fatalf("首次领取失败: %v", err)
	}
	if _, err := svc.Claim(ctx, uid, "daily_checkin"); err == nil || err.Error() != "今日已领取" {
		t.Fatalf("重复领取应报「今日已领取」, got %v", err)
	}
	if got := userBalance(t, db, uid); got != 5 {
		t.Fatalf("领取后余额应为 5, got %d", got)
	}
	if got := ledgerCount(t, db, "user_id = ? AND reason = ?", uid, "daily_checkin"); got != 1 {
		t.Fatalf("领取流水应恰一行, got %d", got)
	}
}

// TestAdminPenaltyFloorsZero 管理员罚分：按余额截断、封底 0、不传幂等键（重复罚分合法）。
func TestAdminPenaltyFloorsZero(t *testing.T) {
	svc, db := newPointsSvc(t)
	uid := seedUserWithBalance(t, db, 30)

	deducted, err := svc.AdminPenalty(context.Background(), 1, uid, 100, "违规")
	if err != nil || deducted != 30 {
		t.Fatalf("罚分应按余额截断为 30: deducted=%d err=%v", deducted, err)
	}
	if got := userBalance(t, db, uid); got != 0 {
		t.Fatalf("罚分后余额应钳 0, got %d", got)
	}
	// 余额 0 再罚：不报错、无新流水
	deducted, err = svc.AdminPenalty(context.Background(), 1, uid, 50, "再罚")
	if err != nil || deducted != 0 {
		t.Fatalf("余额 0 罚分应返回 0: deducted=%d err=%v", deducted, err)
	}
	if got := ledgerCount(t, db, "user_id = ? AND reason = ?", uid, "admin_penalty"); got != 1 {
		t.Fatalf("admin_penalty 流水应恰一行, got %d", got)
	}
}
