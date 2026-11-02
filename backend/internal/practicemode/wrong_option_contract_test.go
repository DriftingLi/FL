// 易错项口径对齐契约（#2155）：同一道题的两个出口必须同值。
//
// 出口 ①：错题本列表 top_wrong_option（wrongquestion.loadTopWrongOptions）
// 出口 ②：提交结果卡 common_wrong（practicemode.questionStats → CommonWrongOption）
//
// 本文件锁四件事：
//  1. 两出口逐字节相等（含并列这种「同一份数据可产出多个合法答案」的情形）；
//  2. 空答案不进候选；
//  3. 判错样本 < 5 时缺省（不是空串、不是 0），且此时若总作答 ≥ 5 正确率仍在——
//     「有 accuracy_rate 无 common_wrong」是合法态，反向不可能（判错数 ≤ 总作答数，当不变式断言）；
//  4. 简答题恒不返回易错项。
//
// 双引擎：断言只写一份，SQLite 与 Postgres 各跑一遍（先例 internal/api/practice_pool_contract_test.go）。
// PG 那腿是本票 AC 明确要求的——易错项是纯 SQL 聚合（GROUP BY + JOIN + 二级排序），
// 两方言对空串/NULL 与排序的处理并不完全一致，只在 SQLite 上绿不足以证明口径对齐。
// 本机无 DATABASE_URL 时 PG 腿干净 skip（看到 ok ≠ 测过，首跑在 CI）。
package practicemode

import (
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// seedWrongOptionFixture 共用的六题样本，返回 题目 + 已落库。
//
// q1 众数（正确项不进候选）；q2 判错不足；q3 并列取字典序；q4 简答题；q5 判错 4 / 总 14
// （两门槛不同源的关键用例）；q6 空答案不进候选。
func seedWrongOptionFixture(t *testing.T, db *gorm.DB, prefix string) map[string]*model.Question {
	t.Helper()
	student := testutil.SeedStudent(t, db, prefix+"_stu", "x")
	now := testutil.Now()

	q := map[string]*model.Question{
		"q1": testutil.SeedQuestion(t, db, "single_choice", "易错题一", "A"),
		"q2": testutil.SeedQuestion(t, db, "single_choice", "样本不足题", "A"),
		"q3": testutil.SeedQuestion(t, db, "single_choice", "并列题", "Z"),
		"q4": testutil.SeedQuestion(t, db, "short_answer", "简答题", "参考答案"),
		"q5": testutil.SeedQuestion(t, db, "single_choice", "多数人做对的题", "A"),
		"q6": testutil.SeedQuestion(t, db, "single_choice", "空答案题", "A"),
	}
	seed := func(key string, isCorrect bool, ans string, n int) {
		for i := 0; i < n; i++ {
			rec := model.QuestionPracticeRecord{
				StudentID: student.ID, QuestionID: q[key].ID,
				IsCorrect: isCorrect, UserAnswer: ans, CreatedAt: now,
			}
			if err := db.Create(&rec).Error; err != nil {
				t.Fatalf("建作答记录失败（%s）: %v", key, err)
			}
		}
	}
	seed("q1", false, "B", 4)
	seed("q1", false, "C", 2)
	seed("q1", true, "A", 1)
	seed("q2", false, "B", 1)
	seed("q2", false, "C", 1)
	seed("q3", false, "Y", 3)
	seed("q3", false, "X", 3)
	seed("q4", false, "学员甲的自由作答", 5)
	seed("q5", false, "B", 4)
	seed("q5", true, "A", 10)
	seed("q6", false, "", 5)
	return q
}

// assertCommonWrongOption 易错项单题口径。
func assertCommonWrongOption(t *testing.T, db *gorm.DB) {
	q := seedWrongOptionFixture(t, db, "common_wrong")

	cases := []struct {
		name    string
		key     string
		want    *string
		wantAcc bool
	}{
		{"q1 众数", "q1", strPtr("B"), true},
		{"q2 判错不足", "q2", nil, false},
		{"q3 并列取字典序", "q3", strPtr("X"), true},
		{"q4 简答题", "q4", nil, true},
		{"q5 判错4/总14", "q5", nil, true},
		{"q6 空答案不进候选", "q6", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := questionStats(db, q[tc.key].ID)
			if got == nil {
				t.Fatalf("questionStats 返回 nil")
			}
			if tc.want == nil {
				if got.commonWrong != nil {
					t.Fatalf("易错项应缺省（不是空串），got %q", *got.commonWrong)
				}
			} else if got.commonWrong == nil || *got.commonWrong != *tc.want {
				t.Fatalf("易错项应为 %q, got %v", *tc.want, fmtPtr(got.commonWrong))
			}
			if (got.accuracyRate != nil) != tc.wantAcc {
				t.Fatalf("正确率存在性应为 %v, got %v", tc.wantAcc, fmtPtr(got.accuracyRate))
			}
		})
	}
}

// assertCommonWrongMatchesTopWrongOption 两出口逐字节相等——本票的核心断言。
func assertCommonWrongMatchesTopWrongOption(t *testing.T, db *gorm.DB) {
	student := testutil.SeedStudent(t, db, "cross_match_stu", "x")
	now := testutil.Now()

	// 覆盖：有值 / 并列 / 判错不足 / 简答 / 多选组合
	qs := []*model.Question{
		testutil.SeedQuestion(t, db, "single_choice", "有值", "A"),
		testutil.SeedQuestion(t, db, "single_choice", "并列", "Z"),
		testutil.SeedQuestion(t, db, "single_choice", "判错不足", "A"),
		testutil.SeedQuestion(t, db, "short_answer", "简答", "参考答案"),
		testutil.SeedQuestion(t, db, "multi_choice", "多选", "AB"),
	}
	add := func(qID int, isCorrect bool, ans string, n int) {
		for i := 0; i < n; i++ {
			rec := model.QuestionPracticeRecord{
				StudentID: student.ID, QuestionID: qID, IsCorrect: isCorrect, UserAnswer: ans, CreatedAt: now,
			}
			if err := db.Create(&rec).Error; err != nil {
				t.Fatalf("建作答记录失败: %v", err)
			}
		}
	}
	add(qs[0].ID, false, "B", 4)
	add(qs[0].ID, true, "A", 1)
	add(qs[1].ID, false, "Y", 3)
	add(qs[1].ID, false, "X", 3)
	add(qs[2].ID, false, "B", 2)
	add(qs[3].ID, false, "自由作答", 6)
	add(qs[4].ID, false, "AB", 3)
	add(qs[4].ID, false, "CD", 3)

	ids := make([]int, 0, len(qs))
	for _, q := range qs {
		ids = append(ids, q.ID)
	}
	batch := LoadTopWrongOptions(db, ids)
	for _, q := range qs {
		single := CommonWrongOption(db, q.ID)
		batchVal, hasBatch := batch[q.ID]
		if (single != nil) != hasBatch {
			t.Fatalf("q%d 两出口存在性不一致：单选=%v 批量=%v", q.ID, fmtPtr(single), hasBatch)
		}
		if single != nil && *single != batchVal {
			t.Fatalf("q%d 两出口值不一致：单选=%q 批量=%q", q.ID, *single, batchVal)
		}
		if stats := questionStats(db, q.ID); (stats.commonWrong != nil) != (single != nil) {
			t.Fatalf("q%d questionStats 与 CommonWrongOption 不一致", q.ID)
		}
	}
}

// assertCommonWrongImpliesAccuracyRate 不变式：有 common_wrong ⇒ 必有 accuracy_rate。
// 反向不成立（q5 那类），所以只断言这一个方向——它是「判错数 ≤ 总作答数」的直接推论，
// 实现若把两个门槛接反（易错项按总数、正确率按判错数）会在这里被抓住。
func assertCommonWrongImpliesAccuracyRate(t *testing.T, db *gorm.DB) {
	student := testutil.SeedStudent(t, db, "invariant_stu", "x")
	now := testutil.Now()

	q := testutil.SeedQuestion(t, db, "single_choice", "不变式题", "A")
	for i := 0; i < 5; i++ {
		rec := model.QuestionPracticeRecord{
			StudentID: student.ID, QuestionID: q.ID, IsCorrect: false, UserAnswer: "B", CreatedAt: now,
		}
		if err := db.Create(&rec).Error; err != nil {
			t.Fatalf("建作答记录失败: %v", err)
		}
	}
	got := questionStats(db, q.ID)
	if got.commonWrong == nil {
		t.Fatalf("判错 5 次应产出易错项")
	}
	if got.accuracyRate == nil {
		t.Fatalf("有易错项却没有正确率：两个门槛接反了（判错数 ≤ 总作答数，不可能出现这种状态）")
	}
}

// —— SQLite 引擎 ——

func TestCommonWrongOptionContract(t *testing.T) {
	t.Parallel()
	assertCommonWrongOption(t, testutil.NewMemoryDB(t))
}

func TestCommonWrongMatchesTopWrongOption(t *testing.T) {
	t.Parallel()
	assertCommonWrongMatchesTopWrongOption(t, testutil.NewMemoryDB(t))
}

func TestCommonWrongImpliesAccuracyRate(t *testing.T) {
	t.Parallel()
	assertCommonWrongImpliesAccuracyRate(t, testutil.NewMemoryDB(t))
}

// —— Postgres 引擎（AC 要求；本机无 DATABASE_URL 时干净 skip，首跑在 CI）——

func TestCommonWrongOptionContractOnPostgres(t *testing.T) {
	assertCommonWrongOption(t, testutil.NewPostgresDB(t))
}

func TestCommonWrongMatchesTopWrongOptionOnPostgres(t *testing.T) {
	assertCommonWrongMatchesTopWrongOption(t, testutil.NewPostgresDB(t))
}

func TestCommonWrongImpliesAccuracyRateOnPostgres(t *testing.T) {
	assertCommonWrongImpliesAccuracyRate(t, testutil.NewPostgresDB(t))
}

func strPtr(s string) *string { return &s }

func fmtPtr[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}
