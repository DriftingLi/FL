// Ticket 1a（issue #51）：旧分类（course.category）退役 + 课程元数据修正。
// seam：service 层（sqlite 内存库）。期望行为即验收标准：
//   - 创建课程必须携带一级培训分类
//   - 学生端列表/详情返回 chapter_count 与 prerequisite_course_ids，不含 category
//   - 管理端列表返回 chapter_count 与 prerequisite_course_ids，不含 category
//   - 编辑课程未携带 prerequisite_course_ids 时前置课程不被清空
package course

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"forklift-training/internal/notification"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/coerce"
	"forklift-training/internal/model"
	"forklift-training/internal/scope"
	"forklift-training/internal/testutil"
)

func TestCreateCourseRequiresCategory(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewAdminService(db, nil, notification.NewService(db, zap.NewNop()), zap.NewNop())
	category := testutil.SeedTrainingCategory(t, db, "维修")
	name := coerce.StrPtr("课程A")
	if _, err := svc.CreateCourse(&CourseInput{Name: name}); !errors.Is(err, ErrCourseCategoryRequired) {
		t.Fatalf("缺少分类应报错: %v", err)
	}
	if _, err := svc.CreateCourse(&CourseInput{Name: name, CategoryID: coerce.IntPtr(category.ID + 100)}); !errors.Is(err, ErrCourseCategoryNotFound) {
		t.Fatalf("未知分类应报错: %v", err)
	}
	draft := int16(0)
	created, err := svc.CreateCourse(&CourseInput{Name: name, CategoryID: &category.ID, Status: &draft, CourseType: coerce.StrPtr(model.CourseTypeVideo)})
	if err != nil {
		t.Fatal(err)
	}
	if created.CategoryID == nil || *created.CategoryID != category.ID || created.Status != 0 {
		t.Fatalf("创建草稿未保留指定分类与状态: %+v", created)
	}
	var stored model.Course
	if err := db.First(&stored, created.CourseID).Error; err != nil || stored.Status != 0 {
		t.Fatalf("草稿状态未保存: %+v, %v", stored, err)
	}
}

// seedCatalogCourse 建一门已挂载课程（含章节与前置课程），返回课程与其前置课程。
func seedCatalogCourse(t *testing.T, db *gorm.DB) (*model.Course, *model.Course) {
	t.Helper()
	category := testutil.SeedTrainingCategory(t, db, "维修")
	spec := model.Specialty{Code: "maintenance", Name: "维修", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "beginner", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}
	prereq := model.Course{Name: "前置课程", Status: 1, CategoryID: &category.ID,
		SpecialtyID: coerce.IntPtr(spec.SpecialtyID), LevelID: coerce.IntPtr(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&prereq).Error; err != nil {
		t.Fatalf("创建前置课程失败: %v", err)
	}
	course := model.Course{Name: "主课程", Status: 1, CategoryID: &category.ID,
		SpecialtyID: coerce.IntPtr(spec.SpecialtyID), LevelID: coerce.IntPtr(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	for i := 1; i <= 2; i++ {
		ch := model.Chapter{CourseID: course.CourseID, Title: "章节", OrderNum: i, CreatedAt: testutil.Now()}
		if err := db.Create(&ch).Error; err != nil {
			t.Fatalf("创建章节失败: %v", err)
		}
		// C11/C6 计数口径：只数有已发布槽的章节——夹具走真实过审路径出槽
		if err := SubmitChapterForReview(context.Background(), db, ch.ChapterID, 7); err != nil {
			t.Fatalf("夹具提交审核失败: %v", err)
		}
		if err := ApproveChapterReview(context.Background(), db, ch.ChapterID); err != nil {
			t.Fatalf("夹具过审失败: %v", err)
		}
	}
	if err := db.Create(&model.CoursePrerequisite{
		CourseID: course.CourseID, PrerequisiteCourseID: prereq.CourseID, CreatedAt: testutil.Now(),
	}).Error; err != nil {
		t.Fatalf("创建前置关联失败: %v", err)
	}
	return &course, &prereq
}

// TestStudentCourseListHasChapterCountAndPrereqIDs 学生端列表返回章节数与前置课程ID，且无 category。
func TestStudentCourseListHasChapterCountAndPrereqIDs(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewService(db, nil, zap.NewNop())
	course, prereq := seedCatalogCourse(t, db)

	// 关联证书模板后，列表应返回 certificate_name（卡片证书标签用）
	tpl := model.CertificateTemplate{Code: "CERT_TEST", Name: "测试证书", ValidityDays: 365, Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&tpl).Error; err != nil {
		t.Fatalf("创建证书模板失败: %v", err)
	}
	if err := db.Model(&model.Course{}).Where("course_id = ?", course.CourseID).
		Update("certificate_template_id", tpl.ID).Error; err != nil {
		t.Fatalf("关联证书失败: %v", err)
	}

	list, err := svc.GetCourses(1, 10, nil, nil, nil, "", nil)
	if err != nil {
		t.Fatalf("GetCourses 失败: %v", err)
	}
	items := list.Courses
	var item CourseDTO
	for _, c := range items {
		if c.CourseID == course.CourseID {
			item = c
		}
	}
	if item.CourseID == 0 {
		t.Fatalf("未找到主课程, items: %+v", items)
	}
	if item.ChapterCount == nil || *item.ChapterCount != 2 {
		t.Fatalf("chapter_count 应为 2, got %v", item.ChapterCount)
	}
	ids := *item.PrerequisiteCourseIDs
	if len(ids) != 1 || ids[0] != prereq.CourseID {
		t.Fatalf("prerequisite_course_ids 应为 [%d], got %v", prereq.CourseID, ids)
	}
	if item.CertificateName != "测试证书" {
		t.Fatalf("certificate_name 应为 测试证书, got %v", item.CertificateName)
	}
	b, _ := json.Marshal(item)
	if bytes.Contains(b, []byte(`"category"`)) {
		t.Fatal("学生端列表不应包含 category")
	}
}

// TestStudentCourseListOmitsUnmountedCourses 未分类课程不出现在学生端列表（与目录树口径统一）。
func TestStudentCourseListOmitsUnmountedCourses(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewService(db, nil, zap.NewNop())
	seedCatalogCourse(t, db)

	unmounted := model.Course{Name: "未挂载课程", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&unmounted).Error; err != nil {
		t.Fatalf("创建未挂载课程失败: %v", err)
	}

	page, err := svc.GetCourses(1, 10, nil, nil, nil, "", nil)
	if err != nil {
		t.Fatalf("GetCourses 失败: %v", err)
	}
	items := page.Courses
	for _, c := range items {
		if c.CourseID == unmounted.CourseID {
			t.Fatal("未分类课程不应出现在学生端列表")
		}
	}
}

// TestStudentCourseDetailHasChapterCountAndPrereqIDs 学生端详情返回章节数，且无 category。
func TestStudentCourseDetailHasChapterCountAndPrereqIDs(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewService(db, nil, zap.NewNop())
	course, prereq := seedCatalogCourse(t, db)

	detail, err := svc.GetCourseDetail(course.CourseID, 0)
	if err != nil {
		t.Fatalf("详情失败: %v", err)
	}
	info := detail.CourseInfo
	if info.ChapterCount == nil || *info.ChapterCount != 2 {
		t.Fatalf("chapter_count 应为 2, got %v", info.ChapterCount)
	}
	ids := *info.PrerequisiteCourseIDs
	if len(ids) != 1 || ids[0] != prereq.CourseID {
		t.Fatalf("prerequisite_course_ids 应为 [%d], got %v", prereq.CourseID, ids)
	}
	b, _ := json.Marshal(info)
	if bytes.Contains(b, []byte(`"category"`)) {
		t.Fatal("学生端详情不应包含 category")
	}
}

// TestAdminCourseListHasChapterCountAndPrereqIDs 管理端列表返回章节数与前置课程ID。
func TestAdminCourseListHasChapterCountAndPrereqIDs(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewAdminService(db, nil, notification.NewService(db, zap.NewNop()), zap.NewNop())
	course, prereq := seedCatalogCourse(t, db)

	list, err := svc.GetCourses(1, 10, "", nil, nil, nil, scope.CategoryFilter{}, "")
	if err != nil {
		t.Fatalf("GetCourses 失败: %v", err)
	}
	items := list.Courses
	if len(items) != 2 {
		t.Fatalf("应返回 2 门课程, got %d", len(items))
	}
	var item CourseDTO
	for _, c := range items {
		if c.CourseID == course.CourseID {
			item = c
		}
	}
	if item.CourseID == 0 {
		t.Fatal("未找到主课程")
	}
	if item.ChapterCount == nil || *item.ChapterCount != 2 {
		t.Fatalf("chapter_count 应为 2, got %v", item.ChapterCount)
	}
	ids := *item.PrerequisiteCourseIDs
	if len(ids) != 1 || ids[0] != prereq.CourseID {
		t.Fatalf("prerequisite_course_ids 应为 [%d], got %v", prereq.CourseID, ids)
	}
	if item.Category == nil || item.Category.ID != *course.CategoryID {
		t.Fatalf("管理端列表应返回 canonical 分类, got %+v", item.Category)
	}
	if item.PublicEligibility == nil || item.PublicEligibility.Eligible {
		t.Fatalf("未映射证件的分类不应公开, got %+v", item.PublicEligibility)
	}
}

// TestTutorCourseListHasChapterCount 导师端课程列表返回章节数。
// 导师端 GetCourses 是 course.ListCourses 的薄包装（internal/tutor/service.go:41），
// 其行为在 internal/tutor/service_test.go 覆盖（波 4d 随导师域包搬走）；本域包测试直接打底层函数
// （域包测试不能 import internal/core 的测试助手，会成环）。
func TestTutorCourseListHasChapterCount(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	course, _ := seedCatalogCourse(t, db)

	list, err := ListCourses(db, 1, 10, CourseListOptions{OnlyMounted: true, WithStudentCount: true, DefaultPageSize: 10})
	if err != nil {
		t.Fatalf("GetCourses 失败: %v", err)
	}
	items := list.Courses
	var item CourseDTO
	for _, c := range items {
		if c.CourseID == course.CourseID {
			item = c
		}
	}
	if item.CourseID == 0 {
		t.Fatalf("未找到主课程, items: %+v", items)
	}
	if item.ChapterCount == nil || *item.ChapterCount != 2 {
		t.Fatalf("chapter_count 应为 2, got %v", item.ChapterCount)
	}
}

// TestUpdateCourseWithoutPrereqKeyKeepsPrereqs 编辑课程不携带前置课程字段时，前置课程保持原样。
func TestUpdateCourseWithoutPrereqKeyKeepsPrereqs(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewAdminService(db, nil, notification.NewService(db, zap.NewNop()), zap.NewNop())
	course, prereq := seedCatalogCourse(t, db)

	updated, err := svc.UpdateCourse(course.CourseID, &CourseInput{Name: coerce.StrPtr("改名")})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	ids := *updated.PrerequisiteCourseIDs
	if len(ids) != 1 || ids[0] != prereq.CourseID {
		t.Fatalf("更新后前置课程应保留 [%d], got %v", prereq.CourseID, ids)
	}
	if updated.Category == nil || updated.PublicEligibility == nil {
		t.Fatalf("更新结果应返回 canonical 分类与公开资格, got %+v", updated)
	}
}
