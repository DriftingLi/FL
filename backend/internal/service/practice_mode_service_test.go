// Package service 标签练习抽题测试。
package service

import (
	"fmt"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func newPracticeSvc(t *testing.T) (*PracticeModeService, *gorm.DB) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	return NewPracticeModeService(db, nil, zap.NewNop()), db
}

// TestStartTagPractice 标签练习开始/续练：抽题、顺序固定（断点续练）、完成后重新抽、错误分支。
func TestStartTagPractice(t *testing.T) {
	svc, db := newPracticeSvc(t)
	catalogSvc := NewTrainingCatalogService(db, zap.NewNop())

	tag1, _ := catalogSvc.CreateQuestionTag(QuestionTagInput{Code: "regulation", Name: "法规", SortOrder: ptrInt(1)})
	tag2, _ := catalogSvc.CreateQuestionTag(QuestionTagInput{Code: "hydraulic", Name: "液压", SortOrder: ptrInt(2)})

	qsvc := NewQuestionBankService(db, nil, zap.NewNop())
	q1, err := qsvc.CreateQuestion(map[string]any{
		"type": "single_choice", "content": "法规已发布题", "options": []string{"A", "B"}, "answer": "A",
		"status": "published", "tag_ids": []int{tag1.ID},
	}, nil, "tutor")
	if err != nil {
		t.Fatalf("创建已发布题目失败: %v", err)
	}
	if _, err := qsvc.CreateQuestion(map[string]any{
		"type": "single_choice", "content": "法规草稿题", "options": []string{"A", "B"}, "answer": "A",
		"status": "draft", "tag_ids": []int{tag1.ID},
	}, nil, "tutor"); err != nil {
		t.Fatalf("创建草稿题目失败: %v", err)
	}
	if _, err := qsvc.CreateQuestion(map[string]any{
		"type": "true_false", "content": "液压已发布题1", "answer": "true",
		"status": "published", "tag_ids": []int{tag2.ID},
	}, nil, "tutor"); err != nil {
		t.Fatalf("创建已发布题目失败: %v", err)
	}
	if _, err := qsvc.CreateQuestion(map[string]any{
		"type": "true_false", "content": "液压已发布题2", "answer": "false",
		"status": "published", "tag_ids": []int{tag2.ID},
	}, nil, "tutor"); err != nil {
		t.Fatalf("创建已发布题目失败: %v", err)
	}

	// 首次进入：抽该标签全部已发布题（草稿不出现），游标 0
	got, err := svc.StartTagPractice(1, tag1.ID, 0)
	if err != nil {
		t.Fatalf("抽题失败: %v", err)
	}
	questions := got["questions"].([]QuestionDTO)
	if len(questions) != 1 {
		t.Fatalf("应抽 1 道已发布题, got %d", len(questions))
	}
	if questions[0].ID != q1.ID {
		t.Fatalf("抽题结果不匹配: %+v", questions[0])
	}
	if got["current_index"] != 0 {
		t.Fatalf("首次进入游标应为 0, got %v", got["current_index"])
	}
	// 不返回答案（学员侧）
	if questions[0].Answer != nil {
		t.Fatal("学员侧题目不应含答案")
	}

	// 断点续练：保存游标后再次进入，顺序不变、游标恢复
	r1, err := svc.StartTagPractice(1, tag2.ID, 0)
	if err != nil {
		t.Fatalf("抽题失败: %v", err)
	}
	order1 := make([]int, 0, len(r1["questions"].([]QuestionDTO)))
	for _, q := range r1["questions"].([]QuestionDTO) {
		order1 = append(order1, q.ID)
	}
	mode := fmt.Sprintf("tag:%d", tag2.ID)
	if err := svc.SaveProgress(1, 1, mode, len(order1), nil); err != nil {
		t.Fatalf("保存进度失败: %v", err)
	}
	r2, err := svc.StartTagPractice(1, tag2.ID, 0)
	if err != nil {
		t.Fatalf("续练失败: %v", err)
	}
	if r2["current_index"] != 1 {
		t.Fatalf("续练游标应为 1, got %v", r2["current_index"])
	}
	order2 := make([]int, 0, len(r2["questions"].([]QuestionDTO)))
	for _, q := range r2["questions"].([]QuestionDTO) {
		order2 = append(order2, q.ID)
	}
	if !sameIDSet(order1, order2) {
		t.Fatalf("续练题目集合应一致: %v vs %v", order1, order2)
	}
	for i := range order1 {
		if order1[i] != order2[i] {
			t.Fatalf("续练题目顺序必须与首次一致（游标才有效）: %v vs %v", order1, order2)
		}
	}

	// 已完成（游标==总数）：再次进入重新抽题、游标归零
	if err := svc.SaveProgress(1, len(order1), mode, len(order1), nil); err != nil {
		t.Fatalf("保存完成进度失败: %v", err)
	}
	r3, err := svc.StartTagPractice(1, tag2.ID, 0)
	if err != nil {
		t.Fatalf("重新抽题失败: %v", err)
	}
	if r3["current_index"] != 0 {
		t.Fatalf("完成后再次进入游标应归零, got %v", r3["current_index"])
	}

	// count 限制（新学生首次进入）
	limited, err := svc.StartTagPractice(2, tag2.ID, 1)
	if err != nil {
		t.Fatalf("抽题失败: %v", err)
	}
	if len(limited["questions"].([]QuestionDTO)) != 1 {
		t.Fatalf("count=1 应抽 1 题, got %d", len(limited["questions"].([]QuestionDTO)))
	}

	// 错误分支
	empty, _ := catalogSvc.CreateQuestionTag(QuestionTagInput{Code: "emergency", Name: "应急"})
	if _, err := svc.StartTagPractice(3, empty.ID, 0); err == nil {
		t.Fatal("无题目标签应报错")
	}
	if _, err := svc.StartTagPractice(3, 9999, 0); err == nil {
		t.Fatal("不存在的标签应报错")
	}
	if _, err := svc.StartTagPractice(3, 0, 0); err == nil {
		t.Fatal("非法标签 ID 应报错")
	}
	disabled, _ := catalogSvc.CreateQuestionTag(QuestionTagInput{Code: "off", Name: "停用", Status: p16(0)})
	if _, err := svc.StartTagPractice(3, disabled.ID, 0); err == nil {
		t.Fatal("停用标签应报错")
	}
}

// TestStartTagPractice_QuestionToDict 校验题目 dict 中带标签字段所需字段完整。
func TestGetTagQuestions_QuestionToDict(t *testing.T) {
	svc, db := newPracticeSvc(t)
	catalogSvc := NewTrainingCatalogService(db, zap.NewNop())
	tag, _ := catalogSvc.CreateQuestionTag(QuestionTagInput{Code: "brake", Name: "制动"})
	q := model.Question{Type: "single_choice", Content: "制动题", Answer: "A",
		Options: model.JSONB([]byte(`["A","B"]`)), Status: "published",
		CreatedAt: testutil.Now(), UpdatedAt: testutil.Now()}
	if err := db.Create(&q).Error; err != nil {
		t.Fatalf("创建题目失败: %v", err)
	}
	if err := catalogSvc.SetQuestionTags(q.ID, []int{tag.ID}); err != nil {
		t.Fatalf("打标失败: %v", err)
	}
	got, err := svc.StartTagPractice(4, tag.ID, 0)
	if err != nil {
		t.Fatalf("抽题失败: %v", err)
	}
	qs := got["questions"].([]QuestionDTO)
	if len(qs) != 1 {
		t.Fatalf("应抽 1 题, got %d", len(qs))
	}
	if qs[0].ID != q.ID || qs[0].Content != "制动题" {
		t.Fatalf("题目字段不完整: %+v", qs[0])
	}
}
