// Package service 标签练习抽题测试。
package service

import (
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

// TestGetTagQuestions 标签练习抽题：按标签过滤、count 限制、错误分支。
func TestGetTagQuestions(t *testing.T) {
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
		"type": "true_false", "content": "液压已发布题", "answer": "true",
		"status": "published", "tag_ids": []int{tag2.ID},
	}, nil, "tutor"); err != nil {
		t.Fatalf("创建已发布题目失败: %v", err)
	}

	// 按标签抽全部已发布题目（草稿题不出现）
	got, err := svc.GetTagQuestions(tag1.ID, 0)
	if err != nil {
		t.Fatalf("抽题失败: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("应抽 1 道已发布题, got %d", len(got))
	}
	if got[0]["id"] != q1["id"].(int) {
		t.Fatalf("抽题结果不匹配: %+v", got[0])
	}
	// 不返回答案（学员侧）
	if _, ok := got[0]["answer"]; ok {
		t.Fatal("学员侧题目不应含答案")
	}

	// count 限制
	limited, err := svc.GetTagQuestions(tag2.ID, 1)
	if err != nil {
		t.Fatalf("抽题失败: %v", err)
	}
	if len(limited) != 1 {
		t.Fatalf("count=1 应抽 1 题, got %d", len(limited))
	}

	// 无题目标签
	empty, _ := catalogSvc.CreateQuestionTag(QuestionTagInput{Code: "emergency", Name: "应急"})
	if _, err := svc.GetTagQuestions(empty.ID, 0); err == nil {
		t.Fatal("无题目标签应报错")
	}

	// 不存在的标签 / 停用标签 / 非法 ID
	if _, err := svc.GetTagQuestions(9999, 0); err == nil {
		t.Fatal("不存在的标签应报错")
	}
	if _, err := svc.GetTagQuestions(0, 0); err == nil {
		t.Fatal("非法标签 ID 应报错")
	}
	disabled, _ := catalogSvc.CreateQuestionTag(QuestionTagInput{Code: "off", Name: "停用", Status: p16(0)})
	if _, err := svc.GetTagQuestions(disabled.ID, 0); err == nil {
		t.Fatal("停用标签应报错")
	}
}

// TestGetTagQuestions_QuestionToDict 校验题目 dict 中带标签字段所需字段完整。
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
	got, err := svc.GetTagQuestions(tag.ID, 0)
	if err != nil {
		t.Fatalf("抽题失败: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("应抽 1 题, got %d", len(got))
	}
	if got[0]["id"] != q.ID || got[0]["content"] != "制动题" {
		t.Fatalf("题目字段不完整: %+v", got[0])
	}
}
