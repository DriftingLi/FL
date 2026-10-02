// 练习域的 nonnil 行为例（ADR-0065 决策 5 判据 5）：跑真实出口、看发出的是 [] 还是 null。
//
// 为什么证据跟着域包走（ADR-0070）：判据 5 的表是「哪些域声明 nonnil、谁举证」的对应关系，
// 域包拆出去之后域的实现与它的举证住在同一个包里，改名/搬目录不会让两侧各自漂。
// 断言本体只有一份：testutil.AssertNonNilOutlets。
//
// 这三格是波 3c-2 从 internal/service 的 nonnilOutletsCatalog 表搬来的（生产者
// StartSequential / GetHistory / GetStats 的方法本体就在本包），键前缀随包名改成 practicemode。
package practicemode

import (
	"testing"

	"forklift-training/internal/testutil"
)

// nonnilOutletsPractice 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
// 前缀 nonnilOutlets 是约定的证据表名：apitypes 的表态锁按它扫目录收键（见
// internal/apitypes/nullability_lock_test.go 的 nonNilEvidenceSources）。
var nonnilOutletsPractice = map[string]func(t *testing.T) any{
	"practicemode.HistoryResultDTO.records":         outletPracticeHistoryEmpty,
	"practicemode.PracticeStartResultDTO.questions": outletPracticeStartSequential,
	"practicemode.PracticeStatsDTO.by_type":         outletPracticeStatsEmpty,
}

// TestNonNilDeclaredOutletsNeverEmitNull 本域的举证入口（判据本体在 testutil）。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsPractice})
}

// ===== 练习三格 =====

// outletPracticeHistoryEmpty 空练习历史：records 由 make([]HistoryItemDTO, 0, n) 起手 ⇒ [ ]。
func outletPracticeHistoryEmpty(t *testing.T) any {
	t.Helper()
	svc, db := newPracticeSvc(t)
	res, err := svc.GetHistory(testutil.SeedStudent(t, db, "练习历史学员", "x").ID, nil, 1, 20, "", "", "")
	if err != nil {
		t.Fatalf("空练习历史失败: %v", err)
	}
	return res
}

// outletPracticeStartSequential 顺序练习开考：池里 0 题直接走 error，所以这条出口最少发 1 题。
// 恒非 null 的判据不靠「凑得出空集」——questions 由 make(0,len(questions)) 起手，
// 而那段长度在 `len(questions)==0 ⇒ error` 守卫之后，两件事各自成立。
func outletPracticeStartSequential(t *testing.T) any {
	t.Helper()
	svc, db := newPracticeSvc(t)
	student := testutil.SeedStudent(t, db, "顺序练习学员", "x")
	testutil.SeedQuestion(t, db, "single", "空池守卫前的第一题", "A")
	res, err := svc.StartSequential(student.ID, nil)
	if err != nil {
		t.Fatalf("顺序练习开考失败: %v", err)
	}
	return res
}

// outletPracticeStatsEmpty 一次练习都没有：by_type 取自 GroupByCountWithFilter 的 make(map,...)
// 再按合法维度零填充 ⇒ 恒 `{}` 起、只会更满。
func outletPracticeStatsEmpty(t *testing.T) any {
	t.Helper()
	svc, db := newPracticeSvc(t)
	student := testutil.SeedStudent(t, db, "练习统计学员", "x")
	res, err := svc.GetStats(student.ID, nil)
	if err != nil {
		t.Fatalf("练习统计失败: %v", err)
	}
	return res
}
