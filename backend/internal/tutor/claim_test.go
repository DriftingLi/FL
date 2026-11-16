// 本文件：课程认领域谓词的服务层契约（#2596 / 规格 B2–B7）。
//
// seam 是 tutor.Service 的公开方法（与 service_test.go 同口径）：认领谓词是本票的核心
// 不变式，「能不能改这门课的章节」这件事必须在**每条写路径**上都答一次，故逐条打穿，
// 而不是只测谓词函数本身——只测函数会漏掉「忘了在某条路径上调用」这类真实缺陷。
package tutor

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/course"
	"forklift-training/internal/filestore"
	"forklift-training/internal/model"
	"forklift-training/internal/storage"
	"forklift-training/internal/testutil"
)

// newClaimTestService 带真实文件存储的 tutor.Service（上传/删除附件要落盘，nil 会走「文件服务不可用」）。
func newClaimTestService(t *testing.T, db *gorm.DB) *Service {
	t.Helper()
	fs := filestore.NewFileStore("", storage.NewLocalStorage(t.TempDir()), zap.NewNop())
	return NewService(db, t.TempDir(), fs, nil, zap.NewNop())
}

// seedClaimCourse 建一门已上架课程 + 一个章节，返回二者 id。
func seedClaimCourse(t *testing.T, db *gorm.DB) (int, int) {
	t.Helper()
	spec := model.Specialty{Code: "claim", Name: "认领方向", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "claim-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("建等级失败: %v", err)
	}
	c := model.Course{Name: "认领课程", Status: 1,
		SpecialtyID: &spec.SpecialtyID, LevelID: &lv.LevelID, CreatedAt: testutil.Now()}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("建课程失败: %v", err)
	}
	ch := model.Chapter{CourseID: c.CourseID, Title: "第一章", Content: "正文",
		ContentType: "text", OrderNum: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("建章节失败: %v", err)
	}
	return c.CourseID, ch.ChapterID
}

// TestWritePathsRejectedWithoutClaim 五条写路径 + swap 在未认领时全拒（B2）。
//
// 逐条打穿而不是只测谓词：只测谓词函数抓不到「某条路径忘了调用它」——那正是本票要防的
// 缺陷形态（收敛前所有路径都不校验归属）。
func TestWritePathsRejectedWithoutClaim(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := newClaimTestService(t, db)
	_, chapterID := seedClaimCourse(t, db)
	ctx := context.Background()
	const outsider = 7 // 从未认领

	// 1. 更新章节
	if _, err := svc.UpdateChapterInfo(chapterID, outsider, &course.ChapterInput{}); !errors.Is(err, course.ErrCourseClaimRequired) {
		t.Fatalf("更新章节应拒认领, got %v", err)
	}

	// 2. 上传章节附件
	if _, err := svc.UploadChapterFile(chapterID, outsider, "课件.pdf", []byte("x")); !errors.Is(err, course.ErrCourseClaimRequired) {
		t.Fatalf("上传附件应拒认领, got %v", err)
	}

	// 3. 删除章节附件（先由已认领者造一行，再由未认领者删）
	if err := svc.ClaimCourse(1, 1); err != nil {
		t.Fatalf("讲师 1 认领失败: %v", err)
	}
	uploaded, err := svc.UploadChapterFile(chapterID, 1, "课件.pdf", []byte("x"))
	if err != nil {
		t.Fatalf("认领者上传应放行: %v", err)
	}
	if _, err := svc.DeleteChapterFileByID(uploaded.FileID, outsider); !errors.Is(err, course.ErrCourseClaimRequired) {
		t.Fatalf("删除附件应拒认领, got %v", err)
	}

	// 4. 批量删除附件
	if _, err := svc.BatchDeleteChapterFiles([]int{uploaded.FileID}, outsider); !errors.Is(err, course.ErrCourseClaimRequired) {
		t.Fatalf("批量删除应拒认领, got %v", err)
	}

	// 5. 提交章节审核
	if _, err := svc.SubmitChapterReview(ctx, chapterID, outsider); !errors.Is(err, course.ErrCourseClaimRequired) {
		t.Fatalf("提交审核应拒认领, got %v", err)
	}
}

// TestClaimThenWriteThenRelease 认领→放行→退出→再拒（B2/B6 闭环）。
//
// 还钉住「退出不回溯」：退出后章节作者归属仍在（ADR-0095 决策二），只是今后改不动了。
func TestClaimThenWriteThenRelease(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := newClaimTestService(t, db)
	courseID, chapterID := seedClaimCourse(t, db)
	ctx := context.Background()
	const tutorID = 3

	if err := svc.ClaimCourse(tutorID, courseID); err != nil {
		t.Fatalf("认领失败: %v", err)
	}
	title := "改过的标题"
	if _, err := svc.UpdateChapterInfo(chapterID, tutorID, &course.ChapterInput{Title: &title}); err != nil {
		t.Fatalf("认领后更新章节应放行: %v", err)
	}

	// 归属已落定
	var ch model.Chapter
	if err := db.First(&ch, chapterID).Error; err != nil {
		t.Fatalf("读章节失败: %v", err)
	}
	if ch.TutorID == nil || *ch.TutorID != tutorID {
		t.Fatalf("章节归属应为 %d, got %v", tutorID, ch.TutorID)
	}

	// 退出后：写被拒，但归属保留
	if err := svc.ReleaseCourseClaim(tutorID, courseID); err != nil {
		t.Fatalf("退出认领失败: %v", err)
	}
	if _, err := svc.UpdateChapterInfo(chapterID, tutorID, &course.ChapterInput{Title: &title}); !errors.Is(err, course.ErrCourseClaimRequired) {
		t.Fatalf("退出后写应拒认领, got %v", err)
	}
	if err := db.First(&ch, chapterID).Error; err != nil {
		t.Fatalf("重读章节失败: %v", err)
	}
	if ch.TutorID == nil || *ch.TutorID != tutorID {
		t.Fatalf("退出不回溯：章节归属应仍为 %d, got %v", tutorID, ch.TutorID)
	}

	// 退出后课程回无主：别人可正常认领
	if err := svc.ClaimCourse(4, courseID); err != nil {
		t.Fatalf("末位退出后课程应开放认领: %v", err)
	}
	_ = ctx
}

// TestClaimDuplicateRejected 重复认领拒（B5）：不静默成功、不新增行。
func TestClaimDuplicateRejected(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := newClaimTestService(t, db)
	courseID, _ := seedClaimCourse(t, db)

	if err := svc.ClaimCourse(5, courseID); err != nil {
		t.Fatalf("首次认领失败: %v", err)
	}
	if err := svc.ClaimCourse(5, courseID); !errors.Is(err, course.ErrCourseAlreadyClaimed) {
		t.Fatalf("重复认领应拒 ErrCourseAlreadyClaimed, got %v", err)
	}
	var n int64
	if err := db.Model(&model.TutorCourseClaim{}).
		Where("tutor_id = ? AND course_id = ?", 5, courseID).Count(&n).Error; err != nil {
		t.Fatalf("查认领行失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("认领行应仍为 1 行, got %d", n)
	}

	// 未认领时退出同样是失败（不冒充成功）
	if err := svc.ReleaseCourseClaim(99, courseID); !errors.Is(err, course.ErrCourseNotClaimed) {
		t.Fatalf("未认领退出应拒 ErrCourseNotClaimed, got %v", err)
	}
}

// TestCoClaimBothCanWrite 共认：两位讲师认领同一门课，各自都能写（B1 多对多）。
func TestCoClaimBothCanWrite(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := newClaimTestService(t, db)
	courseID, chapterID := seedClaimCourse(t, db)

	for _, tid := range []int{11, 12} {
		if err := svc.ClaimCourse(tid, courseID); err != nil {
			t.Fatalf("讲师 %d 认领失败: %v", tid, err)
		}
	}
	for _, tid := range []int{11, 12} {
		title := "共认讲师写的标题"
		if _, err := svc.UpdateChapterInfo(chapterID, tid, &course.ChapterInput{Title: &title}); err != nil {
			t.Fatalf("共认讲师 %d 应能写: %v", tid, err)
		}
	}
	// 一方退出不影响另一方
	if err := svc.ReleaseCourseClaim(11, courseID); err != nil {
		t.Fatalf("讲师 11 退出失败: %v", err)
	}
	title := "仍在编的讲师写"
	if _, err := svc.UpdateChapterInfo(chapterID, 12, &course.ChapterInput{Title: &title}); err != nil {
		t.Fatalf("讲师 12 在 11 退出后应仍能写: %v", err)
	}
}

// TestTutorCourseListClaimColumns 课程列表两列（D1）：只对讲师面填、认领人数与「我是否在编」正确。
func TestTutorCourseListClaimColumns(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := newClaimTestService(t, db)
	courseID, _ := seedClaimCourse(t, db)
	// 需要一门「已挂载」课程才进讲师列表：seedClaimCourse 已挂方向/等级，故列表可见。
	if err := svc.ClaimCourse(21, courseID); err != nil {
		t.Fatalf("讲师 21 认领失败: %v", err)
	}
	if err := svc.ClaimCourse(22, courseID); err != nil {
		t.Fatalf("讲师 22 认领失败: %v", err)
	}

	list, err := svc.GetCourses(1, 10, nil, nil, nil, 21)
	if err != nil {
		t.Fatalf("GetCourses 失败: %v", err)
	}
	var found bool
	for _, c := range list.Courses {
		if c.CourseID != courseID {
			continue
		}
		found = true
		if c.ClaimCount == nil || *c.ClaimCount != 2 {
			t.Fatalf("claim_count 应为 2, got %v", c.ClaimCount)
		}
		if c.ClaimedByMe == nil || !*c.ClaimedByMe {
			t.Fatalf("讲师 21 的 claimed_by_me 应为 true, got %v", c.ClaimedByMe)
		}
	}
	if !found {
		t.Fatalf("课程 %d 未出现在讲师列表里", courseID)
	}

	// 换一个没认领的讲师：claimed_by_me=false，人数不变
	list, err = svc.GetCourses(1, 10, nil, nil, nil, 23)
	if err != nil {
		t.Fatalf("GetCourses 失败: %v", err)
	}
	for _, c := range list.Courses {
		if c.CourseID != courseID {
			continue
		}
		if c.ClaimedByMe == nil || *c.ClaimedByMe {
			t.Fatalf("讲师 23 的 claimed_by_me 应为 false, got %v", c.ClaimedByMe)
		}
		if c.ClaimCount == nil || *c.ClaimCount != 2 {
			t.Fatalf("claim_count 应为 2（不含未认领者）, got %v", c.ClaimCount)
		}
	}
}

// TestListCourseClaimants 共认名单：按认领时间升序、带讲师名、课程不存在与空名单可区分（D1）。
func TestListCourseClaimants(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := newClaimTestService(t, db)
	courseID, _ := seedClaimCourse(t, db)

	// 空名单是 200 而不是 404（课程存在、只是没人认领）
	empty, err := svc.ListCourseClaimants(31, courseID)
	if err != nil {
		t.Fatalf("空名单应成功: %v", err)
	}
	if len(empty.Claimants) != 0 {
		t.Fatalf("空名单应为 0 条, got %d", len(empty.Claimants))
	}

	for _, tu := range []model.Tutor{
		{TutorID: 31, Username: "t31", Name: "讲师甲", Status: 1, CreatedAt: testutil.Now()},
		{TutorID: 32, Username: "t32", Name: "讲师乙", Status: 1, CreatedAt: testutil.Now()},
	} {
		if err := db.Create(&tu).Error; err != nil {
			t.Fatalf("建讲师失败: %v", err)
		}
	}
	if err := svc.ClaimCourse(31, courseID); err != nil {
		t.Fatalf("讲师 31 认领失败: %v", err)
	}
	if err := svc.ClaimCourse(32, courseID); err != nil {
		t.Fatalf("讲师 32 认领失败: %v", err)
	}

	res, err := svc.ListCourseClaimants(31, courseID)
	if err != nil {
		t.Fatalf("ListCourseClaimants 失败: %v", err)
	}
	if len(res.Claimants) != 2 {
		t.Fatalf("名单应 2 条, got %d", len(res.Claimants))
	}
	if res.Claimants[0].TutorName != "讲师甲" || res.Claimants[1].TutorName != "讲师乙" {
		t.Fatalf("名单应按认领时间升序且带讲师名, got %+v", res.Claimants)
	}
	if res.Claimants[0].Source != model.ClaimSourceSelf {
		t.Fatalf("自助认领来源应为 self, got %q", res.Claimants[0].Source)
	}
	// 服务端答「我在不在名单里」：讲师 31 认领过 → true
	if !res.ClaimedByMe {
		t.Fatal("讲师 31 已认领，claimed_by_me 应为 true")
	}
	// 换一个没认领的讲师看同一门课：名单相同、claimed_by_me=false
	outsider, err := svc.ListCourseClaimants(99, courseID)
	if err != nil {
		t.Fatalf("未认领者查名单应成功: %v", err)
	}
	if len(outsider.Claimants) != 2 || outsider.ClaimedByMe {
		t.Fatalf("未认领者应看到 2 人名单且 claimed_by_me=false, got %+v %v", outsider.Claimants, outsider.ClaimedByMe)
	}

	// 课程不存在 ≠ 空名单
	if _, err := svc.ListCourseClaimants(1, 999999); !errors.Is(err, model.ErrCourseNotFound) {
		t.Fatalf("不存在课程应拒 ErrCourseNotFound, got %v", err)
	}
}

// TestClaimCourseNotFound 认领不存在的课程拒（不是静默建行）。
func TestClaimCourseNotFound(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := newClaimTestService(t, db)
	if err := svc.ClaimCourse(1, 999999); !errors.Is(err, model.ErrCourseNotFound) {
		t.Fatalf("认领不存在课程应拒 ErrCourseNotFound, got %v", err)
	}
}
