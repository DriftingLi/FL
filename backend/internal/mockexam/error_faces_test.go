// 模考域的**事实→错误**档位锁（ADR-0064 决策 8）：行不在才叫「不存在」，查不动如实上抛。
// 由 internal/service/practice_paper_face_test.go 按接缝拆来（波 4a，随域包搬走）。
package mockexam

import (
	"errors"
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/testutil"
)

// TestMockExamNotFoundIsOnlyForMissingRows 模考三处读路径：行不在才叫「不存在」，查不动如实上抛。
func TestMockExamNotFoundIsOnlyForMissingRows(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewService(db, nil, zap.NewNop())
	if _, err := svc.GetResult(999999, 1); !errors.Is(err, ErrMockExamNotFound) {
		t.Fatalf("不存在的模考应报具名哨兵，实际 %v", err)
	}
	if err := db.Exec("DROP TABLE mock_exam").Error; err != nil {
		t.Fatalf("注入故障失败: %v", err)
	}
	_, err := svc.GetResult(1, 1)
	if errors.Is(err, ErrMockExamNotFound) {
		t.Fatalf("表都读不到却报「模拟考试不存在」: %v", err)
	}
	if err == nil {
		t.Fatal("查不动返回了成功")
	}
}
