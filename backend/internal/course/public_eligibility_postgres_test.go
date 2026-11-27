package course_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"forklift-training/internal/course"
	"forklift-training/internal/model"
	"forklift-training/internal/publicscope"
	"forklift-training/internal/questionbank"
	"forklift-training/internal/questionpool"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

func TestPublicEligibilityAcrossReadPathsPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()
	create := func(value any) {
		t.Helper()
		if err := db.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	update := func(value any, column string, newValue any) {
		t.Helper()
		if err := db.Model(value).UpdateColumn(column, newValue).Error; err != nil {
			t.Fatal(err)
		}
	}

	credentials := []model.Credential{
		{Code: "eligibility_courses", Name: "公开资格课程证件", Category: "special_operation", Status: 1},
		{Code: "eligibility_questions", Name: "公开资格题库证件", Category: "special_operation", Status: 1},
	}
	create(&credentials)
	categories := []model.TrainingCategory{
		{Name: "公开资格课程分类", Status: 1, LegacyCredentialID: &credentials[0].ID},
		{Name: "公开资格题库分类", Status: 1, LegacyCredentialID: &credentials[1].ID},
		{Name: "公开资格停用分类", Status: 1},
	}
	create(&categories)
	update(&categories[2], "status", 0)
	courses := []model.Course{
		{Name: "公开资格正例", Status: 1, CategoryID: &categories[0].ID},
		{Name: "公开资格关闭", Status: 1, CategoryID: &categories[0].ID, PublicHidden: true},
	}
	create(&courses)
	questions := []model.Question{
		{Type: "single_choice", Answer: "A", Content: "公开资格题目正例", Status: "published", CategoryID: &categories[1].ID},
		{Type: "single_choice", Answer: "A", Content: "公开资格多原因题目", Status: "published", CategoryID: &categories[2].ID, PublicHidden: true},
		{Type: "single_choice", Answer: "A", Content: "题库分类内的来源题", Status: "published", CategoryID: &categories[1].ID},
	}
	create(&questions)
	tag := model.QuestionTag{Code: "eligibility_source", Name: "公开资格测试来源", Status: 1, IsSourceTag: true}
	create(&tag)
	otherTag := model.QuestionTag{Code: "eligibility_other_source", Name: "公开资格第二来源", Status: 1, IsSourceTag: true}
	create(&otherTag)
	create(&[]model.QuestionTagRelation{
		{QuestionID: questions[1].ID, TagID: tag.ID},
		{QuestionID: questions[2].ID, TagID: tag.ID},
		{QuestionID: questions[2].ID, TagID: otherTag.ID},
	})
	var poolIDs []int
	if err := db.Table("question").Where("category_id = ?", categories[1].ID).
		Where(questionpool.PublishedSQL).Where(questionpool.ExcludeSourceTagsSQL).
		Pluck("id", &poolIDs).Error; err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(poolIDs, []int{questions[0].ID}) {
		t.Fatalf("同分类的题库池题目 = %v", poolIDs)
	}

	admin := course.NewAdminService(db, nil, nil, zap.NewNop())
	staffCourse, err := admin.GetCourseDetail(courses[1].CourseID)
	if err != nil {
		t.Fatal(err)
	}
	checkEligibility(t, staffCourse.PublicEligibility, publicscope.ReasonManuallyHidden)
	if staffCourse.PublicState == nil || staffCourse.PublicState.PublicVisibility || staffCourse.PublicState.Category == nil || staffCourse.PublicState.Category.ID != categories[0].ID {
		t.Fatalf("PostgreSQL 员工端课程公开状态 = %+v", staffCourse.PublicState)
	}
	staffQuestions := questionbank.NewService(db, nil, zap.NewNop())
	for i, want := range [][]publicscope.Reason{
		{},
		{publicscope.ReasonSourceTagExcluded, publicscope.ReasonCategoryDisabled, publicscope.ReasonCategoryUnmapped, publicscope.ReasonManuallyHidden},
		{publicscope.ReasonSourceTagExcluded},
	} {
		item, err := staffQuestions.GetQuestion(questions[i].ID)
		if err != nil {
			t.Fatal(err)
		}
		checkEligibility(t, item.PublicEligibility, want...)
		wantCategory := categories[1]
		if i == 1 {
			wantCategory = categories[2]
		}
		// 公开开关与公开资格正交，来源标记只影响资格。
		if item.PublicState == nil || item.PublicState.PublicVisibility != !questions[i].PublicHidden || item.PublicState.Category == nil || item.PublicState.Category.ID != wantCategory.ID {
			t.Fatalf("PostgreSQL 员工端题目公开状态 = %+v", item.PublicState)
		}
	}

	public := course.NewService(db, nil, zap.NewNop())
	testutil.SetTestGinMode()
	router := gin.New()
	session := security.NewSession("eligibility-test", time.Hour, security.CookieConfig{})
	course.RegisterRoutes(router.Group("/api"), session, nil, public)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/public-school/courses?category_id=%d", categories[0].ID), nil))
	if w.Code != http.StatusOK {
		t.Fatalf("访客课程分页 = %d: %s", w.Code, w.Body)
	}
	var envelope struct {
		Data course.PublicSchoolCoursePageDTO
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	page := envelope.Data
	if page.Total != 1 || len(page.Courses) != 1 || page.Courses[0].CourseID != courses[0].CourseID {
		t.Fatalf("访客课程分页 = %+v", page)
	}
	if _, err := public.GetPublicSchoolCourseDetail(ctx, courses[0].CourseID); err != nil {
		t.Fatal(err)
	}
	if _, err := public.GetPublicSchoolCourseDetail(ctx, courses[1].CourseID); !errors.Is(err, course.ErrCourseNotVisible) {
		t.Fatalf("已关闭课程详情 = %v", err)
	}
	options, err := public.ListPublicSchoolCategories(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	counts := make(map[int]int64)
	for _, option := range options.Categories {
		counts[option.ID] = option.CourseCount
	}
	if counts[categories[0].ID] != 1 || counts[categories[1].ID] != 0 {
		t.Fatalf("课程分类计数 = %v", counts)
	}
	banks, err := public.ListPublicQuestionBanks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	clear(counts)
	for _, bank := range banks.QuestionBanks {
		counts[bank.CategoryID] = bank.Total
	}
	if counts[categories[1].ID] != 1 || counts[categories[2].ID] != 0 {
		t.Fatalf("题库分类计数 = %v", counts)
	}
}

func checkEligibility(t *testing.T, got *publicscope.Eligibility, want ...publicscope.Reason) {
	t.Helper()
	if got == nil || got.Eligible != (len(want) == 0) || got.Reasons == nil || !slices.Equal(got.Reasons, want) {
		t.Fatalf("公开资格 = %+v，期望阻塞原因 %v", got, want)
	}
}
