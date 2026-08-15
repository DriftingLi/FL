// Package service 测试：会话进度保存深模块（Ticket #232 C3）。
// 锁定三类新口径行为：提交晚到静默忽略、JSONB null 三态归一、守卫分支（本人+在途）。
// 测试只穿公共 seam（MockExamService.SaveProgress / LevelExamService.SaveAnswer），
// 与深模块 session_progress.go 的内部实现解耦（重构后全绿、契约零漂移）。
package service

import (
	"encoding/json"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// seedMockInProgress 造一条 in_progress 模拟考试记录，返回 (svc, db, mockExamID, studentID)。
func seedMockInProgress(t *testing.T) (*MockExamService, *gorm.DB, int, int) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	svc := NewMockExamService(db, nil, zap.NewNop())

	student := testutil.SeedStudent(t, db, "张三", "x")
	now := time.Now()
	mock := model.MockExam{
		StudentID:     student.ID,
		QuestionIDs:   model.JSONB([]byte("[]")),
		Answers:       model.JSONB([]byte(`{"1":"A"}`)),
		Status:        "in_progress",
		StartTime:     &now,
		RemainingTime: 5400,
		Duration:      90,
		CreatedAt:     now,
	}
	if err := db.Create(&mock).Error; err != nil {
		t.Fatalf("建模拟考失败: %v", err)
	}
	return svc, db, mock.ID, student.ID
}

// TestMockSaveProgressLateAfterSubmit 提交晚到静默忽略：交卷后（status=submitted）
// 的自动保存不再落库，守卫裁定返回非 nil，answers/remaining_time 均不被改写。
func TestMockSaveProgressLateAfterSubmit(t *testing.T) {
	svc, db, mockExamID, studentID := seedMockInProgress(t)

	// 交卷
	if _, err := svc.Submit(mockExamID, studentID); err != nil {
		t.Fatalf("交卷失败: %v", err)
	}
	var before model.MockExam
	if err := db.First(&before, mockExamID).Error; err != nil {
		t.Fatal(err)
	}
	if before.Status != "submitted" {
		t.Fatalf("交卷后状态应为 submitted, got %s", before.Status)
	}

	// 晚到自动保存：应被静默忽略（不落库）
	err := svc.SaveProgress(mockExamID, studentID, map[string]any{"1": "B", "2": "C"}, 1)
	if err == nil {
		t.Fatal("提交后的晚到保存应被守卫拒绝（对齐 level 在途口径）")
	}

	var after model.MockExam
	if err := db.First(&after, mockExamID).Error; err != nil {
		t.Fatal(err)
	}
	if string(after.Answers) != string(before.Answers) {
		t.Fatalf("晚到保存不得改写 answers: before=%s after=%s", before.Answers, after.Answers)
	}
	if after.RemainingTime != before.RemainingTime {
		t.Fatalf("晚到保存不得改写 remaining_time: before=%d after=%d", before.RemainingTime, after.RemainingTime)
	}
}

// TestLevelSaveAnswerLateAfterSubmit 定级考试晚到保存同样静默忽略（现状语义锁定）。
func TestLevelSaveAnswerLateAfterSubmit(t *testing.T) {
	svc, db, participantID, studentID, _ := examSubmitFixture(t, []string{"single_choice"})

	// 交卷
	if _, err := svc.SubmitExam(participantID, studentID, false); err != nil {
		t.Fatalf("交卷失败: %v", err)
	}
	var before model.ExamParticipant
	if err := db.First(&before, participantID).Error; err != nil {
		t.Fatal(err)
	}

	err := svc.SaveAnswer(participantID, studentID, map[string]any{"1": "B"}, 1)
	if err == nil {
		t.Fatal("提交后的晚到保存应被守卫拒绝")
	}
	var after model.ExamParticipant
	if err := db.First(&after, participantID).Error; err != nil {
		t.Fatal(err)
	}
	if string(after.AnswersSnapshot) != string(before.AnswersSnapshot) {
		t.Fatalf("晚到保存不得改写快照: before=%s after=%s", before.AnswersSnapshot, after.AnswersSnapshot)
	}
	if after.RemainingTime != before.RemainingTime {
		t.Fatalf("晚到保存不得改写 remaining_time: before=%d after=%d", before.RemainingTime, after.RemainingTime)
	}
}

// TestMockSaveProgressOwnership 守卫分支：他人会话拒绝、本人+进行中通过。
func TestMockSaveProgressOwnership(t *testing.T) {
	svc, db, mockExamID, studentID := seedMockInProgress(t)

	// 他人（studentID+999）保存应拒绝
	if err := svc.SaveProgress(mockExamID, studentID+999, map[string]any{"1": "B"}, 100); err == nil {
		t.Fatal("他人会话应拒绝保存")
	}

	// 本人+进行中保存应通过并落库
	if err := svc.SaveProgress(mockExamID, studentID, map[string]any{"1": "B"}, 100); err != nil {
		t.Fatalf("本人+进行中保存应通过: %v", err)
	}
	var got model.MockExam
	if err := db.First(&got, mockExamID).Error; err != nil {
		t.Fatal(err)
	}
	if got.RemainingTime != 100 {
		t.Fatalf("remaining_time 应落库为 100, got %d", got.RemainingTime)
	}
}

// TestMarshalAnswersSnapshot 答案快照三态归一：nil/空 一律落库 {}，
// 禁止 JSONB 'null' 落库产生 SQL NULL（#142 口径在保存路径的统一锁定）。
func TestMarshalAnswersSnapshot(t *testing.T) {
	if got := marshalAnswersSnapshot(nil); string(got) != "{}" {
		t.Fatalf("nil 应归一为 {}, got %s", got)
	}
	if got := marshalAnswersSnapshot(map[string]any{}); string(got) != "{}" {
		t.Fatalf("空 map 应归一为 {}, got %s", got)
	}
	got := marshalAnswersSnapshot(map[string]any{"1": "A"})
	if string(got) != `{"1":"A"}` {
		t.Fatalf("有内容应原样保留, got %s", got)
	}
}

// TestSaveProgressNilAnswersWritesEmptyObject 保存路径 nil 答案不得写 JSONB null。
func TestSaveProgressNilAnswersWritesEmptyObject(t *testing.T) {
	svc, db, mockExamID, studentID := seedMockInProgress(t)

	if err := svc.SaveProgress(mockExamID, studentID, nil, 60); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	var got model.MockExam
	if err := db.First(&got, mockExamID).Error; err != nil {
		t.Fatal(err)
	}
	if string(got.Answers) == "null" || len(got.Answers) == 0 {
		t.Fatalf("nil 答案应落库为 {}，禁止 null/空, got %q", got.Answers)
	}
	var m map[string]any
	if err := json.Unmarshal(got.Answers, &m); err != nil {
		t.Fatalf("落库答案应可解析为对象: %v", err)
	}
}

// TestMockSaveProgressNotFound 契约零漂移：不存在的模拟考返回「模拟考试不存在」。
func TestMockSaveProgressNotFound(t *testing.T) {
	svc, _, _, _ := seedMockInProgress(t)
	err := svc.SaveProgress(9999, 1, map[string]any{}, 0)
	if err == nil || err.Error() != "模拟考试不存在" {
		t.Fatalf("不存在的模拟考应返回「模拟考试不存在」, got %v", err)
	}
}
