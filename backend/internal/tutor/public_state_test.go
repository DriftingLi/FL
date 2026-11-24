package tutor

import (
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/scope"
	"forklift-training/internal/testutil"
)

func TestTutorLegacyCategoryFilterIncludesUncategorizedPublishedCourses(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := newTutorServiceForTest(t, db)
	category := model.TrainingCategory{Name: "有效分类", Status: 1}
	credential := model.Credential{Code: "TUTOR-PUBLIC", Name: "有效证件", Category: "special_operation", Status: 1}
	if err := db.Create(&credential).Error; err != nil {
		t.Fatal(err)
	}
	category.LegacyCredentialID = &credential.ID
	if err := db.Create(&category).Error; err != nil {
		t.Fatal(err)
	}
	validCourse := model.Course{Name: "有分类课程", Status: 1, CategoryID: &category.ID}
	if err := db.Create(&validCourse).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Course{Name: "空分类旧课程", Status: 1}).Error; err != nil {
		t.Fatal(err)
	}

	defaultPage, err := svc.GetCourses(1, 10, nil, nil, nil, scope.CategoryFilter{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if defaultPage.Total != 1 || defaultPage.Courses[0].Name != "有分类课程" {
		t.Fatalf("讲师默认列表应保持原口径, got %+v", defaultPage.Courses)
	}
	detail, err := svc.GetCourseChapters(validCourse.CourseID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Course.Category == nil || detail.Course.PublicEligibility == nil || !detail.Course.PublicEligibility.Eligible {
		t.Fatalf("讲师课程详情应返回 canonical 分类与公开资格, got %+v", detail.Course)
	}

	legacy, err := scope.ParseStaffCategoryFilter("legacy", true)
	if err != nil {
		t.Fatal(err)
	}
	legacyPage, err := svc.GetCourses(1, 10, nil, nil, nil, legacy, 0)
	if err != nil {
		t.Fatal(err)
	}
	if legacyPage.Total != 1 || legacyPage.Courses[0].Name != "空分类旧课程" {
		t.Fatalf("遗留桶应定位已发布空分类旧课, got %+v", legacyPage.Courses)
	}
	if legacyPage.Courses[0].PublicEligibility == nil || legacyPage.Courses[0].PublicEligibility.Eligible {
		t.Fatalf("空分类旧课应返回未公开资格, got %+v", legacyPage.Courses[0].PublicEligibility)
	}
}
