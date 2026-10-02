// Package service 留驻 service 侧的题库域测试夹具（#1445 P2 波 3c-1）。
//
// 原夹具随题库域搬去 internal/questionbank，而域包的测试文件不能被 internal/service 反向 import
// —— 留驻侧按「就地内联」处理（同 3b-2 先例）：mock_exam / real_exam 两个留驻测试文件共用这一份。
// （practice_mode 那份随练习域 3c-2 搬去 internal/practicemode 自带副本。）
package service

import (
	"testing"

	"gorm.io/gorm"

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
