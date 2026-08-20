// Ticket 1a（issue #51）：旧分类（course.category）退役 + 课程元数据修正。
// seam：service 层（sqlite 内存库）。期望行为即验收标准：
//   - 创建课程必须携带专业方向/课程等级（旧 category 不再需要）
//   - 学生端列表/详情返回 chapter_count 与 prerequisite_course_ids，不含 category
//   - 管理端列表返回 chapter_count 与 prerequisite_course_ids，不含 category
//   - 编辑课程未携带 prerequisite_course_ids 时前置课程不被清空
package service

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// TestCreateCourseRequiresSpecialtyAndLevel 创建课程必须填专业方向与课程等级。
func TestCreateCourseRequiresSpecialtyAndLevel(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewAdminCourseService(db, nil, zap.NewNop())

	spec := model.Specialty{Code: "maintenance", Name: "维修", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "beginner", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}

	// 只给名称（旧体系曾要求 category，新体系改为方向/等级必填）
	if _, err := svc.CreateCourse(&CourseInput{Name: ptrStr("课程A")}); err == nil || !strings.Contains(err.Error(), "专业方向不能为空") {
		t.Fatalf("缺少专业方向应报错, got: %v", err)
	}
	if _, err := svc.CreateCourse(&CourseInput{Name: ptrStr("课程A"), SpecialtyID: ptrInt(spec.SpecialtyID)}); err == nil || !strings.Contains(err.Error(), "课程等级不能为空") {
		t.Fatalf("缺少课程等级应报错, got: %v", err)
	}

	created, err := svc.CreateCourse(&CourseInput{
		Name: ptrStr("课程A"), SpecialtyID: ptrInt(spec.SpecialtyID), LevelID: ptrInt(lv.LevelID),
	})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	b, _ := json.Marshal(created)
	if bytes.Contains(b, []byte(`"category"`)) {
		t.Fatal("课程 dict 不应包含 category")
	}
}

// seedCatalogCourse 建一门已挂载课程（含章节与前置课程），返回课程与其前置课程。
func seedCatalogCourse(t *testing.T, db *gorm.DB) (*model.Course, *model.Course) {
	t.Helper()
	spec := model.Specialty{Code: "maintenance", Name: "维修", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "beginner", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}
	prereq := model.Course{Name: "前置课程", Status: 1,
		SpecialtyID: ptrInt(spec.SpecialtyID), LevelID: ptrInt(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&prereq).Error; err != nil {
		t.Fatalf("创建前置课程失败: %v", err)
	}
	course := model.Course{Name: "主课程", Status: 1,
		SpecialtyID: ptrInt(spec.SpecialtyID), LevelID: ptrInt(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	for i := 1; i <= 2; i++ {
		ch := model.Chapter{CourseID: course.CourseID, Title: "章节", OrderNum: i, CreatedAt: testutil.Now()}
		if err := db.Create(&ch).Error; err != nil {
			t.Fatalf("创建章节失败: %v", err)
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
	svc := NewCourseService(db, nil, zap.NewNop())
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

	list := svc.GetCourses(1, 10, nil, nil)
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

// TestStudentCourseListOmitsUnmountedCourses 未挂方向/等级的课程不出现在学生端列表（与目录树口径统一）。
func TestStudentCourseListOmitsUnmountedCourses(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewCourseService(db, nil, zap.NewNop())
	seedCatalogCourse(t, db)

	unmounted := model.Course{Name: "未挂载课程", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&unmounted).Error; err != nil {
		t.Fatalf("创建未挂载课程失败: %v", err)
	}

	items := svc.GetCourses(1, 10, nil, nil).Courses
	for _, c := range items {
		if c.CourseID == unmounted.CourseID {
			t.Fatal("未挂方向/等级的课程不应出现在学生端列表")
		}
	}
}

// TestStudentCourseDetailHasChapterCountAndPrereqIDs 学生端详情返回章节数，且无 category。
func TestStudentCourseDetailHasChapterCountAndPrereqIDs(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewCourseService(db, nil, zap.NewNop())
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
	svc := NewAdminCourseService(db, nil, zap.NewNop())
	course, prereq := seedCatalogCourse(t, db)

	list := svc.GetCourses(1, 10, "", nil, nil)
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
	b, _ := json.Marshal(item)
	if bytes.Contains(b, []byte(`"category"`)) {
		t.Fatal("管理端列表不应包含 category")
	}
}

// TestTutorCourseListHasChapterCount 导师端课程列表返回章节数。
func TestTutorCourseListHasChapterCount(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewTutorService(db, "", nil, nil, zap.NewNop())
	course, _ := seedCatalogCourse(t, db)

	list := svc.GetCourses(1, 10, nil, nil)
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
	svc := NewAdminCourseService(db, nil, zap.NewNop())
	course, prereq := seedCatalogCourse(t, db)

	updated, err := svc.UpdateCourse(course.CourseID, &CourseInput{Name: ptrStr("改名")})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	ids := *updated.PrerequisiteCourseIDs
	if len(ids) != 1 || ids[0] != prereq.CourseID {
		t.Fatalf("更新后前置课程应保留 [%d], got %v", prereq.CourseID, ids)
	}
	b, _ := json.Marshal(updated)
	if bytes.Contains(b, []byte(`"category"`)) {
		t.Fatal("更新结果不应包含 category")
	}
}
