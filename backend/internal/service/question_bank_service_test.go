// Package service 题库服务 CRUD 测试，使用内存 sqlite 数据库。
package service

import (
	"encoding/json"
	"errors"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func newQuestionBankSvc(t *testing.T) (*QuestionBankService, *gorm.DB) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	return NewQuestionBankService(db, nil, zap.NewNop()), db
}

// createQuestionAs 测试 fixture 单点：经票 6 typed 写面创建（固定 pending），
// 到达目标状态——published 走显式发布动作，draft 直接改列（写面已无 status 通道，
// 造「被驳回回退」等历史状态形态必须绕面）。
func createQuestionAs(t *testing.T, svc *QuestionBankService, db *gorm.DB, in QuestionCreateInput, status string) QuestionDTO {
	t.Helper()
	q, err := svc.CreateQuestion(in, nil, "tutor")
	if err != nil {
		t.Fatalf("fixture 创建题目失败: %v", err)
	}
	switch status {
	case "", "pending":
		return q
	case "published":
		q, err = svc.PublishQuestion(q.ID)
		if err != nil {
			t.Fatalf("fixture 发布题目失败: %v", err)
		}
		return q
	case "draft":
		if err := db.Model(&model.Question{}).Where("id = ?", q.ID).Update("status", "draft").Error; err != nil {
			t.Fatalf("fixture 置 draft 失败: %v", err)
		}
		q.Status = "draft"
		return q
	}
	t.Fatalf("未知 fixture 状态: %s", status)
	return QuestionDTO{}
}

// --- CreateQuestion（票 6 typed 面）---

func TestCreateQuestion_SingleChoice_Success(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	createdBy := 1
	in := QuestionCreateInput{
		Type:    "single_choice",
		Content: "叉车作业前应检查什么？",
		Options: json.RawMessage(`["轮胎气压","油位","制动系统","以上全部"]`),
		Answer:  json.RawMessage(`"D"`),
	}
	result, err := svc.CreateQuestion(in, &createdBy, "tutor")
	if err != nil {
		t.Fatalf("创建题目失败: %v", err)
	}
	if result.Type != "single_choice" || result.Content != "叉车作业前应检查什么？" {
		t.Fatalf("创建结果不匹配: %+v", result)
	}
	if result.Status != "pending" {
		t.Fatalf("默认状态应为 pending, got %v", result.Status)
	}
	if result.ID == 0 {
		t.Fatal("题目 ID 不应为空")
	}
}

func TestCreateQuestion_InvalidType(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	in := QuestionCreateInput{Type: "invalid_type", Content: "test", Answer: json.RawMessage(`"A"`)}
	_, err := svc.CreateQuestion(in, nil, "tutor")
	if !errors.Is(err, ErrQuestionTypeInvalid) {
		t.Fatalf("应拒绝无效题型（哨兵 ErrQuestionTypeInvalid），got %v", err)
	}
}

func TestCreateQuestion_EmptyContent(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	in := QuestionCreateInput{Type: "single_choice", Answer: json.RawMessage(`"A"`), Options: json.RawMessage(`["A","B"]`)}
	_, err := svc.CreateQuestion(in, nil, "tutor")
	if !errors.Is(err, ErrQuestionContentRequired) {
		t.Fatalf("应拒绝空题干，got %v", err)
	}
}

func TestCreateQuestion_MissingOptions(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	in := QuestionCreateInput{Type: "single_choice", Content: "test", Answer: json.RawMessage(`"A"`)}
	_, err := svc.CreateQuestion(in, nil, "tutor")
	if !errors.Is(err, ErrQuestionOptionsRequired) {
		t.Fatalf("单选题应要求选项，got %v", err)
	}
}

func TestCreateQuestion_ShortAnswer_NoAnswer(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	in := QuestionCreateInput{Type: "short_answer", Content: "请描述液压系统工作原理"}
	result, err := svc.CreateQuestion(in, nil, "tutor")
	if err != nil {
		t.Fatalf("简答题可不提供答案: %v", err)
	}
	if result.Type != "short_answer" {
		t.Fatalf("题型应为 short_answer, got %v", result.Type)
	}
}

func TestCreateQuestion_MultiChoice_AnswerArray(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	in := QuestionCreateInput{
		Type:    "multi_choice",
		Content: "多选",
		Options: json.RawMessage(`["A","B","C"]`),
		Answer:  json.RawMessage(`["A","C"]`),
	}
	result, err := svc.CreateQuestion(in, nil, "tutor")
	if err != nil {
		t.Fatalf("数组答案创建失败: %v", err)
	}
	if result.Answer == nil || *result.Answer != "A,C" {
		t.Fatalf("数组答案应逗号连接存储, got %v", result.Answer)
	}
}

func TestCreateQuestion_WithTagIDs(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	catalogSvc := NewTrainingCatalogService(db, zap.NewNop())
	tag, err := catalogSvc.CreateQuestionTag(QuestionTagInput{Code: "hydraulic", Name: "液压"})
	if err != nil {
		t.Fatalf("创建标签失败: %v", err)
	}
	in := QuestionCreateInput{
		Type:    "single_choice",
		Content: "test",
		Options: json.RawMessage(`["A","B"]`),
		Answer:  json.RawMessage(`"A"`),
		TagIDs:  []int{tag.ID},
	}
	result, err := svc.CreateQuestion(in, nil, "tutor")
	if err != nil {
		t.Fatalf("带标签创建失败: %v", err)
	}
	tags := result.Tags.([]map[string]any)
	if len(tags) != 1 || tags[0]["id"] != tag.ID {
		t.Fatalf("标签未关联: %+v", result.Tags)
	}
}

// --- GetQuestion ---

func TestGetQuestion_Success(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	q := testutil.SeedQuestion(t, db, "true_false", "叉车可以超载运行", "false")
	result, err := svc.GetQuestion(q.ID)
	if err != nil {
		t.Fatalf("查询题目失败: %v", err)
	}
	if result.Content != "叉车可以超载运行" {
		t.Fatalf("内容不匹配: %v", result.Content)
	}
}

func TestGetQuestion_NotFound(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	_, err := svc.GetQuestion(9999)
	if err == nil {
		t.Fatal("应返回题目不存在")
	}
}

// --- UpdateQuestion（票 6：typed + 审核不变式）---

func TestUpdateQuestion_Success(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "旧题干", "A")
	newContent := "新题干"
	result, err := svc.UpdateQuestion(q.ID, QuestionUpdateInput{Content: &newContent}, "admin")
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if result.Content != "新题干" {
		t.Fatalf("内容未更新: %v", result.Content)
	}
}

func TestUpdateQuestion_NotFound(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	x := "x"
	_, err := svc.UpdateQuestion(9999, QuestionUpdateInput{Content: &x}, "tutor")
	if !errors.Is(err, ErrQuestionNotFound) {
		t.Fatalf("应返回题目不存在，got %v", err)
	}
}

func TestUpdateQuestion_ReviewInvariant(t *testing.T) {
	changed := "改过的题干"
	same := "原题干"
	score := 5
	cases := []struct {
		name       string
		actor      string
		in         QuestionUpdateInput
		wantStatus string
	}{
		{"讲师改内容 → 回 pending 重审", "tutor", QuestionUpdateInput{Content: &changed}, "pending"},
		{"讲师重提交相同内容 → 编辑未改不动，留在池内", "tutor", QuestionUpdateInput{Content: &same}, "published"},
		{"管理员改内容 → 即时生效保持 published", "admin", QuestionUpdateInput{Content: &changed}, "published"},
		{"讲师只改分值（计分字段）→ 回 pending", "tutor", QuestionUpdateInput{Score: &score}, "pending"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, db := newQuestionBankSvc(t)
			q := testutil.SeedQuestion(t, db, "single_choice", "原题干", "A")
			q.RejectReason = "旧理由"
			db.Save(q)
			result, err := svc.UpdateQuestion(q.ID, c.in, c.actor)
			if err != nil {
				t.Fatalf("更新失败: %v", err)
			}
			if result.Status != c.wantStatus {
				t.Fatalf("status = %q, want %q", result.Status, c.wantStatus)
			}
			if c.wantStatus == "pending" && result.RejectReason != "" {
				t.Fatalf("回 pending 重审应清驳回理由, got %q", result.RejectReason)
			}
		})
	}
}

func TestUpdateQuestion_TagOnlyKeepsStatus(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	catalogSvc := NewTrainingCatalogService(db, zap.NewNop())
	tag, err := catalogSvc.CreateQuestionTag(QuestionTagInput{Code: "t5", Name: "标签五"})
	if err != nil {
		t.Fatal(err)
	}
	q := testutil.SeedQuestion(t, db, "single_choice", "test", "A")
	ids := []int{tag.ID}
	result, err := svc.UpdateQuestion(q.ID, QuestionUpdateInput{TagIDs: &ids}, "tutor")
	if err != nil {
		t.Fatalf("纯标签更新失败: %v", err)
	}
	if result.Status != "published" {
		t.Fatalf("纯分区属性（标签）修改不得动状态, got %q", result.Status)
	}
}

// --- SubmitQuestion（票 6 显式动作）---

func TestSubmitQuestion_DraftToPending(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "待提交题", "A")
	db.Model(q).UpdateColumns(map[string]any{"status": "draft", "reject_reason": "题干不完整"})
	result, err := svc.SubmitQuestion(q.ID)
	if err != nil {
		t.Fatalf("提交审核失败: %v", err)
	}
	if result.Status != "pending" || result.RejectReason != "" {
		t.Fatalf("提交后应 pending 且清理由, got %+v", result)
	}
}

func TestSubmitQuestion_NonDraftRejected(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "已发布题", "A")
	if _, err := svc.SubmitQuestion(q.ID); !errors.Is(err, ErrSubmitNotDraft) {
		t.Fatalf("published 提交应拒（状态前置），got %v", err)
	}
	if _, err := svc.SubmitQuestion(9999); !errors.Is(err, ErrQuestionNotFound) {
		t.Fatalf("不存在应 404 哨兵，got %v", err)
	}
}

// --- questionContentTouched（不变式谓词表驱动）---

func TestQuestionContentTouched(t *testing.T) {
	stored := &model.Question{Type: "single_choice", Content: "旧", Answer: "A", Explanation: "析", Score: 3, Options: model.JSONB(`["A","B"]`)}
	same := "旧"
	diff := "新"
	sameAnswer := json.RawMessage(`"A"`)
	diffAnswer := json.RawMessage(`"B"`)
	arrAnswer := json.RawMessage(`["A"]`) // 数组形态 ["A"] stringify 后与 "A" 相等——不算变化
	sameType := "single_choice"
	diffType := "true_false"
	sameOpts := json.RawMessage(`["B","A"]`) // 键序不同但序列化后…数组序有意义：不相等 → 变化
	eqOpts := json.RawMessage(`["A","B"]`)
	sameScore := 3
	cases := []struct {
		name string
		in   QuestionUpdateInput
		want bool
	}{
		{"无字段", QuestionUpdateInput{}, false},
		{"题干相同", QuestionUpdateInput{Content: &same}, false},
		{"题干变化", QuestionUpdateInput{Content: &diff}, true},
		{"题型相同/变化", QuestionUpdateInput{Type: &sameType}, false},
		{"题型变化", QuestionUpdateInput{Type: &diffType}, true},
		{"答案字符串相同", QuestionUpdateInput{Answer: &sameAnswer}, false},
		{"答案数组归一后相同", QuestionUpdateInput{Answer: &arrAnswer}, false},
		{"答案变化", QuestionUpdateInput{Answer: &diffAnswer}, true},
		{"选项同字节", QuestionUpdateInput{Options: &eqOpts}, false},
		{"选项序变化即内容变化", QuestionUpdateInput{Options: &sameOpts}, true},
		{"分值相同", QuestionUpdateInput{Score: &sameScore}, false},
		{"仅标签不动内容", QuestionUpdateInput{TagIDs: ptr([]int{1})}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q := *stored
			answerChanged := false
			if c.in.Answer != nil {
				s, err := stringifyAnswerJSON(*c.in.Answer)
				if err != nil {
					t.Fatal(err)
				}
				answerChanged = s != q.Answer
			}
			if got := questionContentTouched(&q, c.in, answerChanged); got != c.want {
				t.Fatalf("questionContentTouched = %v, want %v", got, c.want)
			}
		})
	}
}

// --- DeleteQuestion ---

func TestDeleteQuestion_Success(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "test", "A")
	if err := svc.DeleteQuestion(q.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	_, err := svc.GetQuestion(q.ID)
	if err == nil {
		t.Fatal("删除后应查询不到")
	}
}

func TestDeleteQuestion_NotFound(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	err := svc.DeleteQuestion(9999)
	if err == nil {
		t.Fatal("应返回题目不存在")
	}
}

// --- ListQuestions ---

func TestListQuestions_Pagination(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	for i := 0; i < 5; i++ {
		testutil.SeedQuestion(t, db, "single_choice", "题目前5", "A")
	}
	result, err := svc.ListQuestions(1, 2, "", "", "", nil, NewQuestionEditScope(nil), "")
	if err != nil {
		t.Fatalf("ListQuestions 失败: %v", err)
	}
	if result.Total != 5 {
		t.Fatalf("总数应为 5, got %v", result.Total)
	}
	if len(result.Questions) != 2 {
		t.Fatalf("本页应 2 条, got %d", len(result.Questions))
	}
	if result.Page != 1 || result.PageSize != 2 {
		t.Fatalf("分页参数不匹配: %+v", result)
	}
}

func TestListQuestions_FilterByType(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	testutil.SeedQuestion(t, db, "single_choice", "单选题", "A")
	testutil.SeedQuestion(t, db, "true_false", "判断题", "true")
	result, err := svc.ListQuestions(1, 20, "true_false", "", "", nil, NewQuestionEditScope(nil), "")
	if err != nil {
		t.Fatalf("ListQuestions 失败: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("判断题应 1 条, got %v", result.Total)
	}
}

func TestListQuestions_DefaultPage(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	result, err := svc.ListQuestions(0, 0, "", "", "", nil, NewQuestionEditScope(nil), "")
	if err != nil {
		t.Fatalf("ListQuestions 失败: %v", err)
	}
	if result.Page != 1 {
		t.Fatalf("默认页码应为 1, got %v", result.Page)
	}
	if result.PageSize != 20 {
		t.Fatalf("默认页大小应为 20, got %v", result.PageSize)
	}
}

// --- PublishQuestion ---

func TestPublishQuestion_Success(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "test", "A")
	result, err := svc.PublishQuestion(q.ID)
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	if result.Status != "published" {
		t.Fatalf("状态应为 published, got %v", result.Status)
	}
}

func TestPublishQuestion_NotFound(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	_, err := svc.PublishQuestion(9999)
	if err == nil {
		t.Fatal("应返回题目不存在")
	}
}

// --- BatchPublish ---

func TestBatchPublish_Success(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	q1 := testutil.SeedQuestion(t, db, "single_choice", "q1", "A")
	q2 := testutil.SeedQuestion(t, db, "single_choice", "q2", "A")
	result := svc.BatchPublish([]int{q1.ID, q2.ID})
	if result.PublishedCount != 2 {
		t.Fatalf("应发布 2 条, got %v", result.PublishedCount)
	}
}

func TestBatchPublish_PartialNotFound(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	q1 := testutil.SeedQuestion(t, db, "single_choice", "q1", "A")
	result := svc.BatchPublish([]int{q1.ID, 9999})
	if result.PublishedCount != 1 {
		t.Fatalf("应发布 1 条, got %v", result.PublishedCount)
	}
}

func TestBatchPublish_Empty(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	result := svc.BatchPublish([]int{})
	if result.PublishedCount != 0 {
		t.Fatalf("空列表应 0 条, got %v", result.PublishedCount)
	}
}

// --- BatchImport（票 6：typed 条目）---

func TestBatchImport_Success(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	items := []QuestionCreateInput{
		{Type: "single_choice", Content: "导入题1", Options: json.RawMessage(`["A","B"]`), Answer: json.RawMessage(`"A"`)},
		{Type: "true_false", Content: "导入题2", Answer: json.RawMessage(`"true"`)},
	}
	createdBy := 1
	result := svc.BatchImport(items, &createdBy)
	if result.SuccessCount != 2 {
		t.Fatalf("应成功 2 条, got %v", result.SuccessCount)
	}
	if result.ErrorCount != 0 {
		t.Fatalf("应无错误, got %v", result.ErrorCount)
	}
	if result.Errors == nil {
		t.Fatal("errors 应为空切片而非 nil（响应会是 [] 而不是 null）")
	}
}

func TestBatchImport_WithErrors(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	items := []QuestionCreateInput{
		{Type: "single_choice", Content: "有效题", Options: json.RawMessage(`["A","B"]`), Answer: json.RawMessage(`"A"`)},
		{Type: "invalid_type", Content: "无效题"},
		{Type: "single_choice", Content: "缺选项题", Answer: json.RawMessage(`"A"`)},
	}
	result := svc.BatchImport(items, nil)
	if result.SuccessCount != 1 {
		t.Fatalf("应成功 1 条, got %v", result.SuccessCount)
	}
	if result.ErrorCount != 2 {
		t.Fatalf("应错误 2 条, got %v", result.ErrorCount)
	}
	if len(result.Errors) != 2 {
		t.Fatalf("应带 2 条错误明细, got %v", result.Errors)
	}
}

// --- GetStats ---

func TestGetStats_Empty(t *testing.T) {
	svc, _ := newQuestionBankSvc(t)
	result := svc.GetStats(nil)
	if result.Total != 0 {
		t.Fatalf("空库总数应为 0, got %v", result.Total)
	}
}

func TestGetStats_WithData(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	testutil.SeedQuestion(t, db, "single_choice", "q1", "A")
	testutil.SeedQuestion(t, db, "single_choice", "q2", "A")
	testutil.SeedQuestion(t, db, "true_false", "q3", "true")
	result := svc.GetStats(nil)
	if result.Total != 3 {
		t.Fatalf("总数应为 3, got %v", result.Total)
	}
}
