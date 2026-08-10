// Package service 测试：定级考试 DTO shape-lock（B4 决策 D6）。
// 断言 JSON key 集合与 B4 前的 map 契约逐字一致——前端契约零改动是最高优先级约束，
// 任何 DTO 字段增删都会在这里暴露，需同步评估前端影响。
package service

import (
	"encoding/json"
	"testing"
)

// topLevelKeys 返回 v 序列化后的顶层 key 集合。
func topLevelKeys(t *testing.T, v any) map[string]bool {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	keys := map[string]bool{}
	for k := range m {
		keys[k] = true
	}
	return keys
}

// marshalJSON 序列化 v（shape-lock 字节断言辅助）。
func marshalJSON(t *testing.T, v any) ([]byte, error) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	return b, nil
}

// assertShapeLock 断言 key 集合与期望完全一致（不多不少）。
func assertShapeLock(t *testing.T, v any, want ...string) {
	t.Helper()
	got := topLevelKeys(t, v)
	if len(got) != len(want) {
		t.Errorf("key 数量 = %d, 期望 %d\n实际: %v\n期望: %v", len(got), len(want), got, want)
	}
	for _, k := range want {
		if !got[k] {
			t.Errorf("缺少 key: %s", k)
		}
	}
}

func sampleSession() LevelExamSessionDTO {
	return LevelExamSessionDTO{
		ID: 1, Name: "2026年7月定级考", StartTime: "2026-07-01T09:00:00", EndTime: "2026-07-01T11:00:00",
		Duration: 90, Status: "upcoming", TotalScore: 100, PassScore: 60,
		CreatedAt: "2026-06-01T10:00:00", UpdatedAt: "2026-06-01T10:00:00",
	}
}

func TestLevelExamSessionDTOShapeLock(t *testing.T) {
	sess := sampleSession()
	// created_by nil → null；question_config nil → null（key 均必须存在）
	assertShapeLock(t, sess,
		"id", "name", "start_time", "end_time", "duration", "status",
		"created_by", "question_config", "total_score", "pass_score", "created_at", "updated_at",
	)
}

func TestLevelExamSessionDTOParticipantsOptional(t *testing.T) {
	sess := sampleSession()
	// participants 未装配时省略（omitempty）
	if keys := topLevelKeys(t, sess); keys["participants"] {
		t.Error("participants 应为可选字段（未装配时省略）")
	}
	// 装配时逐字携带 participant 全部 key
	sess.Participants = []LevelExamParticipantDTO{sampleParticipant()}
	assertShapeLock(t, sess,
		"id", "name", "start_time", "end_time", "duration", "status",
		"created_by", "question_config", "total_score", "pass_score", "created_at", "updated_at",
		"participants",
	)
}

func sampleParticipant() LevelExamParticipantDTO {
	return LevelExamParticipantDTO{
		ID: 1, ExamSessionID: 1, StudentID: 42, Status: "submitted",
		RemainingTime: 0, CreatedAt: "2026-07-01T09:05:00",
		IsPassed: true,
	}
}

func TestLevelExamParticipantDTOShapeLock(t *testing.T) {
	p := sampleParticipant()
	// 指针字段 nil → null；snapshot/ids nil → null；附加字段（student_name/session_name）省略
	assertShapeLock(t, p,
		"id", "exam_session_id", "student_id", "status", "start_time", "submit_time",
		"remaining_time", "answers_snapshot", "question_ids", "created_at",
		"score", "objective_score", "subjective_score", "is_passed",
	)

	// 附加字段装配时出现
	p.StudentName = "张三"
	p.SessionName = "2026年7月定级考"
	assertShapeLock(t, p,
		"id", "exam_session_id", "student_id", "status", "start_time", "submit_time",
		"remaining_time", "answers_snapshot", "question_ids", "created_at",
		"score", "objective_score", "subjective_score", "is_passed",
		"student_name", "session_name",
	)
}

func TestLevelExamAnswerDTOShapeLock(t *testing.T) {
	a := LevelExamAnswerDTO{ID: 1, ExamParticipantID: 1, QuestionID: 10, UserAnswer: "A", Score: 3}
	// 指针字段 nil → null（graded_at/ai_graded_at 等）
	assertShapeLock(t, a,
		"id", "exam_participant_id", "question_id", "user_answer", "score",
		"grading_comment", "ai_comment", "is_correct", "grader_id", "graded_at",
		"ai_score", "ai_graded_at",
	)

	// question 为结果详情附加字段（省略时不出现在契约中）
	a.Question = map[string]any{"id": 10}
	assertShapeLock(t, a,
		"id", "exam_participant_id", "question_id", "user_answer", "score",
		"grading_comment", "ai_comment", "is_correct", "grader_id", "graded_at",
		"ai_score", "ai_graded_at", "question",
	)
}

func TestLevelExamDataDTOShapeLock(t *testing.T) {
	d := LevelExamDataDTO{
		ParticipantID: 1,
		Session:       sampleSession(),
		Questions:     []map[string]any{},
		Answers:       map[string]any{},
		RemainingTime: 5400,
		StartTime:     "2026-07-01T09:05:00",
	}
	assertShapeLock(t, d,
		"participant_id", "session", "questions", "answers", "remaining_time", "start_time",
	)
}

func TestLevelExamEnvelopeDTOShapeLock(t *testing.T) {
	list := LevelExamSessionListDTO{Total: 3, Page: 1, PageSize: 20, Sessions: []LevelExamSessionDTO{sampleSession()}}
	assertShapeLock(t, list, "total", "page", "page_size", "sessions")

	hist := LevelExamHistoryDTO{Total: 2, Page: 1, PageSize: 10, Records: []LevelExamParticipantDTO{sampleParticipant()}}
	assertShapeLock(t, hist, "total", "page", "page_size", "records")

	result := LevelExamResultDTO{Participant: sampleParticipant(), Answers: []LevelExamAnswerDTO{{ID: 1}}}
	assertShapeLock(t, result, "participant", "answers")
}

func TestLevelExamAvailableDTOShapeLock(t *testing.T) {
	item := LevelExamAvailableDTO{
		LevelExamSessionDTO: sampleSession(),
		Status:              "ongoing",
		HasParticipated:     false,
		CanEnter:            true,
	}
	// session 全字段 + 可用性附加字段；participant_status/participant_id 未参与 → null（key 存在）
	assertShapeLock(t, item,
		"id", "name", "start_time", "end_time", "duration", "status",
		"created_by", "question_config", "total_score", "pass_score", "created_at", "updated_at",
		"has_participated", "participant_status", "participant_id", "can_enter",
	)

	// status 由外层字段覆盖 session 的原始状态（JSON 只出现一个 status）
	b, _ := json.Marshal(item)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if m["status"] != "ongoing" {
		t.Errorf("status 应被可用性外层字段覆盖为 %q, got %v", "ongoing", m["status"])
	}

	item.ParticipantStatus = "submitted"
	item.ParticipantID = 7
	assertShapeLock(t, item,
		"id", "name", "start_time", "end_time", "duration", "status",
		"created_by", "question_config", "total_score", "pass_score", "created_at", "updated_at",
		"has_participated", "participant_status", "participant_id", "can_enter",
	)
}
