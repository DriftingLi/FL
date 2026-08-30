// Package service #384 回归：删帖时答主余额为 0 的违规回收路径。
// 修复前 rollbackAcceptedBonusTx 在余额为 0 时写 Delta:0 流水，违反
// points_ledger CHECK (delta <> 0)，整笔删帖事务失败；修复后仅落占坑行
// （占坑行即「已处理」标记），删帖成功且回收幂等（ADR-0023）。
package service

import (
	"strconv"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestAdminDeleteTopicZeroBalanceRollback(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewForumService(db, nil, NewNotificationService(db, zap.NewNop()), NewForumCounter(), NewPointsService(db, zap.NewNop(), nil), zap.NewNop())

	answerer := testutil.SeedStudent(t, db, "zero_bal_answerer", "x")
	if err := db.Model(&model.HrwaiUser{}).Where("id = ?", answerer.ID).UpdateColumn("points_balance", 0).Error; err != nil {
		t.Fatalf("清零答主余额失败: %v", err)
	}
	asker := testutil.SeedStudent(t, db, "zero_bal_asker", "x")

	// 已采纳的问答帖 + 曾发放的采纳奖励流水
	acceptedReplyID := int64(9001)
	topic := model.ForumTopic{Category: ForumCategoryQuestion, UserID: asker.ID, Title: "违规回收帖", Content: "内容", AcceptedReplyID: &acceptedReplyID}
	if err := db.Create(&topic).Error; err != nil {
		t.Fatalf("建帖失败: %v", err)
	}
	if err := db.Create(&model.PointsLedger{UserID: answerer.ID, Delta: AcceptBonusPoints, Reason: ReasonAcceptedBonus, RefType: "forum_topic", RefID: "1"}).Error; err != nil {
		t.Fatalf("建采纳奖励流水失败: %v", err)
	}

	// 删帖（同事务内回收）：修复前此处因 Delta:0 违反 CHECK 整笔失败
	if err := svc.AdminDeleteTopic(topic.ID); err != nil {
		t.Fatalf("删帖应成功（余额 0 回收仅落占坑行）: %v", err)
	}
	var topicCnt int64
	if err := db.Model(&model.ForumTopic{}).Where("id = ?", topic.ID).Count(&topicCnt).Error; err != nil || topicCnt != 0 {
		t.Fatalf("帖子应已删除: cnt=%d err=%v", topicCnt, err)
	}
	var rollbackCnt int64
	if err := db.Model(&model.PointsLedger{}).Where("reason = ? AND ref_type = ?", ReasonRollback, "forum_topic").Count(&rollbackCnt).Error; err != nil || rollbackCnt != 0 {
		t.Fatalf("余额 0 不得写回收流水, got %d 行 err=%v", rollbackCnt, err)
	}
	var idemCnt int64
	if err := db.Model(&model.PointsEntryIdem{}).Where("idem_key = ?", "rollback:1").Count(&idemCnt).Error; err != nil || idemCnt != 1 {
		t.Fatalf("占坑行应存在且恰一条: cnt=%d err=%v", idemCnt, err)
	}
	var bal int
	if err := db.Model(&model.HrwaiUser{}).Where("id = ?", answerer.ID).Select("points_balance").Scan(&bal).Error; err != nil || bal != 0 {
		t.Fatalf("答主余额应保持 0: bal=%d err=%v", bal, err)
	}

	// 回收幂等：同键二次 settle 静默跳过，不再产生流水
	for i := 0; i < 2; i++ {
		if err := db.Transaction(func(tx *gorm.DB) error {
			return svc.rollbackAcceptedBonusTx(tx, topic.ID)
		}); err != nil {
			t.Fatalf("第 %d 次重复回收应幂等跳过: %v", i+1, err)
		}
	}
	if err := db.Model(&model.PointsLedger{}).Where("reason = ? AND ref_type = ?", ReasonRollback, "forum_topic").Count(&rollbackCnt).Error; err != nil || rollbackCnt != 0 {
		t.Fatalf("重复回收不得产生流水, got %d 行 err=%v", rollbackCnt, err)
	}
}

// TestAcceptReplyRewardIdempotentOccupy 采纳奖励「每帖只发一次」：状态 CAS + 占坑双保险。
// 同帖重复发放（占坑冲突）静默跳过且不影响状态迁移。
func TestAcceptReplyRewardIdempotentOccupy(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewForumService(db, nil, NewNotificationService(db, zap.NewNop()), NewForumCounter(), NewPointsService(db, zap.NewNop(), nil), zap.NewNop())

	answerer := testutil.SeedStudent(t, db, "occ_answerer", "x")
	asker := testutil.SeedStudent(t, db, "occ_asker", "x")

	topic := model.ForumTopic{Category: ForumCategoryQuestion, UserID: asker.ID, Title: "占坑帖", Content: "内容"}
	if err := db.Create(&topic).Error; err != nil {
		t.Fatalf("建帖失败: %v", err)
	}
	// 预置占坑行（模拟并发先胜者已发放）
	idemKey := "accepted_bonus:" + strconv.FormatInt(topic.ID, 10)
	if err := db.Create(&model.PointsEntryIdem{IdemKey: idemKey}).Error; err != nil {
		t.Fatalf("预置占坑失败: %v", err)
	}

	// 直接走 settle 通道断言占坑语义：奖励静默跳过、无流水
	if err := db.Transaction(func(tx *gorm.DB) error {
		return svc.points.SettleRewardTx(tx, PointsEntry{
			UserID: answerer.ID, Delta: AcceptBonusPoints, Reason: ReasonAcceptedBonus,
			RefType: "forum_topic", RefID: strconv.FormatInt(topic.ID, 10),
			IdemKey: idemKey,
		})
	}); err != nil {
		t.Fatalf("占坑冲突应静默跳过: %v", err)
	}
	var ledgerCnt int64
	if err := db.Model(&model.PointsLedger{}).Where("user_id = ?", answerer.ID).Count(&ledgerCnt).Error; err != nil || ledgerCnt != 0 {
		t.Fatalf("占坑冲突不得重复发分, got %d 行 err=%v", ledgerCnt, err)
	}
}
