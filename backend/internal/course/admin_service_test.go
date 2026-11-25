// 课程域的 admin/学员端课程 CRUD 行为例（波 3b-1 随域从 internal/service/training_catalog_service_test.go 搬来：
// 这几条的接缝是课程域自己的 AdminService/Service，与 TrainingCatalogService 无关）。
package course

import (
	"errors"
	"reflect"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/coerce"
	"forklift-training/internal/model"
	"forklift-training/internal/scope"
	"forklift-training/internal/testutil"
)

// TestCourseSortOrder 课程 sort_order：创建/更新可设置，列表按 sort_order 升序。
func TestCourseSortOrder(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewAdminService(db, nil, nil, zap.NewNop())

	spec := model.Specialty{Code: "operation", Name: "操作", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&spec)
	lv := model.CourseLevel{Code: "beginner", Name: "入门", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&lv)

	// 创建时设置 sort_order
	category := testutil.SeedTrainingCategory(t, db, "维修")
	created, err := svc.CreateCourse(&CourseInput{
		CourseType:  coerce.StrPtr(model.CourseTypeVideo),
		CategoryID:  &category.ID,
		Name:        coerce.StrPtr("课程A"),
		SpecialtyID: coerce.IntPtr(spec.SpecialtyID), LevelID: coerce.IntPtr(lv.LevelID), SortOrder: coerce.IntPtr(5),
	})
	if err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	if created.SortOrder != 5 {
		t.Fatalf("创建返回的 sort_order 不匹配: %+v", created)
	}
	if created.Category == nil || created.PublicEligibility == nil {
		t.Fatalf("创建响应应返回分类与公开资格: %+v", created)
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
	if updated.Category == nil || updated.PublicEligibility == nil {
		t.Fatalf("更新响应应返回分类与公开资格: %+v", updated)
	}

	// 负值应报错
	if _, err := svc.CreateCourse(&CourseInput{CategoryID: &category.ID, Name: coerce.StrPtr("课程B"), SortOrder: coerce.IntPtr(-1)}); err == nil {
		t.Fatal("负排序值应报错")
	}

	// 列表按 sort_order 升序（0 在 1 前，再按创建时间倒序）
	c0 := model.Course{Name: "课程C", Status: 1, CategoryID: &category.ID, SortOrder: 0,
		SpecialtyID: coerce.IntPtr(spec.SpecialtyID), LevelID: coerce.IntPtr(lv.LevelID), CreatedAt: testutil.Now()}
	db.Create(&c0)
	page, err := svc.GetCourses(1, 10, "", nil, nil, nil, scope.CategoryFilter{}, "")
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
	svc := NewAdminService(db, nil, nil, zap.NewNop())

	spec := model.Specialty{Code: "maintenance", Name: "维修", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&spec)
	lv := model.CourseLevel{Code: "beginner", Name: "入门", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&lv)
	tpl := model.CertificateTemplate{Code: "CERT", Name: "证书", ValidityDays: 730, Status: 1, CreatedAt: testutil.Now()}
	db.Create(&tpl)

	// 前置课程
	prereq := model.Course{Name: "前置课程", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&prereq)

	category := testutil.SeedTrainingCategory(t, db, "维修")
	data := &CourseInput{
		CourseType:            coerce.StrPtr(model.CourseTypeVideo),
		CategoryID:            &category.ID,
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
	if _, err := svc.CreateCourse(&CourseInput{CategoryID: &category.ID, Name: coerce.StrPtr("x"), SpecialtyID: coerce.IntPtr(9999)}); err == nil {
		t.Fatal("不存在的专业方向应报错")
	}
	if _, err := svc.CreateCourse(&CourseInput{CategoryID: &category.ID, Name: coerce.StrPtr("x"), TheoryHours: coerce.IntPtr(-1)}); err == nil {
		t.Fatal("负学时应报错")
	}
	if _, err := svc.CreateCourse(&CourseInput{CategoryID: &category.ID, Name: coerce.StrPtr("x"), PrerequisiteCourseIDs: []int{9999}}); err == nil {
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

func TestApplyCourseTrainingFields_ReferenceIDs(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	credential := model.Credential{Code: "course-credential", Name: "课程证件", CreatedAt: testutil.Now()}
	if err := db.Create(&credential).Error; err != nil {
		t.Fatalf("创建证件失败: %v", err)
	}
	template := model.CertificateTemplate{Code: "COURSE-CERT", Name: "课程证书", ValidityDays: 365, Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&template).Error; err != nil {
		t.Fatalf("创建证书模板失败: %v", err)
	}

	previousCredentialID := credential.ID + 1000
	previousTemplateID := template.ID + 1000
	missingCredentialID := credential.ID + 2000
	missingTemplateID := template.ID + 2000
	tests := []struct {
		name             string
		in               CourseInput
		wantErr          error
		wantCredentialID *int
		wantTemplateID   *int
	}{
		{
			name:    "missing credential keeps its sentinel and wins first",
			in:      CourseInput{CredentialID: coerce.IntPtr(missingCredentialID), CertificateTemplateID: coerce.IntPtr(-1)},
			wantErr: ErrCourseCredentialRefNotFound, wantCredentialID: coerce.IntPtr(previousCredentialID), wantTemplateID: coerce.IntPtr(previousTemplateID),
		},
		{
			name:    "missing certificate template keeps its sentinel",
			in:      CourseInput{CertificateTemplateID: coerce.IntPtr(missingTemplateID)},
			wantErr: model.ErrCertificateTemplateNotFound, wantCredentialID: coerce.IntPtr(previousCredentialID), wantTemplateID: coerce.IntPtr(previousTemplateID),
		},
		{
			name: "zero clears both references",
			in:   CourseInput{CredentialID: coerce.IntPtr(0), CertificateTemplateID: coerce.IntPtr(0)},
		},
		{
			name:    "negative credential wins over negative certificate template",
			in:      CourseInput{CredentialID: coerce.IntPtr(-1), CertificateTemplateID: coerce.IntPtr(-1)},
			wantErr: ErrCourseCredentialIDInvalid, wantCredentialID: coerce.IntPtr(previousCredentialID), wantTemplateID: coerce.IntPtr(previousTemplateID),
		},
		{
			name:    "negative certificate template is rejected",
			in:      CourseInput{CertificateTemplateID: coerce.IntPtr(-1)},
			wantErr: ErrCertificateTemplateIDInvalid, wantCredentialID: coerce.IntPtr(previousCredentialID), wantTemplateID: coerce.IntPtr(previousTemplateID),
		},
		{
			name:             "valid IDs assign both references",
			in:               CourseInput{CredentialID: coerce.IntPtr(credential.ID), CertificateTemplateID: coerce.IntPtr(template.ID)},
			wantCredentialID: coerce.IntPtr(credential.ID), wantTemplateID: coerce.IntPtr(template.ID),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			course := model.Course{
				CredentialID: coerce.IntPtr(previousCredentialID), CertificateTemplateID: coerce.IntPtr(previousTemplateID),
			}
			err := applyCourseTrainingFields(db, &course, &tt.in)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("applyCourseTrainingFields 失败: %v", err)
				}
			} else if !errors.Is(err, tt.wantErr) {
				t.Fatalf("错误 = %v，期望 %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(course.CredentialID, tt.wantCredentialID) {
				t.Fatalf("CredentialID = %v，期望 %v", course.CredentialID, tt.wantCredentialID)
			}
			if !reflect.DeepEqual(course.CertificateTemplateID, tt.wantTemplateID) {
				t.Fatalf("CertificateTemplateID = %v，期望 %v", course.CertificateTemplateID, tt.wantTemplateID)
			}
		})
	}

	t.Run("database errors stay distinct from not found", func(t *testing.T) {
		cause := errors.New("reference lookup failed")
		failingTable := ""
		const callbackName = "test:fail_course_training_reference"
		if err := db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
			if tx.Statement.Table == failingTable {
				_ = tx.AddError(cause)
			}
		}); err != nil {
			t.Fatalf("注册查询失败回调失败: %v", err)
		}
		t.Cleanup(func() { _ = db.Callback().Query().Before("gorm:query").Remove(callbackName) })

		lookupTests := []struct {
			table    string
			in       CourseInput
			notFound error
		}{
			{table: "credential", in: CourseInput{CredentialID: coerce.IntPtr(credential.ID)}, notFound: ErrCourseCredentialRefNotFound},
			{table: "certificate_template", in: CourseInput{CertificateTemplateID: coerce.IntPtr(template.ID)}, notFound: model.ErrCertificateTemplateNotFound},
		}
		for _, tt := range lookupTests {
			t.Run(tt.table, func(t *testing.T) {
				failingTable = tt.table
				course := model.Course{}
				err := applyCourseTrainingFields(db, &course, &tt.in)
				if !errors.Is(err, cause) {
					t.Fatalf("错误 = %v，应保留数据库错误 %v", err, cause)
				}
				if errors.Is(err, tt.notFound) {
					t.Fatalf("数据库错误不应映射为不存在: %v", err)
				}
			})
		}
	})
}

func TestApplyCourseTrainingFields_ErrorPriority(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	category := testutil.SeedTrainingCategory(t, db, "维修")
	credential := model.Credential{Code: "priority-credential", Name: "优先级证件", CreatedAt: testutil.Now()}
	if err := db.Create(&credential).Error; err != nil {
		t.Fatalf("创建证件失败: %v", err)
	}
	specialty := model.Specialty{Code: "priority-specialty", Name: "优先级方向", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&specialty).Error; err != nil {
		t.Fatalf("创建专业方向失败: %v", err)
	}
	level := model.CourseLevel{Code: "priority-level", Name: "优先级等级", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&level).Error; err != nil {
		t.Fatalf("创建课程等级失败: %v", err)
	}
	template := model.CertificateTemplate{Code: "PRIORITY-CERT", Name: "优先级证书", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&template).Error; err != nil {
		t.Fatalf("创建证书模板失败: %v", err)
	}

	tests := []struct {
		name           string
		in             CourseInput
		wantErr        error
		wantCategoryID *int
	}{
		{
			name:    "credential before certificate template",
			in:      CourseInput{CredentialID: coerce.IntPtr(-1), CertificateTemplateID: coerce.IntPtr(-1)},
			wantErr: ErrCourseCredentialIDInvalid,
		},
		{
			name:    "specialty before certificate template",
			in:      CourseInput{SpecialtyID: coerce.IntPtr(0), CertificateTemplateID: coerce.IntPtr(-1)},
			wantErr: ErrCourseSpecialtyIDInvalid,
		},
		{
			name:    "level before certificate template",
			in:      CourseInput{SpecialtyID: coerce.IntPtr(specialty.SpecialtyID), LevelID: coerce.IntPtr(0), CertificateTemplateID: coerce.IntPtr(-1)},
			wantErr: ErrCourseLevelIDInvalid,
		},
		{
			name:    "certential lookup before specialty validation",
			in:      CourseInput{CredentialID: coerce.IntPtr(credential.ID + 1000), SpecialtyID: coerce.IntPtr(0)},
			wantErr: ErrCourseCredentialRefNotFound,
		},
		{
			name:           "category assignment precedes reference validation",
			in:             CourseInput{CategoryID: &category.ID, CredentialID: coerce.IntPtr(-1), SpecialtyID: coerce.IntPtr(0)},
			wantErr:        ErrCourseCredentialIDInvalid,
			wantCategoryID: &category.ID,
		},
		{
			name: "certificate template before theory hours",
			in: CourseInput{
				CredentialID: coerce.IntPtr(credential.ID), SpecialtyID: coerce.IntPtr(specialty.SpecialtyID), LevelID: coerce.IntPtr(level.LevelID),
				CertificateTemplateID: coerce.IntPtr(-1), TheoryHours: coerce.IntPtr(-1),
			},
			wantErr: ErrCertificateTemplateIDInvalid,
		},
		{
			name: "theory hours before practice and sort",
			in: CourseInput{
				CredentialID: coerce.IntPtr(credential.ID), SpecialtyID: coerce.IntPtr(specialty.SpecialtyID), LevelID: coerce.IntPtr(level.LevelID),
				CertificateTemplateID: coerce.IntPtr(template.ID), TheoryHours: coerce.IntPtr(-1), PracticeHours: coerce.IntPtr(-1), SortOrder: coerce.IntPtr(-1),
			},
			wantErr: ErrCourseTheoryHoursNegative,
		},
		{
			name: "practice hours before sort",
			in: CourseInput{
				CredentialID: coerce.IntPtr(credential.ID), SpecialtyID: coerce.IntPtr(specialty.SpecialtyID), LevelID: coerce.IntPtr(level.LevelID),
				CertificateTemplateID: coerce.IntPtr(template.ID), TheoryHours: coerce.IntPtr(0), PracticeHours: coerce.IntPtr(-1), SortOrder: coerce.IntPtr(-1),
			},
			wantErr: ErrCoursePracticeHoursNegative,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			course := model.Course{}
			if err := applyCourseTrainingFields(db, &course, &tt.in); !errors.Is(err, tt.wantErr) {
				t.Fatalf("错误 = %v，期望 %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(course.CategoryID, tt.wantCategoryID) {
				t.Fatalf("CategoryID = %v, want %v", course.CategoryID, tt.wantCategoryID)
			}
		})
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
	course := model.Course{Name: "安全操作规范", Status: 1, CategoryID: &testutil.SeedTrainingCategory(t, db, "维修").ID,
		SpecialtyID: coerce.IntPtr(spec.SpecialtyID), LevelID: coerce.IntPtr(lv.LevelID),
		TheoryHours: 12, PracticeHours: 8, CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}

	// 学员端列表按专业方向/等级过滤
	list, err := svc.GetCourses(1, 10, nil, coerce.IntPtr(spec.SpecialtyID), coerce.IntPtr(lv.LevelID), "", nil)
	if err != nil {
		t.Fatalf("GetCourses 失败: %v", err)
	}
	if list.Total != 1 {
		t.Fatalf("过滤后应 1 条, got %v", list.Total)
	}
	empty, err := svc.GetCourses(1, 10, nil, coerce.IntPtr(spec.SpecialtyID), coerce.IntPtr(9999), "", nil)
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
