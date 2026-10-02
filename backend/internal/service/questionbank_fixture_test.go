// Package service 留驻 service 侧的题库域测试夹具（#1445 P2 波 3c-1）。
//
// 原夹具随题库域搬去 internal/questionbank，而域包的测试文件不能被 internal/service 反向 import
// —— 留驻侧按「就地内联」处理（同 3b-2 先例）：mock_exam / real_exam / practice_mode 三个留驻
// 测试文件共用这一份，3c-2 练习域搬走后再由 practicemode 侧自带副本。
package service

import (
	"encoding/json"
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/questionbank"
)

// createQuestionAs 经票 6 typed 写面创建（固定 pending），published 走显式发布动作。
func createQuestionAs(t *testing.T, svc *questionbank.Service, db *gorm.DB, in questionbank.QuestionCreateInput, status string) questionbank.QuestionDTO {
	t.Helper()
	q, err := svc.CreateQuestion(in, nil, "tutor")
	if err != nil {
		t.Fatalf("fixture 创建题目失败: %v", err)
	}
	if status == "published" {
		q, err = svc.PublishQuestion(q.ID)
		if err != nil {
			t.Fatalf("fixture 发布题目失败: %v", err)
		}
	}
	return q
}

// sampleQuestionForShape 留驻侧 shape-lock 参照题目（随 question_dto_shape_test.go 搬去题库域后
// 按接缝就地内联；practice_mode_dto_shape_test.go 用它构造命中题目的 HistoryItemDTO）。
func sampleQuestionForShape() *model.Question {
	options, _ := json.Marshal([]map[string]string{
		{"A": "选项A"}, {"B": "选项B"}, {"C": "选项C"},
	})
	createdBy := 7
	credID := 3
	return &model.Question{
		ID:              42,
		Type:            "multi_choice",
		Content:         "题干",
		Options:         model.JSONB(options),
		ImageURL:        "https://example.com/q.png",
		Status:          "published",
		RejectReason:    "驳回理由",
		Score:           4,
		CreatedBy:       &createdBy,
		CreatedByType:   "tutor",
		CredentialID:    &credID,
		Answer:          "A,B",
		Explanation:     "解析",
		ReferenceAnswer: "参考答案",
		ScoringCriteria: "评分标准",
	}
}
