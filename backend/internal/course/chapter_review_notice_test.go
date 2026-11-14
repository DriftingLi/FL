// 本文件：章节审核通知的强一致语义与章节归属落定（#2537 / ADR-0095）——
//   - 通知写失败必须连坐审核事务（不留「已过审但讲师不知道」的静默洞）；
//   - 章节归属首次落定后不被改写（作者 ≠ 最后编辑者）；
//   - 存量章节（作者未知）不投递，但审核照常成功。
package course

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/notification"
	"forklift-training/internal/testutil"
)

// seedChapterForReview 造「课程 + 章节」（可带归属讲师），不提交审核。
// 与 seedQueueChapter 分开：本文件的用例要控制提交人与归属，不走它的固定 actor=7。
func seedChapterForReview(t *testing.T, db *gorm.DB, title string, tutorID *int) *model.Chapter {
	t.Helper()
	course := model.Course{Name: "通知课程", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("造课程失败: %v", err)
	}
	ch := model.Chapter{CourseID: course.CourseID, Title: title, Content: "原始正文", ContentType: "text",
		OrderNum: 1, TutorID: tutorID, CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("造章节失败: %v", err)
	}
	return &ch
}

// TestNotifyFailureRollsBackReview 通知写失败 ⇒ 过审回滚（强一致族，ADR-0095 决策五）。
// 注入方式沿 points/admin_penalty_consistency_test.go 先例：删掉 notifications 表让写入必失败。
func TestNotifyFailureRollsBackReview(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewAdminService(db, nil, notification.NewService(db, zap.NewNop()), zap.NewNop())

	chapter := seedChapterForReview(t, db, "强一致章", nil)
	if err := SubmitChapterForReview(context.Background(), db, chapter.ChapterID, 31); err != nil {
		t.Fatalf("提交审核失败: %v", err)
	}

	// 删掉通知表：投递必然失败。
	if err := db.Migrator().DropTable(&model.Notification{}); err != nil {
		t.Fatalf("删通知表失败: %v", err)
	}

	err := svc.approveOneWithNotice(context.Background(), chapter.ChapterID)
	if err == nil {
		t.Fatal("通知写失败时过审应整体失败（强一致族）")
	}

	// 章节仍在 pending、已发布槽未生成——审核没有被部分提交。
	var after model.Chapter
	if err := db.First(&after, chapter.ChapterID).Error; err != nil {
		t.Fatalf("查章节失败: %v", err)
	}
	if after.ReviewStatus != model.ChapterReviewPending {
		t.Fatalf("通知失败后章节状态应仍为 pending（回滚）, got %q", after.ReviewStatus)
	}
	var published int64
	if err := db.Model(&model.ChapterContentSlot{}).
		Where("chapter_id = ? AND slot = ?", chapter.ChapterID, model.ChapterReviewPublished).
		Count(&published).Error; err != nil {
		t.Fatalf("查已发布槽失败: %v", err)
	}
	if published != 0 {
		t.Fatalf("通知失败后不应留下已发布槽（回滚）, got %d 行", published)
	}
	var pending int64
	if err := db.Model(&model.ChapterContentSlot{}).
		Where("chapter_id = ? AND slot = ?", chapter.ChapterID, model.ChapterReviewPending).
		Count(&pending).Error; err != nil {
		t.Fatalf("查在审槽失败: %v", err)
	}
	if pending != 1 {
		t.Fatalf("在审快照应完好保留, got %d 行", pending)
	}
}

// TestClaimChapterOwnershipFirstWriteWins 章节归属：首次落定后不被改写（ADR-0095 决策二）。
func TestClaimChapterOwnershipFirstWriteWins(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	chapter := seedChapterForReview(t, db, "归属章", nil)

	// 讲师 A 先落定
	if err := ClaimChapterOwnership(db, chapter.ChapterID, 41); err != nil {
		t.Fatalf("首次落定失败: %v", err)
	}
	// 讲师 B 再写：不该抢走归属
	if err := ClaimChapterOwnership(db, chapter.ChapterID, 42); err != nil {
		t.Fatalf("二次落定不应报错（幂等跳过）: %v", err)
	}
	var after model.Chapter
	if err := db.First(&after, chapter.ChapterID).Error; err != nil {
		t.Fatalf("查章节失败: %v", err)
	}
	if after.TutorID == nil || *after.TutorID != 41 {
		t.Fatalf("归属应保持首次落定的讲师 41（作者≠最后编辑者）, got %v", after.TutorID)
	}
}

// TestSubmitChapterClaimsOwnership SubmitChapterForReview 也落定归属（讲师侧写路径之一）。
func TestSubmitChapterClaimsOwnership(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	chapter := seedChapterForReview(t, db, "提交落归属章", nil)

	if err := SubmitChapterForReview(context.Background(), db, chapter.ChapterID, 51); err != nil {
		t.Fatalf("提交审核失败: %v", err)
	}
	var after model.Chapter
	if err := db.First(&after, chapter.ChapterID).Error; err != nil {
		t.Fatalf("查章节失败: %v", err)
	}
	if after.TutorID == nil || *after.TutorID != 51 {
		t.Fatalf("提交审核应落定归属为 51, got %v", after.TutorID)
	}

	// 另一个讲师再提交（先驳回腾出 pending 位）——归属不该被改写
	if err := RejectChapterReview(context.Background(), db, chapter.ChapterID, "改一版"); err != nil {
		t.Fatalf("驳回失败: %v", err)
	}
	if err := SubmitChapterForReview(context.Background(), db, chapter.ChapterID, 52); err != nil {
		t.Fatalf("二次提交失败: %v", err)
	}
	after = model.Chapter{}
	if err := db.First(&after, chapter.ChapterID).Error; err != nil {
		t.Fatalf("查章节失败: %v", err)
	}
	if after.TutorID == nil || *after.TutorID != 51 {
		t.Fatalf("二次提交不应改写归属（仍应是 51）, got %v", after.TutorID)
	}
}

// TestReviewSkipsNotificationWhenAuthorUnknown 存量章节（作者未知）审核照常成功但不投递
// （ADR-0095 决策九：如实的「不知道发给谁」，不是投给错的人）。
func TestReviewSkipsNotificationWhenAuthorUnknown(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewAdminService(db, nil, notification.NewService(db, zap.NewNop()), zap.NewNop())

	// 提交人 0 = 匿名（内存库直建 pending 槽模拟存量形状），章节无归属
	chapter := seedChapterForReview(t, db, "无主章", nil)
	if err := SubmitChapterForReview(context.Background(), db, chapter.ChapterID, 0); err != nil {
		t.Fatalf("提交审核失败: %v", err)
	}
	// 清掉归属与提交人：模拟「作者未知」的存量章节
	if err := db.Model(&model.Chapter{}).Where("chapter_id = ?", chapter.ChapterID).
		Update("tutor_id", nil).Error; err != nil {
		t.Fatalf("清归属失败: %v", err)
	}
	if err := db.Model(&model.ChapterContentSlot{}).
		Where("chapter_id = ? AND slot = ?", chapter.ChapterID, model.ChapterReviewPending).
		Update("submitted_by", 0).Error; err != nil {
		t.Fatalf("清提交人失败: %v", err)
	}

	if err := svc.approveOneWithNotice(context.Background(), chapter.ChapterID); err != nil {
		t.Fatalf("作者未知时审核应照常成功: %v", err)
	}
	var after model.Chapter
	if err := db.First(&after, chapter.ChapterID).Error; err != nil {
		t.Fatalf("查章节失败: %v", err)
	}
	if after.ReviewStatus != model.ChapterReviewPublished {
		t.Fatalf("审核应成功（状态 published）, got %q", after.ReviewStatus)
	}
	var notices int64
	if err := db.Model(&model.Notification{}).Where("type = ?", notification.NotifTypeChapterReview).
		Count(&notices).Error; err != nil {
		t.Fatalf("查通知失败: %v", err)
	}
	if notices != 0 {
		t.Fatalf("作者未知时不应投递（不知道发给谁）, got %d 条", notices)
	}
}

// TestAdminEditNotifiesAuthorInSameTx C7 管理员直接生效：通知与发布同一事务。
// 用「删通知表 → 改章节」验证回滚：章节正文不该被改掉。
func TestAdminEditNotifiesAuthorInSameTx(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewAdminService(db, nil, notification.NewService(db, zap.NewNop()), zap.NewNop())

	authorID := 61
	chapter := seedChapterForReview(t, db, "被改章", &authorID)
	originalContent := chapter.Content

	if err := db.Migrator().DropTable(&model.Notification{}); err != nil {
		t.Fatalf("删通知表失败: %v", err)
	}

	newContent := "管理员改后的正文"
	if _, err := svc.UpdateChapter(context.Background(), chapter.ChapterID, &ChapterInput{Content: &newContent}); err == nil {
		t.Fatal("通知写失败时管理员改写应整体失败（强一致族）")
	}

	// 正文未被改掉、状态未变 published——发布与通知同事务，一起回滚。
	var after model.Chapter
	if err := db.First(&after, chapter.ChapterID).Error; err != nil {
		t.Fatalf("查章节失败: %v", err)
	}
	if after.Content != originalContent {
		t.Fatalf("通知失败后正文应回滚到原值 %q, got %q", originalContent, after.Content)
	}
	if after.ReviewStatus == model.ChapterReviewPublished {
		t.Fatal("通知失败后不应落成 published（发布已回滚）")
	}
}
