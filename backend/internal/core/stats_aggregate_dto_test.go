// Package core 统计聚合 typed DTO shape-lock（Ticket #226）。
// 断言 JSON key 集合与重构前的 map 输出契约逐字一致；练习那套（PracticeStatsDTO）随 3c-2 搬进 internal/practicemode，
// 错题那套（WrongQuestionStatsDTO）随 4c 搬进 internal/wrongquestion。
package core

import "forklift-training/internal/questionbank"

import "testing"

func TestQuestionBankStatsDTOShapeLock(t *testing.T) {
	// 旧 question_service GetStats 顶层：{total, by_type, by_status}
	d := questionbank.QuestionBankStatsDTO{
		Total:    12,
		ByType:   map[string]int64{"single_choice": 5, "multi_choice": 7},
		ByStatus: map[string]int64{"published": 12},
	}
	assertShapeLock(t, d, "total", "by_type", "by_status")
}
