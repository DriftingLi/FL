// 本文件：读面夹具助手（#2498 行为变更的夹具配套）——章节先审后发后，学员读面
// 只认已发布槽（spec C11），凡「学员要看章节内容」的夹具都须把章节推到已发布态。
// 走真实过审路径（Submit→Approve）出槽，不直插槽行——夹具与生产路径同构。
package api

import (
	"context"
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/course"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
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

// seedTutorCourseClaim 让讲师认领某门课（#2596）：讲师面写路径全部前置认领谓词，
// 凡「讲师经 HTTP 写章节」的夹具都须先落这一行，否则夹具会撞 403 而测不到本来要测的东西。
//
// 认领行是**数据前置**而不是被测行为：本助手直接插行、不走认领端点——
// 认领端点自己由 tutor/claim_test.go 与 authz 面的签到覆盖，夹具不为它做二次背书。
func seedTutorCourseClaim(t *testing.T, db *gorm.DB, tutorID, courseID int) {
	t.Helper()
	var n int64
	if err := db.Model(&model.TutorCourseClaim{}).
		Where("tutor_id = ? AND course_id = ?", tutorID, courseID).Count(&n).Error; err != nil {
		t.Fatalf("查认领行失败: %v", err)
	}
	if n > 0 {
		return
	}
	row := model.TutorCourseClaim{TutorID: tutorID, CourseID: courseID, Source: model.ClaimSourceSelf, CreatedAt: testutil.Now()}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("造认领行失败（讲师 %d / 课程 %d）: %v", tutorID, courseID, err)
	}
}
