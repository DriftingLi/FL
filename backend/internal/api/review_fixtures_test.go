// 本文件：读面夹具助手（#2498 行为变更的夹具配套）——章节先审后发后，学员读面
// 只认已发布槽（spec C11），凡「学员要看章节内容」的夹具都须把章节推到已发布态。
// 走真实过审路径（Submit→Approve）出槽，不直插槽行——夹具与生产路径同构。
package api

import (
	"context"
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/course"
)

// publishChapterForRead 冻结并过审章节（#2498 C4 真实路径出 published 槽）。
func publishChapterForRead(t *testing.T, db *gorm.DB, chapterID int) {
	t.Helper()
	if err := course.SubmitChapterForReview(context.Background(), db, chapterID, 1); err != nil {
		t.Fatalf("夹具提交章节审核失败: %v", err)
	}
	if err := course.ApproveChapterReview(context.Background(), db, chapterID); err != nil {
		t.Fatalf("夹具过审章节失败: %v", err)
	}
}
