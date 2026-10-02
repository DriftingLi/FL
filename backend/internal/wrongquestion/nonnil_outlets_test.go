// 错题本域的 nonnil 行为例（ADR-0065 决策 5 判据 5）：跑真实出口、看发出的是 [] 还是 null。
//
// 为什么证据跟着域包走（ADR-0070）：判据 5 的表是「哪些域声明 nonnil、谁举证」的对应关系，
// 域包拆出去之后域的实现与它的举证住在同一个包里，改名 / 搬目录不会让两侧各自漂。
// 断言本体只有一份：testutil.AssertNonNilOutlets。
//
// 这两格是波 4c 从 internal/service 的 nonnilOutletsStats / nonnilOutletsCatalog 两张表搬来的
// （生产者 GetWrongQuestions / GetStats 的方法体就在本包），键前缀随包名改成 wrongquestion。
package wrongquestion

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/testutil"
)

// nonnilOutletsWrongQuestion 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
// 前缀 nonnilOutlets 是约定的证据表名：apitypes 的表态锁按它扫目录收键（见
// internal/apitypes/nullability_lock_test.go 的 nonNilEvidenceSources）。
var nonnilOutletsWrongQuestion = map[string]func(t *testing.T) any{
	"wrongquestion.WrongQuestionPageDTO.items":    outletWrongQuestionPageEmpty,
	"wrongquestion.WrongQuestionStatsDTO.by_type": outletWrongQuestionStatsEmpty,
}

// TestNonNilDeclaredOutletsNeverEmitNull 本域的举证入口（判据本体在 testutil）。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsWrongQuestion})
}

// outletWrongQuestionPageEmpty 错题本分页：一行错题都没有时 items 仍是 make 出来的空集。
func outletWrongQuestionPageEmpty(t *testing.T) any {
	t.Helper()
	res, err := NewService(testutil.NewMemoryDB(t), nil, zap.NewNop()).
		GetWrongQuestions(1, 1, 20, "", nil, false, "", nil)
	if err != nil {
		t.Fatalf("空错题本分页失败: %v", err)
	}
	return res
}

// outletWrongQuestionStatsEmpty 错题统计：by_type 由 make(map, ...) 起手再按合法维度零填充。
func outletWrongQuestionStatsEmpty(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "错题统计学员", "x")
	return NewService(db, nil, zap.NewNop()).GetStats(student.ID)
}
