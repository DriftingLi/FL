// 练习域留驻测试夹具（#1445 P2 波 3c-2）。
//
// createQuestionAs 与留驻 internal/service/questionbank_fixture_test.go 的同名函数按接缝就地内联
// 各一份（域包测试不得反向 import internal/service，同 3b-2 先例）；seed 出的题目供练习抽题/
// 会话续练测试用。
package practicemode

import (
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/questionbank"
)

// createQuestionAs 经题库域 typed 写面创建（固定 pending），published 走显式发布动作。
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
