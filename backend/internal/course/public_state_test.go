package course

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/publicscope"
	"forklift-training/internal/scope"
	"forklift-training/internal/testutil"
)

func TestAdminCourseCanPublishWithoutCategory(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewAdminService(db, nil, nil, zap.NewNop())
	course := model.Course{Name: "空分类旧课程", Status: 0}
	if err := db.Create(&course).Error; err != nil {
		t.Fatal(err)
	}
	status := int16(1)
	updated, err := svc.UpdateCourse(course.CourseID, &CourseInput{Status: &status})
	if err != nil {
		t.Fatalf("空分类旧课程应允许发布: %v", err)
	}
	if updated.Status != 1 {
		t.Fatalf("发布状态应为 1, got %d", updated.Status)
	}
	if updated.PublicEligibility == nil || !reflect.DeepEqual(updated.PublicEligibility.Reasons, []publicscope.Reason{publicscope.ReasonCategoryMissing}) {
		t.Fatalf("发布后应返回未公开及分类缺失原因, got %+v", updated.PublicEligibility)
	}
}

func TestAdminCoursesCategoryFilterAndPublicState(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	activeCredential := model.Credential{Code: "COURSE-PUB-A", Name: "有效证件", Category: "special_operation", Status: 1}
	disabledCredential := model.Credential{Code: "COURSE-PUB-D", Name: "停用证件", Category: "special_operation", Status: 0}
	if err := db.Create(&activeCredential).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&disabledCredential).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Credential{}).Where("id = ?", disabledCredential.ID).Update("status", 0).Error; err != nil {
		t.Fatal(err)
	}
	activeCategory := model.TrainingCategory{Name: "有效分类", Status: 1, LegacyCredentialID: &activeCredential.ID}
	disabledCategory := model.TrainingCategory{Name: "停用分类", Status: 0, LegacyCredentialID: &disabledCredential.ID}
	unmappedCategory := model.TrainingCategory{Name: "未映射分类", Status: 1}
	if err := db.Create(&activeCategory).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&disabledCategory).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&unmappedCategory).Error; err != nil {
		t.Fatal(err)
	}

	courses := []model.Course{
		{Name: "公开课", Status: 1, CategoryID: &activeCategory.ID},
		{Name: "未发布课", Status: 0, CategoryID: &activeCategory.ID},
		{Name: "停用分类课", Status: 1, CategoryID: &disabledCategory.ID},
		{Name: "未映射分类课", Status: 1, CategoryID: &unmappedCategory.ID},
		{Name: "空分类课", Status: 1},
	}
	if err := db.Create(&courses).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Course{}).Where("course_id = ?", courses[1].CourseID).Update("status", 0).Error; err != nil {
		t.Fatal(err)
	}

	svc := NewAdminService(db, nil, nil, zap.NewNop())
	activeFilter, err := scope.ParseStaffCategoryFilter(strconv.Itoa(activeCategory.ID), true)
	if err != nil {
		t.Fatal(err)
	}
	activePage, err := svc.GetCourses(1, 10, "", nil, nil, nil, activeFilter, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if activePage.Total != 2 || len(activePage.Courses) != 2 {
		t.Fatalf("有效分类筛选应返回 2 条, got %+v", activePage)
	}
	byName := make(map[string]CourseDTO, len(activePage.Courses))
	for _, item := range activePage.Courses {
		byName[item.Name] = item
	}
	if got := byName["公开课"]; got.Category == nil || got.Category.ID != activeCategory.ID || got.PublicEligibility == nil || !got.PublicEligibility.Eligible {
		t.Fatalf("有效分类课程应公开且返回 canonical 分类, got %+v", got)
	}
	if got := byName["公开课"]; got.PublicState == nil || !got.PublicState.PublicVisibility || !got.PublicState.Eligibility.Eligible {
		t.Fatalf("员工端应显式返回公开开关与资格, got %+v", got.PublicState)
	}
	if got := byName["未发布课"]; got.PublicEligibility == nil || !reflect.DeepEqual(got.PublicEligibility.Reasons, []publicscope.Reason{publicscope.ReasonNotPublished}) {
		t.Fatalf("未发布课程应只返回未发布原因, got %+v", got.PublicEligibility)
	}

	legacyFilter, err := scope.ParseStaffCategoryFilter("legacy", true)
	if err != nil {
		t.Fatal(err)
	}
	legacyPage, err := svc.GetCourses(1, 2, "", nil, nil, nil, legacyFilter, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if legacyPage.Total != 3 || legacyPage.Pages != 2 || len(legacyPage.Courses) != 2 {
		t.Fatalf("遗留桶应按同一查询计数并分页, got %+v", legacyPage)
	}
	legacyNames := make(map[string]CourseDTO)
	for _, item := range legacyPage.Courses {
		legacyNames[item.Name] = item
	}
	secondPage, err := svc.GetCourses(2, 2, "", nil, nil, nil, legacyFilter, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range secondPage.Courses {
		legacyNames[item.Name] = item
	}
	if got := legacyNames["停用分类课"]; got.PublicEligibility == nil || !reflect.DeepEqual(got.PublicEligibility.Reasons, []publicscope.Reason{publicscope.ReasonCategoryDisabled, publicscope.ReasonCredentialDisabled}) {
		t.Fatalf("停用分类与停用证件应同时返回, got %+v", got.PublicEligibility)
	}
	if got := legacyNames["未映射分类课"]; got.PublicEligibility == nil || !reflect.DeepEqual(got.PublicEligibility.Reasons, []publicscope.Reason{publicscope.ReasonCategoryUnmapped}) {
		t.Fatalf("未映射分类应返回映射原因, got %+v", got.PublicEligibility)
	}
	if got := legacyNames["空分类课"]; got.PublicEligibility == nil || !reflect.DeepEqual(got.PublicEligibility.Reasons, []publicscope.Reason{publicscope.ReasonCategoryMissing}) {
		t.Fatalf("空分类应返回缺失原因, got %+v", got.PublicEligibility)
	}

	detail, err := svc.GetCourseDetail(courses[0].CourseID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Category == nil || detail.PublicEligibility == nil || !detail.PublicEligibility.Eligible {
		t.Fatalf("详情应返回 canonical 分类与公开资格, got %+v", detail.CourseDTO)
	}
}

func TestAdminCoursePublicStateMissingViewRowFails(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	record := model.Course{Name: "视图缺行课程", Status: 1}
	if err := db.Create(&record).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropView(publicscope.CourseView); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE VIEW course_public_state AS SELECT course_id, NULL AS category_id, '' AS category_name, 0 AS category_status, 0 AS eligible, 0 AS public_visibility, '' AS blocker_codes FROM course WHERE 0").Error; err != nil {
		t.Fatal(err)
	}
	_, err := NewAdminService(db, nil, nil, zap.NewNop()).GetCourseDetail(record.CourseID)
	if !errors.Is(err, publicscope.ErrPublicStateMissing) {
		t.Fatalf("内容行存在但公开视图缺行应显式失败, got %v", err)
	}
}

func TestCoursePublicVisibilityPreservesPublication(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	credential := model.Credential{Code: "COURSE-VISIBILITY", Name: "证件", Category: "special_operation", Status: 1}
	if err := db.Create(&credential).Error; err != nil {
		t.Fatal(err)
	}
	category := model.TrainingCategory{Name: "分类", Status: 1, LegacyCredentialID: &credential.ID}
	if err := db.Create(&category).Error; err != nil {
		t.Fatal(err)
	}
	record := model.Course{Name: "课程", Status: 1, CategoryID: &category.ID}
	if err := db.Create(&record).Error; err != nil {
		t.Fatal(err)
	}
	admin := NewAdminService(db, nil, nil, zap.NewNop())
	ctx := context.Background()
	eligibility, err := admin.setPublicVisibility(ctx, record.CourseID, false)
	if err != nil || eligibility.Eligible || !reflect.DeepEqual(eligibility.Reasons, []publicscope.Reason{publicscope.ReasonManuallyHidden}) {
		t.Fatalf("关闭公开后资格 = %+v, err = %v", eligibility, err)
	}
	name := "编辑后的课程"
	updated, err := admin.UpdateCourse(record.CourseID, &CourseInput{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != 1 || updated.PublicEligibility.Eligible {
		t.Fatalf("编辑应保留发布状态及开关: %+v", updated)
	}
	if updated.PublicState == nil || updated.PublicState.PublicVisibility || len(updated.PublicState.Eligibility.Reasons) != 1 {
		t.Fatalf("编辑后应显式返回关闭的公开开关: %+v", updated.PublicState)
	}
	var count int64
	if err := publicSchoolCourseScope(db.Table("course")).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("公开课程数 = %d, err = %v", count, err)
	}
	eligibility, err = admin.setPublicVisibility(ctx, record.CourseID, true)
	if err != nil || !eligibility.Eligible {
		t.Fatalf("重新允许公开后的资格 = %+v, err = %v", eligibility, err)
	}
	if err := publicSchoolCourseScope(db.Table("course")).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("恢复后的公开课程数 = %d, err = %v", count, err)
	}
}
