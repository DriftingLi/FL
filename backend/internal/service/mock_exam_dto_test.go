// Package service 测试：模拟考试 DTO shape-lock（B6 决策 D6）。
// 断言 JSON key 集合与 B6 前的 map 契约逐字一致——前端契约零改动是最高优先级约束。
package service

import "testing"

func TestMockExamDTOShapeLock(t *testing.T) {
	start := MockExamStartDTO{
		MockExamID: 1, Duration: 90, TotalScore: 100, TotalQuestions: 40,
		RemainingTime: 5400, Questions: []map[string]any{{"id": 1}},
	}
	assertShapeLock(t, start,
		"mock_exam_id", "duration", "total_score", "total_questions", "remaining_time", "questions",
	)

	resume := MockExamResumeDTO{
		MockExamID: 1, Duration: 90, RemainingTime: 5000,
		Questions: []map[string]any{}, Answers: map[string]any{}, StartTime: "2026-08-01T10:00:00",
	}
	assertShapeLock(t, resume,
		"mock_exam_id", "duration", "remaining_time", "questions", "answers", "start_time",
	)

	// 交卷明细：is_correct/options 恒在（null 语义），AI 字段未评分时省略
	detail := MockExamAnswerDetailDTO{
		QuestionID: 1, Type: "single_choice", Content: "题干", UserAnswer: "A",
		CorrectAnswer: "A", Score: 3, MaxScore: 3, Explanation: "解析",
		Options: nil, IsCorrect: nil,
	}
	assertShapeLock(t, detail,
		"question_id", "type", "content", "user_answer", "correct_answer",
		"score", "max_score", "explanation", "options", "is_correct",
	)
	// AI 评分成功路径：ai_score / ai_comment 出现，ai_fallback 仅降级时出现
	score := 2.5
	comment := "AI 评分"
	detail.AIScore = &score
	detail.AIComment = &comment
	assertShapeLock(t, detail,
		"question_id", "type", "content", "user_answer", "correct_answer",
		"score", "max_score", "explanation", "options", "is_correct",
		"ai_score", "ai_comment",
	)
	fallback := true
	detail.AIFallback = &fallback
	assertShapeLock(t, detail,
		"question_id", "type", "content", "user_answer", "correct_answer",
		"score", "max_score", "explanation", "options", "is_correct",
		"ai_score", "ai_comment", "ai_fallback",
	)

	submit := MockExamSubmitDTO{
		TotalScore: 60, MaxScore: 100, CorrectCount: 20, TotalQuestions: 40,
		Accuracy: 50, Details: []MockExamAnswerDetailDTO{detail},
	}
	assertShapeLock(t, submit,
		"total_score", "max_score", "correct_count", "total_questions", "accuracy", "details",
	)

	result := MockExamResultDTO{
		MockExamSubmitDTO: submit,
		MockExamID:        1,
		SubmitTime:        "2026-08-01T10:30:00",
	}
	assertShapeLock(t, result,
		"total_score", "max_score", "correct_count", "total_questions", "accuracy", "details",
		"mock_exam_id", "submit_time",
	)

	// 历史条目：score 为 null 时 key 仍在（未交卷）
	item := MockExamHistoryItemDTO{
		ID: 1, StudentID: 42, RemainingTime: 5000, Duration: 90,
		Status: "in_progress", CreatedAt: "2026-08-01T10:00:00", Score: nil,
	}
	assertShapeLock(t, item,
		"id", "student_id", "question_ids", "answers", "start_time", "submit_time",
		"remaining_time", "duration", "status", "result", "created_at", "score",
	)

	history := MockExamHistoryDTO{Total: 1, Page: 1, PageSize: 10, Exams: []MockExamHistoryItemDTO{item}}
	assertShapeLock(t, history, "total", "page", "page_size", "exams")
}
