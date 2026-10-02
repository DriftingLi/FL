// 模拟考域的 nonnil 行为例（ADR-0065 决策 5 判据 5）：跑真实出口、看发出的是 [] 还是 null。
//
// 为什么证据跟着域包走（ADR-0070）：判据 5 的表是「哪些域声明 nonnil、谁举证」的对应关系，
// 域包拆出去之后域的实现与它的举证住在同一个包里，改名/搬目录不会让两侧各自漂。
// 断言本体只有一份：testutil.AssertNonNilOutlets。
//
// 这三格是波 4a 从 internal/service 的 nonnilOutletsCatalog / nonnilOutletsStats 两张表搬来的
// （生产者 Start / Resume / GetHistory 的方法本体就在本包），键前缀随包名改成 mockexam。
package mockexam

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// nonnilOutletsExam 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
// 前缀 nonnilOutlets 是约定的证据表名：apitypes 的表态锁按它扫目录收键（见
// internal/apitypes/nullability_lock_test.go 的 nonNilEvidenceSources）。
var nonnilOutletsExam = map[string]func(t *testing.T) any{
	"mockexam.MockExamHistoryDTO.exams":    outletMockExamHistoryEmpty,
	"mockexam.MockExamStartDTO.questions":  outletMockExamStart,
	"mockexam.MockExamResumeDTO.questions": outletMockExamResume,
}

// TestNonNilDeclaredOutletsNeverEmitNull 本域的举证入口（判据本体在 testutil）。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsExam})
}

// ===== 模拟考三格 =====

// outletMockExamHistoryEmpty 空模考历史：exams 由 make([]MockExamHistoryItemDTO, 0, n) 起手 ⇒ [ ]。
func outletMockExamHistoryEmpty(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "模考历史学员", "x")
	res, err := NewService(db, nil, zap.NewNop()).GetHistory(student.ID, nil, 1, 20)
	if err != nil {
		t.Fatalf("空模考历史失败: %v", err)
	}
	return res
}

// outletMockExamStart 开考抽题：questions 由 make(0, len(questions)) 起手。
func outletMockExamStart(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "模考开科学员", "x")
	testutil.SeedQuestion(t, db, "single", "模考抽题源", "A")
	res, err := NewService(db, nil, zap.NewNop()).Start(student.ID, 1, 90, nil)
	if err != nil {
		t.Fatalf("模拟考试开考失败: %v", err)
	}
	return res
}

// outletMockExamResume 断点续考：卷面存的题号指向**已被删除的题目**时 practicemode.LoadOrderedQuestions
// 取不到任何一行，questions 走 make(0,0) 而不是 nil —— 这条正是「题被下架后学员还在考试中途」的线上形状。
func outletMockExamResume(t *testing.T) any {
	t.Helper()
	svc, db, mockExamID, studentID := seedMockInProgress(t)
	if err := db.Model(&model.MockExam{}).Where("id = ?", mockExamID).
		Update("question_ids", model.JSONB("[999999]")).Error; err != nil {
		t.Fatalf("改写考卷题号失败: %v", err)
	}
	res, err := svc.Resume(mockExamID, studentID)
	if err != nil {
		t.Fatalf("续考失败: %v", err)
	}
	return res
}
