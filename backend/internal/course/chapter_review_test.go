// 本文件：章节先审后发状态机（#2498，spec C1–C8/C11）——转移全表、冻结快照 TOCTOU、
// 过审原子切换、驳回弃快照、单 pending 哨兵、学员可见谓词与已发布读面。
// seam：testutil.NewMemoryDB 内存 sqlite。
package course

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func seedReviewChapter(t *testing.T, db *gorm.DB) *model.Chapter {
	t.Helper()
	ch := model.Chapter{CourseID: 1, Title: "第一章", Content: "v1 正文", ContentType: "text",
		Description: "desc", Duration: 10, OrderNum: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("造章节失败: %v", err)
	}
	cf := model.ChapterFile{ChapterID: &ch.ChapterID, FileURL: "/static/uploads/chapter/a.pdf",
		FileName: "a.pdf", ContentType: "document", FileSize: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&cf).Error; err != nil {
		t.Fatalf("造 chapter_file 失败: %v", err)
	}
	return &ch
}

func TestChapterReviewStateMachineFullCycle(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	ch := seedReviewChapter(t, db)
	ctx := context.Background()

	// 新建默认 draft（C1）
	if ch.ReviewStatus != "" && ch.ReviewStatus != model.ChapterReviewDraft {
		t.Fatalf("新建章节状态 = %q, want draft", ch.ReviewStatus)
	}

	// 提交审核：冻结快照 + 状态 pending（C2/C4）
	if err := SubmitChapterForReview(ctx, db, ch.ChapterID, 7); err != nil {
		t.Fatalf("提交审核失败: %v", err)
	}
	snap, err := loadPublishedContent(db, ch.ChapterID) // published 槽还没有
	if err != nil || snap != nil {
		t.Fatalf("提交后不应有已发布槽: %v %v", snap, err)
	}
	var pending model.ChapterContentSlot
	if err := db.Where("slot = ? AND chapter_id = ?", model.ChapterReviewPending, ch.ChapterID).
		First(&pending).Error; err != nil {
		t.Fatalf("应有在审快照: %v", err)
	}
	if pending.Content != "v1 正文" {
		t.Fatalf("快照内容 = %q, want v1 正文", pending.Content)
	}
	files, err := loadPublishedFiles(db, ch.ChapterID)
	if err != nil || len(files) != 0 {
		t.Fatalf("published 清单应为空: %v %v", files, err)
	}
	var pendingFiles []model.ChapterFileSlot
	db.Where("slot = ? AND chapter_id = ?", model.ChapterReviewPending, ch.ChapterID).Find(&pendingFiles)
	if len(pendingFiles) != 1 || pendingFiles[0].FileName != "a.pdf" {
		t.Fatalf("快照清单 = %+v, want 1 条 a.pdf", pendingFiles)
	}

	// 单 pending（C8）：重复提交拒绝
	if err := SubmitChapterForReview(ctx, db, ch.ChapterID, 7); !errors.Is(err, ErrChapterReviewPending) {
		t.Fatalf("重复提交错误 = %v, want ErrChapterReviewPending", err)
	}

	// TOCTOU（C4）：审中改草稿不影响在审内容
	if err := db.Model(&model.Chapter{}).Where("chapter_id = ?", ch.ChapterID).
		Update("content", "v2 草稿").Error; err != nil {
		t.Fatalf("改草稿失败: %v", err)
	}
	db.Where("slot = ? AND chapter_id = ?", model.ChapterReviewPending, ch.ChapterID).First(&pending)
	if pending.Content != "v1 正文" {
		t.Fatalf("审中改草稿污染了快照: %q", pending.Content)
	}

	// 过审：原子切换到已发布槽，内容=快照（不是 v2 草稿）
	if err := ApproveChapterReview(ctx, db, ch.ChapterID); err != nil {
		t.Fatalf("过审失败: %v", err)
	}
	pub, err := loadPublishedContent(db, ch.ChapterID)
	if err != nil || pub == nil {
		t.Fatalf("应有已发布槽: %v %v", pub, err)
	}
	if pub.Content != "v1 正文" {
		t.Fatalf("过审切了错内容: %q", pub.Content)
	}
	pubFiles, err := loadPublishedFiles(db, ch.ChapterID)
	if err != nil || len(pubFiles) != 1 {
		t.Fatalf("已发布清单 = %+v, want 1 条", pubFiles)
	}
	var after model.Chapter
	db.First(&after, ch.ChapterID)
	if after.ReviewStatus != model.ChapterReviewPublished {
		t.Fatalf("过审后状态 = %q, want published", after.ReviewStatus)
	}
	// pending 槽已清（单 pending 释放）
	var n int64
	db.Model(&model.ChapterContentSlot{}).
		Where("slot = ? AND chapter_id = ?", model.ChapterReviewPending, ch.ChapterID).Count(&n)
	if n != 0 {
		t.Fatalf("过审后应无在审快照, got %d", n)
	}
}

func TestChapterReviewRejectDropsSnapshotAndAllowsResubmit(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	ch := seedReviewChapter(t, db)
	ctx := context.Background()

	if err := SubmitChapterForReview(ctx, db, ch.ChapterID, 7); err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	if err := RejectChapterReview(ctx, db, ch.ChapterID, "正文缺少安全须知"); err != nil {
		t.Fatalf("驳回失败: %v", err)
	}
	var after model.Chapter
	db.First(&after, ch.ChapterID)
	if after.ReviewStatus != model.ChapterReviewDraft || after.RejectReason != "正文缺少安全须知" {
		t.Fatalf("驳回后 = %q/%q, want draft+理由", after.ReviewStatus, after.RejectReason)
	}
	var n int64
	db.Model(&model.ChapterContentSlot{}).
		Where("slot = ? AND chapter_id = ?", model.ChapterReviewPending, ch.ChapterID).Count(&n)
	if n != 0 {
		t.Fatalf("驳回应弃快照, 残留 %d", n)
	}
	// 驳回后可改再提（C2），再提清理由
	if err := SubmitChapterForReview(ctx, db, ch.ChapterID, 7); err != nil {
		t.Fatalf("驳回后再提失败: %v", err)
	}
	db.First(&after, ch.ChapterID)
	if after.RejectReason != "" || after.ReviewStatus != model.ChapterReviewPending {
		t.Fatalf("再提后 = %q/%q, want pending+清理由", after.ReviewStatus, after.RejectReason)
	}
	// 非法边：无在审时过审/驳回
	other := seedReviewChapter(t, db)
	if err := ApproveChapterReview(ctx, db, other.ChapterID); !errors.Is(err, ErrChapterReviewNoPending) {
		t.Fatalf("无在审过审错误 = %v, want ErrChapterReviewNoPending", err)
	}
	if err := RejectChapterReview(ctx, db, other.ChapterID, "理由"); !errors.Is(err, ErrChapterReviewNoPending) {
		t.Fatalf("无在审驳回错误 = %v, want ErrChapterReviewNoPending", err)
	}
	// 驳回理由不能为空
	if err := SubmitChapterForReview(ctx, db, other.ChapterID, 7); err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	if err := RejectChapterReview(ctx, db, other.ChapterID, ""); !errors.Is(err, ErrInvalidReviewInput) {
		t.Fatalf("空理由错误 = %v, want ErrInvalidReviewInput", err)
	}
}

func TestChapterReviewVisibilityPredicateAndOverlay(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	ch := seedReviewChapter(t, db)
	ctx := context.Background()

	// C6/C11：无已发布槽 = 学员不可见
	has, err := chapterHasPublished(db, ch.ChapterID)
	if err != nil || has {
		t.Fatalf("新章不应有已发布槽: %v %v", has, err)
	}
	got := applyPublishedOverlay(db, []ChapterDTO{ChapterToDTO(ch)})
	if len(got) != 0 {
		t.Fatalf("无已发布槽章节应从学员列表消失: %+v", got)
	}

	// 提交中（pending）也无已发布槽——学员不可见
	if err := SubmitChapterForReview(ctx, db, ch.ChapterID, 7); err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	if got := applyPublishedOverlay(db, []ChapterDTO{ChapterToDTO(ch)}); len(got) != 0 {
		t.Fatalf("pending 章节应不可见: %+v", got)
	}

	// 过审后可见，内容/文件用已发布槽（C5：working 再改不影响学员读面）
	if err := ApproveChapterReview(ctx, db, ch.ChapterID); err != nil {
		t.Fatalf("过审失败: %v", err)
	}
	if err := db.Model(&model.Chapter{}).Where("chapter_id = ?", ch.ChapterID).
		Update("content", "v2 草稿").Error; err != nil {
		t.Fatalf("改草稿失败: %v", err)
	}
	got = applyPublishedOverlay(db, []ChapterDTO{ChapterToDTO(ch)})
	if len(got) != 1 || got[0].Content != "v1 正文" {
		t.Fatalf("学员读面应是已发布内容: %+v", got)
	}
}
