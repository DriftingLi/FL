// 课程域的 admin/学员端课程 CRUD 行为例（波 3b-1 随域从 internal/service/training_catalog_service_test.go 搬来：
// 这几条的接缝是课程域自己的 AdminService/Service，与 TrainingCatalogService 无关）。
package course

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/coerce"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// TestCourseSortOrder 课程 sort_order：创建/更新可设置，列表按 sort_order 升序。
func TestCourseSortOrder(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewAdminService(db, nil, zap.NewNop())

	spec := model.Specialty{Code: "operation", Name: "操作", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&spec)
	lv := model.CourseLevel{Code: "beginner", Name: "入门", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&lv)

	// 创建时设置 sort_order
	created, err := svc.CreateCourse(&CourseInput{
		Name:        coerce.StrPtr("课程A"),
		SpecialtyID: coerce.IntPtr(spec.SpecialtyID), LevelID: coerce.IntPtr(lv.LevelID), SortOrder: coerce.IntPtr(5),
	})
	if err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	if created.SortOrder != 5 {
		t.Fatalf("创建返回的 sort_order 不匹配: %+v", created)
	}
	courseID := created.CourseID

	// 更新时修改 sort_order
	updated, err := svc.UpdateCourse(courseID, &CourseInput{SortOrder: coerce.IntPtr(1)})
	if err != nil {
		t.Fatalf("更新课程失败: %v", err)
	}
	if updated.SortOrder != 1 {
		t.Fatalf("更新后的 sort_order 不匹配: %+v", updated)
	}

	// 负值应报错
	if _, err := svc.CreateCourse(&CourseInput{Name: coerce.StrPtr("课程B"), SortOrder: coerce.IntPtr(-1)}); err == nil {
		t.Fatal("负排序值应报错")
	}

	// 列表按 sort_order 升序（0 在 1 前，再按创建时间倒序）
	c0 := model.Course{Name: "课程C", Status: 1, SortOrder: 0,
		SpecialtyID: coerce.IntPtr(spec.SpecialtyID), LevelID: coerce.IntPtr(lv.LevelID), CreatedAt: testutil.Now()}
	db.Create(&c0)
	page, err := svc.GetCourses(1, 10, "", nil, nil, nil, "")
	if err != nil {
		t.Fatalf("GetCourses 失败: %v", err)
	}
	list := page.Courses
	if len(list) != 2 {
		t.Fatalf("应 2 门课程, got %d", len(list))
	}
	if list[0].Name != "课程C" || list[1].Name != "课程A" {
		t.Fatalf("课程列表应按 sort_order 升序: %+v", list)
	}
}

// --- 课程扩展字段与前置课程 ---

func TestAdminCourse_TrainingFields(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewAdminService(db, nil, zap.NewNop())

	spec := model.Specialty{Code: "maintenance", Name: "维修", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&spec)
	lv := model.CourseLevel{Code: "beginner", Name: "入门", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&lv)
	tpl := model.CertificateTemplate{Code: "CERT", Name: "证书", ValidityDays: 730, Status: 1, CreatedAt: testutil.Now()}
	db.Create(&tpl)

	// 前置课程
	prereq := model.Course{Name: "前置课程", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&prereq)

	data := &CourseInput{
		Name:                  coerce.StrPtr("液压系统维护"),
		SpecialtyID:           coerce.IntPtr(spec.SpecialtyID),
		LevelID:               coerce.IntPtr(lv.LevelID),
		CertificateTemplateID: coerce.IntPtr(tpl.ID),
		TheoryHours:           coerce.IntPtr(30),
		PracticeHours:         coerce.IntPtr(20),
		PrerequisiteCourseIDs: []int{prereq.CourseID},
	}
	result, err := svc.CreateCourse(data)
	if err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	courseID := result.CourseID
	if result.TheoryHours != 30 || result.PracticeHours != 20 {
		t.Fatalf("学时字段不匹配: %+v", result)
	}

	detail, err := svc.GetCourseDetail(courseID)
	if err != nil {
		t.Fatalf("获取详情失败: %v", err)
	}
	if detail.Specialty.Name != "维修" {
		t.Fatalf("专业方向元数据缺失: %+v", detail.Specialty)
	}
	if detail.Level.Name != "入门" {
		t.Fatalf("等级元数据缺失: %+v", detail.Level)
	}
	if detail.CertificateTemplate.ValidityDays != 730 {
		t.Fatalf("证书模板元数据缺失: %+v", detail.CertificateTemplate)
	}
	prereqs := *detail.Prerequisites
	if len(prereqs) != 1 || prereqs[0].Name != "前置课程" {
		t.Fatalf("前置课程元数据缺失: %+v", prereqs)
	}

	// 不存在的引用应报错
	if _, err := svc.CreateCourse(&CourseInput{Name: coerce.StrPtr("x"), SpecialtyID: coerce.IntPtr(9999)}); err == nil {
		t.Fatal("不存在的专业方向应报错")
	}
	if _, err := svc.CreateCourse(&CourseInput{Name: coerce.StrPtr("x"), TheoryHours: coerce.IntPtr(-1)}); err == nil {
		t.Fatal("负学时应报错")
	}
	if _, err := svc.CreateCourse(&CourseInput{Name: coerce.StrPtr("x"), PrerequisiteCourseIDs: []int{9999}}); err == nil {
		t.Fatal("不存在的前置课程应报错")
	}

	// 更新：等级不可清空（应用层必填，旧 category 退役后方向/等级为必备维度）
	if _, err := svc.UpdateCourse(courseID, &CourseInput{
		LevelID: coerce.IntPtr(0), PrerequisiteCourseIDs: []int{},
	}); err == nil {
		t.Fatal("清空课程等级应报错")
	}
	// 只替换前置课程（不含方向/等级字段）应成功，且前置课程被清空
	if _, err := svc.UpdateCourse(courseID, &CourseInput{PrerequisiteCourseIDs: []int{}}); err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	detail, _ = svc.GetCourseDetail(courseID)
	if len(*detail.Prerequisites) != 0 {
		t.Fatal("前置课程应被清空")
	}

	// 自己作为前置课程应报错
	if _, err := svc.UpdateCourse(courseID, &CourseInput{PrerequisiteCourseIDs: []int{courseID}}); err == nil {
		t.Fatal("自引用前置课程应报错")
	}

	// 多级依赖成环应报错（C→B→A→C 与 A↔B 两课程环）
	a := model.Course{Name: "课程A", Status: 1, CreatedAt: testutil.Now()}
	b := model.Course{Name: "课程B", Status: 1, CreatedAt: testutil.Now()}
	c := model.Course{Name: "课程C", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&a)
	db.Create(&b)
	db.Create(&c)
	if _, err := svc.UpdateCourse(a.CourseID, &CourseInput{PrerequisiteCourseIDs: []int{b.CourseID}}); err != nil {
		t.Fatalf("设置前置课程失败: %v", err)
	}
	if _, err := svc.UpdateCourse(b.CourseID, &CourseInput{PrerequisiteCourseIDs: []int{c.CourseID}}); err != nil {
		t.Fatalf("设置前置课程失败: %v", err)
	}
	if _, err := svc.UpdateCourse(c.CourseID, &CourseInput{PrerequisiteCourseIDs: []int{a.CourseID}}); err == nil {
		t.Fatal("多级依赖成环应报错")
	}
	if _, err := svc.UpdateCourse(b.CourseID, &CourseInput{PrerequisiteCourseIDs: []int{a.CourseID}}); err == nil {
		t.Fatal("两课程互相依赖应报错")
	}
	// 成环请求被拒绝后，原有关联应保持不变
	detail, err = svc.GetCourseDetail(b.CourseID)
	if err != nil {
		t.Fatalf("获取详情失败: %v", err)
	}
	prereqs = *detail.Prerequisites
	if len(prereqs) != 1 || prereqs[0].Name != "课程C" {
		t.Fatalf("成环拒绝后原关联应保留: %+v", prereqs)
	}
}

// --- 学员端课程详情与列表过滤 ---

func TestCourseService_TrainingFields(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewService(db, nil, zap.NewNop())

	spec := model.Specialty{Code: "safety", Name: "安全", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&spec)
	lv := model.CourseLevel{Code: "beginner", Name: "入门", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&lv)
	course := model.Course{Name: "安全操作规范", Status: 1,
		SpecialtyID: coerce.IntPtr(spec.SpecialtyID), LevelID: coerce.IntPtr(lv.LevelID),
		TheoryHours: 12, PracticeHours: 8, CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}

	// 学员端列表按专业方向/等级过滤
	list, err := svc.GetCourses(1, 10, nil, coerce.IntPtr(spec.SpecialtyID), coerce.IntPtr(lv.LevelID), "")
	if err != nil {
		t.Fatalf("GetCourses 失败: %v", err)
	}
	if list.Total != 1 {
		t.Fatalf("过滤后应 1 条, got %v", list.Total)
	}
	empty, err := svc.GetCourses(1, 10, nil, coerce.IntPtr(spec.SpecialtyID), coerce.IntPtr(9999), "")
	if err != nil {
		t.Fatalf("GetCourses 失败: %v", err)
	}
	if empty.Total != 0 {
		t.Fatal("不存在的等级应过滤为空")
	}

	// 详情含等级/学时元数据
	detail, err := svc.GetCourseDetail(course.CourseID, 0)
	if err != nil {
		t.Fatalf("获取详情失败: %v", err)
	}
	info := detail.CourseInfo
	if info.TheoryHours != 12 || info.PracticeHours != 8 {
		t.Fatalf("学时不匹配: %+v", info)
	}
	if info.Level.Name != "入门" {
		t.Fatalf("等级元数据缺失: %+v", info.Level)
	}
}
