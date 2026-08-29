// Package service 测试：模拟考试历史列表口径（Ticket: 历史记录出现"没考过却有记录"）。
// 锁定两条行为：
//  1. GetHistory 只返回已交卷（submitted）记录 —— 点过「开始」但没交卷的废弃尝试不展示；
//  2. Start 开新考试时清理该学生超过 mockExamAbandonTTL 仍未交卷的旧记录 —— 防止表无限堆积。
package service

import (
	"testing"
	"time"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// TestGetHistoryOnlySubmitted 历史只含已交卷记录：
// 学生点了「开始考试」但没交卷（status=in_progress）不应出现在历史中。
func TestGetHistoryOnlySubmitted(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewMockExamService(db, nil, zap.NewNop())
	student := testutil.SeedStudent(t, db, "李四", "x")

	now := time.Now()
	// 废弃尝试：点过开始、没交卷
	abandoned := model.MockExam{
		StudentID: student.ID,
		Status:    mockExamStatusInProgress,
		StartTime: &now,
		CreatedAt: now,
	}
	// 已交卷：应出现在历史中
	submitAt := now.Add(time.Hour)
	done := model.MockExam{
		StudentID:  student.ID,
		Status:     mockExamStatusSubmitted,
		StartTime:  &now,
		SubmitTime: &submitAt,
		CreatedAt:  now,
	}
	for _, m := range []model.MockExam{abandoned, done} {
		if err := db.Create(&m).Error; err != nil {
			t.Fatalf("插入模拟考试失败: %v", err)
		}
	}

	got := svc.GetHistory(student.ID, 1, 10)
	if got.Total != 1 {
		t.Fatalf("历史应只含 1 条已交卷记录, got total=%d", got.Total)
	}
	if len(got.Exams) != 1 {
		t.Fatalf("历史条目数应为 1, got %d", len(got.Exams))
	}
	if got.Exams[0].Status != mockExamStatusSubmitted {
		t.Fatalf("历史条目状态应为 submitted, got %s", got.Exams[0].Status)
	}
}

// TestStartCleansAbandonedExams 开新考试时清理超时废弃记录：
// 超过 mockExamAbandonTTL 的未交卷记录被清理，未超期的进行中记录保留（可断点续考）。
func TestStartCleansAbandonedExams(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewMockExamService(db, nil, zap.NewNop())
	student := testutil.SeedStudent(t, db, "王五", "x")
	// Start 需要题库非空，否则提前返回错误、清理逻辑不执行
	testutil.SeedQuestion(t, db, "single", "1+1=?", "A")

	now := time.Now()
	// 超期废弃：48 小时前开始，从未交卷
	stale := model.MockExam{
		StudentID: student.ID,
		Status:    mockExamStatusInProgress,
		StartTime: &now,
		CreatedAt: now,
	}
	// 近期进行中：1 小时前开始，仍在续考窗口内
	fresh := model.MockExam{
		StudentID: student.ID,
		Status:    mockExamStatusInProgress,
		StartTime: &now,
		CreatedAt: now,
	}
	if err := db.Create(&stale).Error; err != nil {
		t.Fatalf("插入超期记录失败: %v", err)
	}
	if err := db.Create(&fresh).Error; err != nil {
		t.Fatalf("插入近期记录失败: %v", err)
	}
	// CreatedAt 由 GORM 自动维护，Create 时无法覆盖，这里显式回写为历史时间
	if err := db.Model(&stale).Update("created_at", now.Add(-2*mockExamAbandonTTL)).Error; err != nil {
		t.Fatalf("回写 created_at 失败: %v", err)
	}
	if err := db.Model(&fresh).Update("created_at", now.Add(-time.Hour)).Error; err != nil {
		t.Fatalf("回写 created_at 失败: %v", err)
	}

	if _, err := svc.Start(student.ID, 1, 90); err != nil {
		t.Fatalf("开始考试失败: %v", err)
	}

	var staleGot model.MockExam
	if err := db.First(&staleGot, stale.ID).Error; err == nil {
		t.Fatalf("超期 %v 的未交卷记录应被清理, 但仍存在", mockExamAbandonTTL)
	}
	var freshGot model.MockExam
	if err := db.First(&freshGot, fresh.ID).Error; err != nil {
		t.Fatalf("未超期的进行中记录应保留（可续考）, 查询失败: %v", err)
	}
	if freshGot.Status != mockExamStatusInProgress {
		t.Fatalf("保留的记录状态应仍为 in_progress, got %s", freshGot.Status)
	}
}

// TestStartKeepsOtherStudentsAbandoned 清理只作用于本人：他人的废弃记录不受影响。
func TestStartKeepsOtherStudentsAbandoned(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewMockExamService(db, nil, zap.NewNop())
	me := testutil.SeedStudent(t, db, "本人", "x")
	other := testutil.SeedStudent(t, db, "他人", "y")
	testutil.SeedQuestion(t, db, "single", "1+1=?", "A")

	now := time.Now()
	otherStale := model.MockExam{
		StudentID: other.ID,
		Status:    mockExamStatusInProgress,
		StartTime: &now,
		CreatedAt: now,
	}
	if err := db.Create(&otherStale).Error; err != nil {
		t.Fatalf("插入他人记录失败: %v", err)
	}
	if err := db.Model(&otherStale).Update("created_at", now.Add(-2*mockExamAbandonTTL)).Error; err != nil {
		t.Fatalf("回写 created_at 失败: %v", err)
	}

	if _, err := svc.Start(me.ID, 1, 90); err != nil {
		t.Fatalf("开始考试失败: %v", err)
	}

	var got model.MockExam
	if err := db.First(&got, otherStale.ID).Error; err != nil {
		t.Fatalf("他人的废弃记录不应被本人开考清理, 查询失败: %v", err)
	}
}
