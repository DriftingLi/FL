package questionbank

import (
	"context"
	"reflect"
	"strconv"
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/publicscope"
	"forklift-training/internal/scope"
)

func TestEditorQuestionsCategoryFilterAndPublicState(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	activeCredential := model.Credential{Code: "QUESTION-PUB-A", Name: "有效证件", Category: "special_operation", Status: 1}
	disabledCredential := model.Credential{Code: "QUESTION-PUB-D", Name: "停用证件", Category: "special_operation", Status: 0}
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
	sourceTag := model.QuestionTag{Code: "source-public-state", Name: "真题", IsSourceTag: true, Status: 1}
	if err := db.Create(&sourceTag).Error; err != nil {
		t.Fatal(err)
	}

	questions := []model.Question{
		{Type: "single_choice", Content: "公开题", Status: "published", CategoryID: &activeCategory.ID},
		{Type: "single_choice", Content: "待审真题", Status: "pending", CategoryID: &activeCategory.ID},
		{Type: "single_choice", Content: "停用分类题", Status: "published", CategoryID: &disabledCategory.ID},
		{Type: "single_choice", Content: "未映射分类题", Status: "published", CategoryID: &unmappedCategory.ID},
		{Type: "single_choice", Content: "空分类题", Status: "draft"},
	}
	if err := db.Create(&questions).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.QuestionTagRelation{QuestionID: questions[1].ID, TagID: sourceTag.ID}).Error; err != nil {
		t.Fatal(err)
	}

	activeFilter, err := scope.ParseStaffCategoryFilter(strconv.Itoa(activeCategory.ID), true)
	if err != nil {
		t.Fatal(err)
	}
	activePage, err := svc.ListQuestions(1, 10, "", "", "", nil, activeFilter, NewQuestionEditScope(nil, nil), "")
	if err != nil {
		t.Fatal(err)
	}
	if activePage.Total != 2 || len(activePage.Questions) != 2 {
		t.Fatalf("有效分类筛选应返回 2 条, got %+v", activePage)
	}
	byName := make(map[string]QuestionDTO, len(activePage.Questions))
	for _, item := range activePage.Questions {
		byName[item.Content] = item
	}
	if got := byName["公开题"]; got.Category == nil || got.Category.ID != activeCategory.ID || got.PublicEligibility == nil || !got.PublicEligibility.Eligible {
		t.Fatalf("有效分类题目应入池且返回 canonical 分类, got %+v", got)
	}
	if got := byName["待审真题"]; got.PublicEligibility == nil || !reflect.DeepEqual(got.PublicEligibility.Reasons, []publicscope.Reason{publicscope.ReasonNotPublished, publicscope.ReasonSourceTagExcluded}) {
		t.Fatalf("待审与来源排除应同时返回, got %+v", got.PublicEligibility)
	}

	legacyFilter, err := scope.ParseStaffCategoryFilter("legacy", true)
	if err != nil {
		t.Fatal(err)
	}
	legacyPage, err := svc.ListQuestions(1, 2, "", "", "", nil, legacyFilter, NewQuestionEditScope(nil, nil), "")
	if err != nil {
		t.Fatal(err)
	}
	if legacyPage.Total != 3 || len(legacyPage.Questions) != 2 {
		t.Fatalf("遗留桶应按同一查询计数并分页, got %+v", legacyPage)
	}
	legacyNames := make(map[string]QuestionDTO)
	for _, item := range legacyPage.Questions {
		legacyNames[item.Content] = item
	}
	secondPage, err := svc.ListQuestions(2, 2, "", "", "", nil, legacyFilter, NewQuestionEditScope(nil, nil), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range secondPage.Questions {
		legacyNames[item.Content] = item
	}
	if got := legacyNames["停用分类题"]; got.PublicEligibility == nil || !reflect.DeepEqual(got.PublicEligibility.Reasons, []publicscope.Reason{publicscope.ReasonCategoryDisabled, publicscope.ReasonCredentialDisabled}) {
		t.Fatalf("停用分类与停用证件应同时返回, got %+v", got.PublicEligibility)
	}
	if got := legacyNames["未映射分类题"]; got.PublicEligibility == nil || !reflect.DeepEqual(got.PublicEligibility.Reasons, []publicscope.Reason{publicscope.ReasonCategoryUnmapped}) {
		t.Fatalf("未映射分类应返回映射原因, got %+v", got.PublicEligibility)
	}
	if got := legacyNames["空分类题"]; got.PublicEligibility == nil || !reflect.DeepEqual(got.PublicEligibility.Reasons, []publicscope.Reason{publicscope.ReasonNotPublished, publicscope.ReasonCategoryMissing}) {
		t.Fatalf("空分类草稿应返回工作流与分类原因, got %+v", got.PublicEligibility)
	}

	detail, err := svc.GetQuestion(questions[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.PublicEligibility == nil || !reflect.DeepEqual(detail.PublicEligibility.Reasons, []publicscope.Reason{publicscope.ReasonNotPublished, publicscope.ReasonSourceTagExcluded}) {
		t.Fatalf("详情应返回全部公开阻塞原因, got %+v", detail.PublicEligibility)
	}

	studentPage, err := svc.ListPoolQuestions(1, 20, "", "", nil, NewQuestionReadScope(nil), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range studentPage.Questions {
		if item.Category != nil || item.PublicEligibility != nil {
			t.Fatalf("学员池读面不应泄漏内部公开状态, got %+v", item)
		}
	}
}

func TestQuestionPublicVisibilityPreservesReviewStatus(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	record := model.Question{Type: "true_false", Content: "待审核题目", Answer: "true", Status: "pending"}
	if err := db.Create(&record).Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	eligibility, err := svc.setPublicVisibility(ctx, record.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if eligibility.Eligible || len(eligibility.Reasons) != 3 {
		t.Fatalf("关闭公开应叠加在已有阻塞原因上: %+v", eligibility)
	}
	published, err := svc.PublishQuestion(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if published.Status != "published" || published.PublicEligibility == nil ||
		!reflect.DeepEqual(published.PublicEligibility.Reasons, []publicscope.Reason{
			publicscope.ReasonCategoryMissing, publicscope.ReasonManuallyHidden,
		}) {
		t.Fatalf("审核发布应保留开关: %+v", published)
	}
	eligibility, err = svc.setPublicVisibility(ctx, record.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if eligibility.Eligible || len(eligibility.Reasons) != 1 || eligibility.Reasons[0] != publicscope.ReasonCategoryMissing {
		t.Fatalf("重新开启后仍须符合公开条件: %+v", eligibility)
	}
}
