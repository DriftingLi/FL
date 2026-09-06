// Package service 回收对冲深方法 RollbackByRef 测试（#609）：多笔 SUM、单行特例、
// 原账为零、并发占坑冲突（整事务回滚）、封底 0，及存量 rollback 标记防双扣。
package service

import (
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// seedRollbackUser 独立命名建号（account/username 唯一索引），带余额。
func seedRollbackUser(t *testing.T, db *gorm.DB, name string, balance int) int {
	t.Helper()
	u := testutil.SeedStudent(t, db, name, "x")
	if err := db.Model(&model.HrwaiUser{}).Where("id = ?", u.ID).UpdateColumn("points_balance", balance).Error; err != nil {
		t.Fatalf("设置余额失败: %v", err)
	}
	return u.ID
}

// seedLedger 造一笔原账流水。
func seedLedger(t *testing.T, db *gorm.DB, userID, delta int, reason, refType, refID string) {
	t.Helper()
	if err := db.Create(&model.PointsLedger{UserID: userID, Delta: delta, Reason: reason, RefType: refType, RefID: refID, CreatedAt: time.Now()}).Error; err != nil {
		t.Fatalf("建流水失败: %v", err)
	}
}

func rollbackInTx(t *testing.T, db *gorm.DB, svc *PointsService, r PointsRollback) (int, error) {
	t.Helper()
	var clawed int
	err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		clawed, err = svc.RollbackByRef(tx, r)
		return err
	})
	return clawed, err
}

func idemCount(t *testing.T, db *gorm.DB, key string) int64 {
	t.Helper()
	var cnt int64
	if err := db.Model(&model.PointsEntryIdem{}).Where("idem_key = ?", key).Count(&cnt).Error; err != nil {
		t.Fatalf("查占坑行失败: %v", err)
	}
	return cnt
}

// TestRollbackByRefSumsMultipleReasons 多笔 SUM：投稿过审 +50 与达阶 +30 两笔合计追回 -80。
func TestRollbackByRefSumsMultipleReasons(t *testing.T) {
	svc, db := newPointsSvc(t)
	uid := seedRollbackUser(t, db, "rb_multi", 100)
	seedLedger(t, db, uid, 50, ReasonContributionApproved, RefTypeContribution, "7")
	seedLedger(t, db, uid, 30, ReasonContributionTier, RefTypeContribution, "7")

	clawed, err := rollbackInTx(t, db, svc, PointsRollback{
		RefType: RefTypeContribution, RefID: "7",
		Reasons: []string{ReasonContributionApproved, ReasonContributionTier},
		IdemKey: ContributionRollbackIdemKey(7),
	})
	if err != nil || clawed != 80 {
		t.Fatalf("多笔 SUM 应追回 80: clawed=%d err=%v", clawed, err)
	}
	if got := userBalance(t, db, uid); got != 20 {
		t.Fatalf("余额应为 100-80=20, got %d", got)
	}
	if got := ledgerCount(t, db, "user_id = ? AND reason = ? AND ref_type = ? AND ref_id = ?", uid, ReasonRollback, RefTypeContribution, "7"); got != 1 {
		t.Fatalf("回收流水应恰一行, got %d", got)
	}
	if got := idemCount(t, db, "contribution_rollback:7"); got != 1 {
		t.Fatalf("占坑行应存在且恰一条: got %d", got)
	}
}

// TestRollbackByRefSingleRowForumBonus 单行特例：论坛采纳奖励单行 SUM=该行 delta。
func TestRollbackByRefSingleRowForumBonus(t *testing.T) {
	svc, db := newPointsSvc(t)
	uid := seedRollbackUser(t, db, "rb_single", 200)
	seedLedger(t, db, uid, AcceptBonusPoints, ReasonAcceptedBonus, "forum_topic", "101")

	clawed, err := rollbackInTx(t, db, svc, PointsRollback{
		RefType: "forum_topic", RefID: "101",
		Reasons: []string{ReasonAcceptedBonus},
		IdemKey: ForumRollbackIdemKey(101),
	})
	if err != nil || clawed != AcceptBonusPoints {
		t.Fatalf("单行特例应追回 %d: clawed=%d err=%v", AcceptBonusPoints, clawed, err)
	}
	if got := userBalance(t, db, uid); got != 200-AcceptBonusPoints {
		t.Fatalf("余额应为 %d, got %d", 200-AcceptBonusPoints, got)
	}
	if got := idemCount(t, db, "rollback:101"); got != 1 {
		t.Fatalf("占坑行应存在: got %d", got)
	}
}

// TestRollbackByRefZeroOriginals 原账为零：无事可收直接跳过——不占坑、无流水、余额不动。
func TestRollbackByRefZeroOriginals(t *testing.T) {
	svc, db := newPointsSvc(t)
	uid := seedRollbackUser(t, db, "rb_zero", 50)

	// (a) 该 ref 无任何原账
	clawed, err := rollbackInTx(t, db, svc, PointsRollback{
		RefType: RefTypeContribution, RefID: "9",
		Reasons: []string{ReasonContributionApproved, ReasonContributionTier},
		IdemKey: ContributionRollbackIdemKey(9),
	})
	if err != nil || clawed != 0 {
		t.Fatalf("无原账应跳过: clawed=%d err=%v", clawed, err)
	}
	if got := idemCount(t, db, "contribution_rollback:9"); got != 0 {
		t.Fatalf("无事可收不得占坑: got %d", got)
	}

	// (b) 原账存在但全为非正向（负向消耗不参与聚合）
	seedLedger(t, db, uid, -10, "admin_penalty", RefTypeContribution, "10")
	clawed, err = rollbackInTx(t, db, svc, PointsRollback{
		RefType: RefTypeContribution, RefID: "10",
		Reasons: []string{ReasonContributionApproved, ReasonContributionTier},
		IdemKey: ContributionRollbackIdemKey(10),
	})
	if err != nil || clawed != 0 {
		t.Fatalf("仅负向流水应跳过: clawed=%d err=%v", clawed, err)
	}
	if got := idemCount(t, db, "contribution_rollback:10"); got != 0 {
		t.Fatalf("原账为零不得占坑: got %d", got)
	}
	if got := userBalance(t, db, uid); got != 50 {
		t.Fatalf("余额不得被改动: got %d", got)
	}
	if got := ledgerCount(t, db, "user_id = ? AND reason = ?", uid, ReasonRollback); got != 0 {
		t.Fatalf("跳过路径不得产生回收流水: got %d", got)
	}
}

// TestRollbackByRefIdemConflictRollsBackTx 并发占坑冲突：返回 ErrPointsProcessed，
// 调用方整笔回滚——事务内其他写入一并撤销，余额与原账不动。
func TestRollbackByRefIdemConflictRollsBackTx(t *testing.T) {
	svc, db := newPointsSvc(t)
	uid := seedRollbackUser(t, db, "rb_conflict", 100)
	seedLedger(t, db, uid, AcceptBonusPoints, ReasonAcceptedBonus, "forum_topic", "5")
	// 预置占坑行（模拟并发先胜者已回收）
	if err := db.Create(&model.PointsEntryIdem{IdemKey: ForumRollbackIdemKey(5)}).Error; err != nil {
		t.Fatalf("预置占坑失败: %v", err)
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		// 事务内先写一笔标记流水，验证占坑冲突时随整笔回滚撤销
		if err := tx.Create(&model.PointsLedger{UserID: uid, Delta: -1, Reason: "rb_marker", RefType: "test", RefID: "conflict"}).Error; err != nil {
			return err
		}
		_, err := svc.RollbackByRef(tx, PointsRollback{
			RefType: "forum_topic", RefID: "5",
			Reasons: []string{ReasonAcceptedBonus},
			IdemKey: ForumRollbackIdemKey(5),
		})
		if !errors.Is(err, ErrPointsProcessed) {
			t.Errorf("占坑冲突应返回 ErrPointsProcessed, got %v", err)
		}
		return err // 冲突 → 整笔事务回滚
	})
	if err == nil {
		t.Fatal("占坑冲突事务应回滚返回错误")
	}
	if got := ledgerCount(t, db, "user_id = ? AND reason = ?", uid, "rb_marker"); got != 0 {
		t.Fatalf("事务内标记流水应随整笔回滚撤销, got %d 行", got)
	}
	if got := ledgerCount(t, db, "user_id = ? AND reason = ?", uid, ReasonRollback); got != 0 {
		t.Fatalf("占坑冲突不得产生回收流水, got %d 行", got)
	}
	if got := userBalance(t, db, uid); got != 100 {
		t.Fatalf("余额不得被冲突事务改动, got %d", got)
	}
	if got := ledgerCount(t, db, "user_id = ? AND reason = ? AND delta = ?", uid, ReasonAcceptedBonus, AcceptBonusPoints); got != 1 {
		t.Fatalf("原账应原样保留, got %d 行", got)
	}
}

// TestRollbackByRefFloorsZero 封底 0：扣减按余额截断、余额钳 0、返回值仍为声明回收额；
// 余额为 0 的首次回收仅落占坑行、无流水。
func TestRollbackByRefFloorsZero(t *testing.T) {
	svc, db := newPointsSvc(t)
	uid := seedRollbackUser(t, db, "rb_floor", 30)
	seedLedger(t, db, uid, 60, ReasonContributionApproved, RefTypeContribution, "11")
	seedLedger(t, db, uid, 40, ReasonContributionTier, RefTypeContribution, "11")

	// 原账 100、余额仅 30：实扣截断为 -30，返回值仍为声明回收额 100（与迁移前 clawedBack 口径一致）
	clawed, err := rollbackInTx(t, db, svc, PointsRollback{
		RefType: RefTypeContribution, RefID: "11",
		Reasons: []string{ReasonContributionApproved, ReasonContributionTier},
		IdemKey: ContributionRollbackIdemKey(11),
	})
	if err != nil || clawed != 100 {
		t.Fatalf("返回值应为原账合计 100: clawed=%d err=%v", clawed, err)
	}
	if got := userBalance(t, db, uid); got != 0 {
		t.Fatalf("余额应钳 0, got %d", got)
	}
	if got := ledgerCount(t, db, "user_id = ? AND reason = ? AND ref_id = ? AND delta = ?", uid, ReasonRollback, "11", -30); got != 1 {
		t.Fatalf("回收流水应按余额截断为 -30, got %d 行", got)
	}

	// 余额 0 的首次回收：占坑落行、无流水，返回值仍为原账合计
	uid2 := seedRollbackUser(t, db, "rb_floor_zero", 0)
	seedLedger(t, db, uid2, AcceptBonusPoints, ReasonAcceptedBonus, "forum_topic", "77")
	clawed2, err := rollbackInTx(t, db, svc, PointsRollback{
		RefType: "forum_topic", RefID: "77",
		Reasons: []string{ReasonAcceptedBonus},
		IdemKey: ForumRollbackIdemKey(77),
	})
	if err != nil || clawed2 != AcceptBonusPoints {
		t.Fatalf("余额 0 回收应返回原账合计: clawed=%d err=%v", clawed2, err)
	}
	if got := ledgerCount(t, db, "user_id = ? AND reason = ?", uid2, ReasonRollback); got != 0 {
		t.Fatalf("余额 0 不得写回收流水, got %d 行", got)
	}
	if got := idemCount(t, db, "rollback:77"); got != 1 {
		t.Fatalf("占坑行应存在: got %d", got)
	}
	if got := userBalance(t, db, uid2); got != 0 {
		t.Fatalf("余额应保持 0, got %d", got)
	}
}

// TestRollbackByRefSkipsOnLegacyRollbackRow 存量标记：占坑表之前的 rollback 流水即「已回收」，
// 有标记不再对冲（防历史数据双扣），也不补占坑。
func TestRollbackByRefSkipsOnLegacyRollbackRow(t *testing.T) {
	svc, db := newPointsSvc(t)
	uid := seedRollbackUser(t, db, "rb_legacy", 100)
	seedLedger(t, db, uid, AcceptBonusPoints, ReasonAcceptedBonus, "forum_topic", "13")
	// 存量回收流水（无占坑行——占坑表上线前写入）
	seedLedger(t, db, uid, -AcceptBonusPoints, ReasonRollback, "forum_topic", "13")

	clawed, err := rollbackInTx(t, db, svc, PointsRollback{
		RefType: "forum_topic", RefID: "13",
		Reasons: []string{ReasonAcceptedBonus},
		IdemKey: ForumRollbackIdemKey(13),
	})
	if err != nil || clawed != 0 {
		t.Fatalf("存量标记应跳过: clawed=%d err=%v", clawed, err)
	}
	if got := userBalance(t, db, uid); got != 100 {
		t.Fatalf("余额不得被改动, got %d", got)
	}
	if got := ledgerCount(t, db, "user_id = ? AND reason = ?", uid, ReasonRollback); got != 1 {
		t.Fatalf("不得新增回收流水, got %d 行", got)
	}
	if got := idemCount(t, db, "rollback:13"); got != 0 {
		t.Fatalf("跳过路径不得补占坑, got %d", got)
	}
}
