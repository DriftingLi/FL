package practicemode

import "testing"

// 练习统计 DTO shape-lock（Ticket #226；原 internal/service/stats_aggregate_dto_test.go，随 3c-2 练习域搬入）。
// 断言 JSON key 集合与重构前的 map 输出契约逐字一致；practice by_type 正确率 accuracy 为加性新增 key。

func TestPracticeStatsDTOShapeLock(t *testing.T) {
	// 旧 practice_mode GetStats 顶层：{total, correct, wrong, accuracy, by_type}
	// by_type 每项旧为 {total, correct}；新加 accuracy（加性变更，前端 #227 消费）
	correct := int64(3)
	d := PracticeStatsDTO{
		Total:    10,
		Correct:  correct,
		Wrong:    7,
		Accuracy: 30,
		ByType: map[string]PracticeTypeStat{
			"single_choice": {Total: 5, Correct: 3, Accuracy: 60},
			"true_false":    {Total: 5, Correct: 0, Accuracy: 0},
		},
	}
	assertShapeLock(t, d, "total", "correct", "wrong", "accuracy", "by_type")

	// by_type 每项 key：{total, correct, accuracy}
	typeStat := PracticeTypeStat{Total: 5, Correct: 3, Accuracy: 60}
	assertShapeLock(t, typeStat, "total", "correct", "accuracy")
}
