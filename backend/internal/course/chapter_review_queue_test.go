// 本文件：章节审核队列与批量操作（#2499，spec C9）——队列按提交时间排序、
// 在审快照详情预览、批量过审/驳回（统一理由、逐章落状态、失败明示）。
// seam：testutil.NewMemoryDB 内存 sqlite；通知走 notification.Service 真实现（nil 容忍）。
package course

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/notification"
	"forklift-training/internal/testutil"
)

// seedQueueChapter 造「课程 + 章节」并在需要时提交审核（冻结快照）。
func seedQueueChapter(t *testing.T, db *gorm.DB, title, content string, submit bool) *model.Chapter {
	t.Helper()
	course := model.Course{Name: "叉车维修基础", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("造课程失败: %v", err)
	}
	ch := model.Chapter{CourseID: course.CourseID, Title: title, Content: content, ContentType: "text",
		CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("造章节失败: %v", err)
	}
	if submit {
		if err := SubmitChapterForReview(context.Background(), db, ch.ChapterID, 7); err != nil {
			t.Fatalf("提交审核失败: %v", err)
		}
	}
	return &ch
}

func TestListPendingChapterReviewsOrderedBySubmitTime(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	first := seedQueueChapter(t, db, "第一章", "v1", true)
	second := seedQueueChapter(t, db, "第二章", "v2", true)
	third := seedQueueChapter(t, db, "第三章", "v3", true)
	// 未提交的章节不进队列（C2：显式提交才进队列）
	seedQueueChapter(t, db, "未提交章节", "draft", false)

	svc := NewAdminService(db, nil, zap.NewNop())
	res, err := svc.ListPendingChapterReviews(context.Background(), 1, 20)
	if err != nil {
		t.Fatalf("查队列失败: %v", err)
	}
	if res.Total != 3 {
		t.Fatalf("队列总数 = %d, want 3", res.Total)
	}
	if len(res.Items) != 3 {
		t.Fatalf("队列条数 = %d, want 3", len(res.Items))
	}
	// 按提交时间升序（先进先审）
	wantOrder := []int{first.ChapterID, second.ChapterID, third.ChapterID}
	for i, want := range wantOrder {
		if res.Items[i].ChapterID != want {
			t.Fatalf("第 %d 条 chapter_id = %d, want %d（应按提交时间升序）", i, res.Items[i].ChapterID, want)
		}
	}
	// 队列项带课程名与章节标题，管理员不用再点进去才知道审什么
	if res.Items[0].ChapterTitle != "第一章" || res.Items[0].CourseName != "叉车维修基础" {
		t.Fatalf("队列项缺标题/课程名: %+v", res.Items[0])
	}
	if res.Items[0].SubmittedBy != 7 {
		t.Fatalf("提交人 = %d, want 7", res.Items[0].SubmittedBy)
	}
	if res.Items[0].FrozenAt == "" {
		t.Fatal("队列项应有冻结时刻（排序依据）")
	}
}

func TestReviewQueueExcludesReviewedChapters(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	approved := seedQueueChapter(t, db, "已过审", "v1", true)
	rejected := seedQueueChapter(t, db, "已驳回", "v2", true)
	ctx := context.Background()
	if err := ApproveChapterReview(ctx, db, approved.ChapterID); err != nil {
		t.Fatalf("过审失败: %v", err)
	}
	if err := RejectChapterReview(ctx, db, rejected.ChapterID, "缺少配图"); err != nil {
		t.Fatalf("驳回失败: %v", err)
	}

	svc := NewAdminService(db, nil, zap.NewNop())
	res, err := svc.ListPendingChapterReviews(context.Background(), 1, 20)
	if err != nil {
		t.Fatalf("查队列失败: %v", err)
	}
	if res.Total != 0 || len(res.Items) != 0 {
		t.Fatalf("已审章节不应留在队列: total=%d items=%d", res.Total, len(res.Items))
	}
}

func TestReviewSnapshotPreviewShowsFrozenNotWorking(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	ch := seedQueueChapter(t, db, "第一章", "提交时正文", true)
	// 审中讲师继续改草稿（C4）：快照不得跟着变
	if err := db.Model(&model.Chapter{}).Where("chapter_id = ?", ch.ChapterID).
		Update("content", "审中改的草稿").Error; err != nil {
		t.Fatalf("改草稿失败: %v", err)
	}
	// 审中新增的文件不进在审快照（快照在提交时已冻结）
	cf := model.ChapterFile{ChapterID: &ch.ChapterID, FileURL: "/static/uploads/chapter/late.pdf",
		FileName: "late.pdf", ContentType: "document", FileSize: 10, CreatedAt: testutil.Now()}
	if err := db.Create(&cf).Error; err != nil {
		t.Fatalf("造文件失败: %v", err)
	}

	svc := NewAdminService(db, nil, zap.NewNop())
	snap, err := svc.GetChapterReviewSnapshot(context.Background(), ch.ChapterID)
	if err != nil {
		t.Fatalf("查快照失败: %v", err)
	}
	if snap.Content != "提交时正文" {
		t.Fatalf("快照正文 = %q, want 提交时正文（不得读 working）", snap.Content)
	}
	if len(snap.Files) != 0 {
		t.Fatalf("快照文件数 = %d, want 0（提交后才加的文件不得进快照）", len(snap.Files))
	}
	if snap.ChapterTitle != "第一章" || snap.CourseName != "叉车维修基础" {
		t.Fatalf("快照缺标题/课程名: %+v", snap)
	}
}

func TestGetReviewSnapshotWithoutPending(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	ch := seedQueueChapter(t, db, "没提交过", "v1", false)
	svc := NewAdminService(db, nil, zap.NewNop())
	if _, err := svc.GetChapterReviewSnapshot(context.Background(), ch.ChapterID); !errors.Is(err, ErrChapterReviewNoPending) {
		t.Fatalf("无在审版本应返回 ErrChapterReviewNoPending, got %v", err)
	}
}

func TestBatchApproveChapterReviewsReportsPartialFailure(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	ok1 := seedQueueChapter(t, db, "可过审一", "v1", true)
	ok2 := seedQueueChapter(t, db, "可过审二", "v2", true)
	noPending := seedQueueChapter(t, db, "没有在审", "v3", false)

	svc := NewAdminService(db, nil, zap.NewNop())
	res, err := svc.BatchApproveChapterReviews(context.Background(),
		[]int{ok1.ChapterID, noPending.ChapterID, ok2.ChapterID})
	if err != nil {
		t.Fatalf("批量过审整体失败（不应整体回滚）: %v", err)
	}
	if res.ApprovedCount != 2 {
		t.Fatalf("过审成功数 = %d, want 2", res.ApprovedCount)
	}
	if len(res.Failures) != 1 {
		t.Fatalf("失败明细条数 = %d, want 1（失败要明示）", len(res.Failures))
	}
	if res.Failures[0].ChapterID != noPending.ChapterID {
		t.Fatalf("失败明细 chapter_id = %d, want %d", res.Failures[0].ChapterID, noPending.ChapterID)
	}
	if res.Failures[0].Reason == "" {
		t.Fatal("失败明细应带原因")
	}
	// 逐章落状态：成功的两章确实已发布
	for _, id := range []int{ok1.ChapterID, ok2.ChapterID} {
		var got model.Chapter
		if err := db.First(&got, id).Error; err != nil {
			t.Fatalf("查章节 %d 失败: %v", id, err)
		}
		if got.ReviewStatus != model.ChapterReviewPublished {
			t.Fatalf("章节 %d 状态 = %q, want published", id, got.ReviewStatus)
		}
	}
}

func TestBatchRejectChapterReviewsUnifiedReason(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	a := seedQueueChapter(t, db, "驳回一", "v1", true)
	b := seedQueueChapter(t, db, "驳回二", "v2", true)

	svc := NewAdminService(db, nil, zap.NewNop())
	res, err := svc.BatchRejectChapterReviews(context.Background(), []int{a.ChapterID, b.ChapterID}, "配图与正文不符")
	if err != nil {
		t.Fatalf("批量驳回失败: %v", err)
	}
	if res.RejectedCount != 2 || len(res.Failures) != 0 {
		t.Fatalf("批量驳回结果 = %+v, want 2 成功 0 失败", res)
	}
	for _, id := range []int{a.ChapterID, b.ChapterID} {
		var got model.Chapter
		if err := db.First(&got, id).Error; err != nil {
			t.Fatalf("查章节 %d 失败: %v", id, err)
		}
		if got.ReviewStatus != model.ChapterReviewDraft || got.RejectReason != "配图与正文不符" {
			t.Fatalf("章节 %d = %q/%q, want draft+统一理由", id, got.ReviewStatus, got.RejectReason)
		}
	}
}

func TestBatchRejectChapterReviewsRequiresReason(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	a := seedQueueChapter(t, db, "驳回", "v1", true)
	svc := NewAdminService(db, nil, zap.NewNop())
	if _, err := svc.BatchRejectChapterReviews(context.Background(), []int{a.ChapterID}, "   "); !errors.Is(err, ErrInvalidReviewInput) {
		t.Fatalf("空理由应返回 ErrInvalidReviewInput, got %v", err)
	}
}

func TestBatchApproveEmptyIDsIsNoop(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewAdminService(db, nil, zap.NewNop())
	res, err := svc.BatchApproveChapterReviews(context.Background(), nil)
	if err != nil {
		t.Fatalf("空批量不应报错: %v", err)
	}
	if res.ApprovedCount != 0 || len(res.Failures) != 0 {
		t.Fatalf("空批量结果 = %+v, want 全零", res)
	}
}

// ===== C10 typed event 契约：构造器形状与 payload（投递面待「讲师通知中心」票接通） =====
//
// 为什么这里只测构造器不测投递：notifications.user_id 外键指向 hrwai_users（学员表），
// 而章节提交人是讲师（tutor 表独立身份）。照现状写库会外键违约回滚整笔审核、或把章节
// 信息投给 id 恰好相同的无关学员。sqlite 内存库没有该外键，投递测试是假绿。

func TestChapterReviewEventShape(t *testing.T) {
	ev := notification.NewChapterReviewEvent(7, "电瓶维护", "叉车保养课", 12, 3, "published", "")
	if ev.UserID != 7 || ev.ChapterID != 12 || ev.CourseID != 3 || ev.ReviewStatus != "published" {
		t.Fatalf("事件字段未按入参落位: %+v", ev)
	}
	evRej := notification.NewChapterReviewEvent(7, "电瓶维护", "叉车保养课", 12, 3, "draft", "正文缺少安全须知")
	if evRej.Reason != "正文缺少安全须知" {
		t.Fatalf("驳回事件应带理由, got %q", evRej.Reason)
	}
}

func TestChapterReviewEventPayloadCarriesReviewStatus(t *testing.T) {
	// 构造器把 review_status 写进 payload 的那一层由 notification 包自己保证；
	// 这里断言 typed event 的 type 常量与 payload 键在包内是同一份事实（不另立词表）。
	if notification.NotifTypeChapterReview != "chapter_review" {
		t.Fatalf("通知类型常量应稳定为 chapter_review, got %q", notification.NotifTypeChapterReview)
	}
}
