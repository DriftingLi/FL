// Package service 练习流 typed DTO shape-lock（Ticket #225）。
// 断言 JSON key 集合与重构前的 map 输出契约逐字一致——前端契约零改动是最高优先级约束。
package service

import (
	"encoding/json"
	"testing"
)

func TestPracticeStartResultDTOShapeLock(t *testing.T) {
	// 旧 StartTagPractice/StartSequential 的 map 输出顶层 key：
	// {questions, current_index, total, completed}
	d := PracticeStartResultDTO{
		Questions:    []QuestionDTO{{ID: 1}},
		CurrentIndex: 0,
		Total:        5,
		Completed:    0,
	}
	assertShapeLock(t, d,
		"questions", "current_index", "total", "completed",
	)
}

func TestProgressResultDTOShapeLock(t *testing.T) {
	// 旧 GetProgress/GetSequentialProgress 的 map 输出顶层 key：
	// {completed, total, current_index, answers_state}
	d := ProgressResultDTO{
		Completed:    2,
		Total:        10,
		CurrentIndex: 2,
		AnswersState: map[string]any{"a": true},
	}
	assertShapeLock(t, d,
		"completed", "total", "current_index", "answers_state",
	)
}

func TestSubmitResultDTOShapeLock_Objective(t *testing.T) {
	// 旧 SubmitAnswer 客观题输出顶层 key：{is_correct, correct_answer, explanation, question_id, user_answer}
	correct := true
	d := SubmitResultDTO{
		IsCorrect:     &correct,
		CorrectAnswer: "A",
		Explanation:   "解析",
		QuestionID:    42,
		UserAnswer:    "A",
	}
	assertShapeLock(t, d,
		"is_correct", "correct_answer", "explanation", "question_id", "user_answer",
	)
}

func TestSubmitResultDTOShapeLock_ShortAnswerNoAI(t *testing.T) {
	// 简答题但无 AI 评分：追加 reference_answer / scoring_criteria / max_score
	d := SubmitResultDTO{
		CorrectAnswer:   "参考答案",
		Explanation:     "解析",
		QuestionID:      7,
		UserAnswer:      "作答",
		ReferenceAnswer: "参考答案",
		ScoringCriteria: "评分标准",
		MaxScore:        10,
	}
	assertShapeLock(t, d,
		"is_correct", "correct_answer", "explanation", "question_id", "user_answer",
		"reference_answer", "scoring_criteria", "max_score",
	)
}

func TestSubmitResultDTOShapeLock_ShortAnswerWithAI(t *testing.T) {
	// 简答题 + AI 评分：追加 ai_score / ai_comment；降级时追加 ai_fallback
	score := 8.0
	d := SubmitResultDTO{
		CorrectAnswer:   "参考答案",
		Explanation:     "解析",
		QuestionID:      7,
		UserAnswer:      "作答",
		ReferenceAnswer: "参考答案",
		ScoringCriteria: "评分标准",
		MaxScore:        10,
		AIScore:         &score,
		AIComment:       "答得很好",
	}
	assertShapeLock(t, d,
		"is_correct", "correct_answer", "explanation", "question_id", "user_answer",
		"reference_answer", "scoring_criteria", "max_score",
		"ai_score", "ai_comment",
	)
	fallback := true
	d.AIFallback = &fallback
	assertShapeLock(t, d,
		"is_correct", "correct_answer", "explanation", "question_id", "user_answer",
		"reference_answer", "scoring_criteria", "max_score",
		"ai_score", "ai_comment", "ai_fallback",
	)
}

func TestHistoryResultDTOShapeLock(t *testing.T) {
	// 旧 GetHistory 顶层 key：{total, page, page_size, records}
	// 每条 item：{id, student_id, question_id, is_correct, practice_type, user_answer, created_at}
	// + 可选 question（命中题目时出现）
	item := HistoryItemDTO{
		ID:           1,
		StudentID:    42,
		QuestionID:   99,
		IsCorrect:    true,
		PracticeType: "free",
		UserAnswer:   "A",
		CreatedAt:    "2026-08-01T10:00:00+08:00",
	}
	assertShapeLock(t, item,
		"id", "student_id", "question_id", "is_correct", "practice_type", "user_answer", "created_at",
	)
	qd := newQuestionDTO(sampleQuestionForShape(), false)
	item.Question = &qd
	got, _ := json.Marshal(item)
	var asMap map[string]any
	if err := json.Unmarshal(got, &asMap); err != nil {
		t.Fatal(err)
	}
	if _, ok := asMap["question"]; !ok {
		t.Error("命中题目时 JSON 应包含 question key")
	}

	h := HistoryResultDTO{Total: 1, Page: 1, PageSize: 20, Records: []HistoryItemDTO{item}}
	assertShapeLock(t, h, "total", "page", "page_size", "records")
}
