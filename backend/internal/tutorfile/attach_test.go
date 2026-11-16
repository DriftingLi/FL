// 本文件：文件库挂接与管理面服务契约（#2497）——单挂/解挂/换挂/改名/引用期删除拒绝/筛选。
// 只经公开方法断言（与 service_test.go 同口径）；落行造数据沿 seedFileRows 先例做前置。
package tutorfile

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"forklift-training/internal/course"
	"forklift-training/internal/model"
)

// seedChapter 造一章（course_id=1）**并让讲师 1 认领该课程**（#2596：换挂走认领谓词前置，
// 未认领的讲师连「旧文件未挂该章」这类校验都到不了）。需要别的讲师也能写时另调 seedClaim。
func seedChapter(t *testing.T, svc *Service, title string) int {
	t.Helper()
	ch := model.Chapter{CourseID: 1, Title: title, ContentType: "text", CreatedAt: time.Now()}
	if err := svc.db.Create(&ch).Error; err != nil {
		t.Fatalf("造章节失败: %v", err)
	}
	seedClaim(t, svc, 1, ch.CourseID)
	return ch.ChapterID
}

// seedClaim 造一条课程认领行（#2596）。多讲师共认同一门课是正常形态，故按 (讲师, 课程) 幂等。
func seedClaim(t *testing.T, svc *Service, tutorID, courseID int) {
	t.Helper()
	var n int64
	if err := svc.db.Model(&model.TutorCourseClaim{}).
		Where("tutor_id = ? AND course_id = ?", tutorID, courseID).Count(&n).Error; err != nil {
		t.Fatalf("查认领行失败: %v", err)
	}
	if n > 0 {
		return
	}
	row := model.TutorCourseClaim{TutorID: tutorID, CourseID: courseID, Source: model.ClaimSourceSelf, CreatedAt: time.Now()}
	if err := svc.db.Create(&row).Error; err != nil {
		t.Fatalf("造认领行失败: %v", err)
	}
}

func makeFile(t *testing.T, svc *Service, tutorID int, name string) *model.TutorFile {
	t.Helper()
	f := &model.TutorFile{
		TutorID:     tutorID,
		FileURL:     "/static/uploads/tutor-files/1/" + name,
		FileName:    name,
		ContentType: "video",
		FileSize:    1024,
		CreatedAt:   time.Now(),
	}
	if err := svc.db.Create(f).Error; err != nil {
		t.Fatalf("造文件行失败: %v", err)
	}
	return f
}

func reloadFile(t *testing.T, svc *Service, fileID int) *model.TutorFile {
	t.Helper()
	var f model.TutorFile
	if err := svc.db.First(&f, "file_id = ?", fileID).Error; err != nil {
		t.Fatalf("读文件行失败: %v", err)
	}
	return &f
}

func TestAttachDetachSingleAttachMutex(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	chA := seedChapter(t, svc, "第一章")
	chB := seedChapter(t, svc, "第二章")
	f := makeFile(t, svc, 1, "录课.mp4")

	// 挂接：行带章节归属
	if err := svc.AttachFile(ctx, 1, f.FileID, chA); err != nil {
		t.Fatalf("AttachFile 失败: %v", err)
	}
	got := reloadFile(t, svc, f.FileID)
	if got.ChapterID == nil || *got.ChapterID != chA {
		t.Fatalf("挂接后 chapter_id = %v, want %d", got.ChapterID, chA)
	}

	// 单挂互斥：再挂别章拒绝
	if err := svc.AttachFile(ctx, 1, f.FileID, chB); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("重复挂接应拒 ErrInvalidState, got %v", err)
	}

	// 挂到同章幂等
	if err := svc.AttachFile(ctx, 1, f.FileID, chA); err != nil {
		t.Fatalf("重复挂同章应幂等: %v", err)
	}

	// 他人文件挂接拒绝
	if err := svc.AttachFile(ctx, 2, f.FileID, chB); !errors.Is(err, ErrForbidden) {
		t.Fatalf("他人挂接应拒 ErrForbidden, got %v", err)
	}

	// 解挂回库：chapter_id 清空
	if err := svc.DetachFile(ctx, 1, f.FileID); err != nil {
		t.Fatalf("DetachFile 失败: %v", err)
	}
	got = reloadFile(t, svc, f.FileID)
	if got.ChapterID != nil {
		t.Fatalf("解挂后 chapter_id 应为空, got %v", *got.ChapterID)
	}

	// 回库后可挂别章
	if err := svc.AttachFile(ctx, 1, f.FileID, chB); err != nil {
		t.Fatalf("解挂后再挂失败: %v", err)
	}

	// 不存在的章节拒绝
	f2 := makeFile(t, svc, 1, "课件.pdf")
	if err := svc.AttachFile(ctx, 1, f2.FileID, 99999); !errors.Is(err, course.ErrChapterNotFound) {
		t.Fatalf("挂不存在章节应拒章节不存在, got %v", err)
	}
}

func TestSwapChapterFileAtomic(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	ch := seedChapter(t, svc, "第一章")
	old := makeFile(t, svc, 1, "旧.mp4")
	fresh := makeFile(t, svc, 1, "新.mp4")
	if err := svc.AttachFile(ctx, 1, old.FileID, ch); err != nil {
		t.Fatalf("先挂旧文件失败: %v", err)
	}

	// 换挂：挂新 + 旧回库，一步原子
	if err := svc.SwapChapterFile(ctx, 1, ch, old.FileID, fresh.FileID); err != nil {
		t.Fatalf("SwapChapterFile 失败: %v", err)
	}
	gotOld := reloadFile(t, svc, old.FileID)
	gotNew := reloadFile(t, svc, fresh.FileID)
	if gotOld.ChapterID != nil {
		t.Fatalf("旧文件应已回库, chapter_id=%v", *gotOld.ChapterID)
	}
	if gotNew.ChapterID == nil || *gotNew.ChapterID != ch {
		t.Fatalf("新文件应挂上章, chapter_id=%v", gotNew.ChapterID)
	}

	// 旧文件未挂该章时拒绝
	f3 := makeFile(t, svc, 1, "游离.mp4")
	if err := svc.SwapChapterFile(ctx, 1, ch, f3.FileID, old.FileID); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("旧文件未挂该章应拒 ErrInvalidState, got %v", err)
	}

	// 新文件已挂别章时拒绝（单挂互斥在换挂同样成立）
	other := seedChapter(t, svc, "第二章")
	f4 := makeFile(t, svc, 1, "占用.mp4")
	if err := svc.AttachFile(ctx, 1, f4.FileID, other); err != nil {
		t.Fatalf("挂 f4 失败: %v", err)
	}
	if err := svc.SwapChapterFile(ctx, 1, ch, fresh.FileID, f4.FileID); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("新文件已挂别章应拒 ErrInvalidState, got %v", err)
	}

	// old == new 拒绝
	if err := svc.SwapChapterFile(ctx, 1, ch, fresh.FileID, fresh.FileID); !errors.Is(err, ErrInvalidParam) {
		t.Fatalf("old==new 应拒 ErrInvalidParam, got %v", err)
	}

	// 他人文件参与换挂拒绝：讲师 2 也认领同一门课（共认是正常形态），否则先撞认领谓词、
	// 测不到「文件归属」这条（#2596）
	seedClaim(t, svc, 2, 1)
	if err := svc.SwapChapterFile(ctx, 2, ch, fresh.FileID, old.FileID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("他人换挂应拒 ErrForbidden, got %v", err)
	}
}

// TestSwapChapterFileRequiresClaim 换挂受认领谓词约束（#2596 / 规格 B2 + #2595 遗留 swap 洞）：
// 没认领这门课的讲师换不了它的附件，即便文件都是自己的。
func TestSwapChapterFileRequiresClaim(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	ch := seedChapter(t, svc, "第一章") // 讲师 1 已认领
	other := seedChapter(t, svc, "第二章")

	// 讲师 2：文件都是他的，但没认领这门课
	fOld := makeFile(t, svc, 2, "旧.mp4")
	fNew := makeFile(t, svc, 2, "新.mp4")
	if err := svc.AttachFile(ctx, 2, fOld.FileID, ch); err != nil {
		t.Fatalf("挂 fOld 失败: %v", err)
	}
	if err := svc.SwapChapterFile(ctx, 2, ch, fOld.FileID, fNew.FileID); !errors.Is(err, course.ErrCourseClaimRequired) {
		t.Fatalf("未认领讲师换挂应拒 ErrCourseClaimRequired, got %v", err)
	}

	// 认领后同一次调用放行（谓词不再拦），换挂自身的前置仍照常生效
	seedClaim(t, svc, 2, 1)
	if err := svc.SwapChapterFile(ctx, 2, ch, fOld.FileID, fNew.FileID); err != nil {
		t.Fatalf("认领后换挂应放行: %v", err)
	}
	_ = other
}

func TestDeleteFileRefusedWhileReferencedThenDeletable(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	ch := seedChapter(t, svc, "第一章")
	f := makeFile(t, svc, 1, "录课.mp4")
	if err := svc.AttachFile(ctx, 1, f.FileID, ch); err != nil {
		t.Fatalf("挂接失败: %v", err)
	}

	// 引用期删除拒绝并明示原因（B5）
	err := svc.DeleteFile(ctx, 1, f.FileID)
	if !errors.Is(err, ErrFileInUse) {
		t.Fatalf("挂接中删除应拒 ErrFileInUse, got %v", err)
	}
	if !strings.Contains(err.Error(), "解挂") {
		t.Fatalf("拒绝信息应含可行动原因「解挂」, got %q", err.Error())
	}

	// 解挂后可删（行与存储都删掉——沿既有 DeleteFile 语义）
	if err := svc.DetachFile(ctx, 1, f.FileID); err != nil {
		t.Fatalf("解挂失败: %v", err)
	}
	if err := svc.DeleteFile(ctx, 1, f.FileID); err != nil {
		t.Fatalf("解挂后删除失败: %v", err)
	}
	var count int64
	svc.db.Model(&model.TutorFile{}).Where("file_id = ?", f.FileID).Count(&count)
	if count != 0 {
		t.Fatalf("删除后行应不存在, count=%d", count)
	}
}

// TestDeleteFileRefusedWhileSlotReferenced C13（#2498 B5 扩展）：在审快照 / 已发布槽
// 引用的文件视同挂接中——已解挂仍不可删（否则版本槽引用悬空），版本槽引用消失后可删。
func TestDeleteFileRefusedWhileSlotReferenced(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	ch := seedChapter(t, svc, "第一章")
	f := makeFile(t, svc, 1, "录课.mp4")
	if err := svc.AttachFile(ctx, 1, f.FileID, ch); err != nil {
		t.Fatalf("挂接失败: %v", err)
	}
	// 提交审核冻结清单（真实路径出在审快照）
	if err := course.SubmitChapterForReview(ctx, svc.db, ch, 1); err != nil {
		t.Fatalf("提交审核失败: %v", err)
	}
	// 解挂回库：live 挂接没了，但快照仍引用
	if err := svc.DetachFile(ctx, 1, f.FileID); err != nil {
		t.Fatalf("解挂失败: %v", err)
	}
	err := svc.DeleteFile(ctx, 1, f.FileID)
	if !errors.Is(err, ErrFileInUse) {
		t.Fatalf("被在审快照引用的文件删除应拒 ErrFileInUse, got %v", err)
	}
	if !strings.Contains(err.Error(), "在审/已发布版本") {
		t.Fatalf("拒绝信息应明示版本槽引用, got %q", err.Error())
	}
	// 版本槽引用消失（驳回弃快照）后可删
	if err := course.RejectChapterReview(ctx, svc.db, ch, "不行"); err != nil {
		t.Fatalf("驳回失败: %v", err)
	}
	if err := svc.DeleteFile(ctx, 1, f.FileID); err != nil {
		t.Fatalf("快照弃置后删除失败: %v", err)
	}
}

func TestRenameFileUpdatesDisplayName(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	f := makeFile(t, svc, 1, "录课.mp4")

	if err := svc.RenameFile(ctx, 1, f.FileID, "叉车维修第一节.mp4"); err != nil {
		t.Fatalf("RenameFile 失败: %v", err)
	}
	got := reloadFile(t, svc, f.FileID)
	if got.FileName != "叉车维修第一节.mp4" {
		t.Fatalf("改名后 file_name = %q", got.FileName)
	}

	if err := svc.RenameFile(ctx, 1, f.FileID, "  "); !errors.Is(err, ErrInvalidParam) {
		t.Fatalf("空名应拒 ErrInvalidParam, got %v", err)
	}
	if err := svc.RenameFile(ctx, 2, f.FileID, "x.mp4"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("他人改名应拒 ErrForbidden, got %v", err)
	}
}

func TestListFilesFilters(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	ch := seedChapter(t, svc, "第一章")
	video := makeFile(t, svc, 1, "录课.mp4") // video，将挂接
	doc := makeFile(t, svc, 1, "讲义.pdf")   // document，未挂
	doc.ContentType = "document"
	svc.db.Save(doc)
	if err := svc.AttachFile(ctx, 1, video.FileID, ch); err != nil {
		t.Fatalf("挂接失败: %v", err)
	}

	// 类型筛选
	got, err := svc.ListFiles(ctx, 1, FileFilter{ContentType: "video"})
	if err != nil || len(got) != 1 || got[0].FileID != video.FileID {
		t.Fatalf("类型筛选不符: %v %v", got, err)
	}

	// 挂接状态筛选
	attached, err := svc.ListFiles(ctx, 1, FileFilter{Attached: boolPtr(true)})
	if err != nil || len(attached) != 1 || attached[0].FileID != video.FileID {
		t.Fatalf("已挂筛选不符: %v %v", attached, err)
	}
	unattached, err := svc.ListFiles(ctx, 1, FileFilter{Attached: boolPtr(false)})
	if err != nil || len(unattached) != 1 || unattached[0].FileID != doc.FileID {
		t.Fatalf("未挂筛选不符: %v %v", unattached, err)
	}

	// 时间区间筛选（只命中 doc：created_at 移到未来，分界取 video 落行之后）
	cut := video.CreatedAt.Add(time.Second)
	svc.db.Model(&model.TutorFile{}).Where("file_id = ?", doc.FileID).
		Update("created_at", time.Now().Add(time.Hour))
	ranged, err := svc.ListFiles(ctx, 1, FileFilter{CreatedFrom: cut})
	if err != nil || len(ranged) != 1 || ranged[0].FileID != doc.FileID {
		t.Fatalf("时间筛选不符: %v %v", ranged, err)
	}

	// 空过滤=全量（既有语义不回归）
	all, err := svc.ListFiles(ctx, 1, FileFilter{})
	if err != nil || len(all) != 2 {
		t.Fatalf("空过滤应全量: %v %v", all, err)
	}
}

func boolPtr(b bool) *bool { return &b }
