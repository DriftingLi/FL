// Package service 存量回填的行为锁（ADR-0068 决策 4）：
// 按逐题事实重算派生分值、幂等、且不动对错与 AI 原始分。
package service

import (
	"encoding/json"
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/coerce"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// seedSubmittedMockExamWithResult 落一条已交卷记录（result JSONB 由调用方给定，模拟各代旧口径）。
func seedSubmittedMockExamWithResult(t *testing.T, db *gorm.DB, studentID int, score *float64, result map[string]any) *model.MockExam {
	t.Helper()
	var raw model.JSONB
	if result != nil {
		buf, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("序列化 result 失败: %v", err)
		}
		raw = model.JSONB(buf)
	}
	now := clock.Now()
	mock := &model.MockExam{
		StudentID:  studentID,
		Status:     mockExamStatusSubmitted,
		StartTime:  &now,
		SubmitTime: &now,
		CreatedAt:  now,
		Score:      score,
		Result:     raw,
	}
	if err := db.Create(mock).Error; err != nil {
		t.Fatalf("插入已交卷模考失败: %v", err)
	}
	return mock
}

// legacyDetail 旧口径的单题明细（键序无关，回填按 map 读写）。
func legacyDetail(score float64, aiScore *float64) map[string]any {
	d := map[string]any{
		"question_id":    1,
		"type":           "multi_choice",
		"score":          score,
		"max_score":      4,
		"is_correct":     false,
		"correct_answer": "A,B,C",
	}
	if aiScore != nil {
		d["type"] = "short_answer"
		d["ai_score"] = *aiScore
		d["ai_comment"] = "回答到位"
		d["is_correct"] = nil
		d["max_score"] = 10
	}
	return d
}

// TestBackfillMockExamTotalScores 旧口径（半对与简答分都没进总分）被按逐题事实重算，
// 且重跑一次不改任何值（幂等）。
func TestBackfillMockExamTotalScores(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "李四", "x")

	aiScore := 8.0
	zero := 0.0
	// 旧口径：多选半对 0.7、简答 score=0 / ai_score=8 —— total_score 却是 0。
	mock := seedSubmittedMockExamWithResult(t, db, student.ID, &zero, map[string]any{
		"total_score":     0.0,
		"max_score":       14.0,
		"correct_count":   0,
		"total_questions": 2,
		"accuracy":        0.0,
		"details": []any{
			legacyDetail(0.7, nil),
			legacyDetail(0, &aiScore),
		},
	})

	report, err := BackfillMockExamTotalScores(db)
	if err != nil {
		t.Fatalf("回填失败: %v", err)
	}
	if report.Scanned != 1 || report.Updated != 1 || report.NoFacts != 0 {
		t.Fatalf("回填统计不符: %+v", report)
	}

	var saved model.MockExam
	if err := db.First(&saved, mock.ID).Error; err != nil {
		t.Fatalf("读回记录失败: %v", err)
	}
	if saved.Score == nil || *saved.Score != 8.7 {
		t.Errorf("mock_exam.score = %v, want 8.7", saved.Score)
	}

	var payload map[string]any
	if err := json.Unmarshal(saved.Result, &payload); err != nil {
		t.Fatalf("解析 result 失败: %v", err)
	}
	if got := coerce.ToFloat(payload["total_score"]); got != 8.7 {
		t.Errorf("result.total_score = %v, want 8.7", got)
	}
	details, _ := payload["details"].([]any)
	if len(details) != 2 {
		t.Fatalf("明细应保留 2 条, got %d", len(details))
	}
	// 短答明细：score 写成与 ai_score 同源的数（决策 3 明细同源）。
	sa, _ := details[1].(map[string]any)
	if got := coerce.ToFloat(sa["score"]); got != 8 {
		t.Errorf("短答明细 score = %v, want 8", got)
	}
	if got := coerce.ToFloat(sa["ai_score"]); got != 8 {
		t.Errorf("ai_score 是判分事实，回填不得改动, got %v", got)
	}
	// 不是改判：对错、AI 注释、满分、作答口径一律不动。
	if _, exists := sa["is_correct"]; !exists {
		t.Error("is_correct 键必须原样保留（哪怕是 null）")
	}
	if sa["ai_comment"] != "回答到位" {
		t.Errorf("ai_comment 不应被改动, got %v", sa["ai_comment"])
	}
	if got := coerce.ToFloat(payload["correct_count"]); got != 0 {
		t.Errorf("correct_count 不应被改动, got %v", got)
	}
	if got := coerce.ToFloat(payload["max_score"]); got != 14 {
		t.Errorf("max_score 不应被改动, got %v", got)
	}

	// 幂等：同一条记录重跑一次，Updated 归零且值不变。
	second, err := BackfillMockExamTotalScores(db)
	if err != nil {
		t.Fatalf("二次回填失败: %v", err)
	}
	if second.Scanned != 1 || second.Updated != 0 {
		t.Fatalf("二次回填应不改值: %+v", second)
	}
	var again model.MockExam
	if err := db.First(&again, mock.ID).Error; err != nil {
		t.Fatalf("二次读回失败: %v", err)
	}
	if again.Score == nil || *again.Score != 8.7 {
		t.Errorf("二次回填后 score = %v, want 8.7", again.Score)
	}
}

// TestBackfillMockExamTotalScoresSkipsRecordsWithoutFacts 没有逐题事实的行（空 result /
// details 为 null）与未交卷的行都不动 —— 回填不猜、不把「不知道」写成「0 分」。
func TestBackfillMockExamTotalScoresSkipsRecordsWithoutFacts(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "王五", "x")

	zero := 0.0
	emptyResult := seedSubmittedMockExamWithResult(t, db, student.ID, &zero, nil)
	nullDetails := seedSubmittedMockExamWithResult(t, db, student.ID, &zero, map[string]any{
		"total_score": 0.0,
		"details":     nil,
	})

	// 未交卷：即使带明细也不在回填面内。
	now := clock.Now()
	inProgress := &model.MockExam{
		StudentID: student.ID,
		Status:    mockExamStatusInProgress,
		StartTime: &now,
		CreatedAt: now,
		Result:    model.JSONB([]byte(`{"total_score":0,"details":[{"score":0,"ai_score":9}]}`)),
	}
	if err := db.Create(inProgress).Error; err != nil {
		t.Fatalf("插入进行中记录失败: %v", err)
	}

	report, err := BackfillMockExamTotalScores(db)
	if err != nil {
		t.Fatalf("回填失败: %v", err)
	}
	if report.Scanned != 2 || report.Updated != 0 || report.NoFacts != 2 {
		t.Fatalf("回填统计不符（应只扫已交卷且无事实的两条）: %+v", report)
	}
	for _, id := range []int{emptyResult.ID, nullDetails.ID} {
		var row model.MockExam
		if err := db.First(&row, id).Error; err != nil {
			t.Fatalf("读回 %d 失败: %v", id, err)
		}
		if row.Score == nil || *row.Score != 0 {
			t.Errorf("记录 %d 的 score 不应被改动, got %v", id, row.Score)
		}
	}
	// 未交卷行本就无成绩：回填不得替它写一个 0 分。
	var stillRunning model.MockExam
	if err := db.First(&stillRunning, inProgress.ID).Error; err != nil {
		t.Fatalf("读回进行中记录失败: %v", err)
	}
	if stillRunning.Score != nil {
		t.Errorf("未交卷记录的 score 应保持 nil, got %v", *stillRunning.Score)
	}
}
