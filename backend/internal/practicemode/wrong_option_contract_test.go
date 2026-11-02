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
package practicemode

import (
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestCommonWrongOptionContract(t *testing.T) {
	t.Parallel()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "common_wrong_stu", "x")
	now := testutil.Now()

	// q1：7 次作答，B×4 / C×2 / 正确 A×1 ⇒ 易错项 B（正确项不进候选）
	q1 := testutil.SeedQuestion(t, db, "single_choice", "易错题一", "A")
	// q2：判错仅 2 次 ⇒ 缺省
	q2 := testutil.SeedQuestion(t, db, "single_choice", "样本不足题", "A")
	// q3：并列（X×3 / Y×3，无正确作答）⇒ 字典序升序取 X
	q3 := testutil.SeedQuestion(t, db, "single_choice", "并列题", "Z")
	// q4：简答题（自由文本，没有「选项」）⇒ 绝不返回易错项
	q4 := testutil.SeedQuestion(t, db, "short_answer", "简答题", "参考答案")
	// q5：判错 4 次 + 做对 10 次 = 总 14 ⇒ 正确率在、易错项缺省（两门槛不同源的关键用例）
	q5 := testutil.SeedQuestion(t, db, "single_choice", "多数人做对的题", "A")
	// q6：空答案 5 次（空提交不是「选错某个选项」）⇒ 不进候选 ⇒ 缺省
	q6 := testutil.SeedQuestion(t, db, "single_choice", "空答案题", "A")

	records := []model.QuestionPracticeRecord{
		{StudentID: student.ID, QuestionID: q1.ID, IsCorrect: false, UserAnswer: "B", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q1.ID, IsCorrect: false, UserAnswer: "B", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q1.ID, IsCorrect: false, UserAnswer: "B", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q1.ID, IsCorrect: false, UserAnswer: "B", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q1.ID, IsCorrect: false, UserAnswer: "C", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q1.ID, IsCorrect: false, UserAnswer: "C", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q1.ID, IsCorrect: true, UserAnswer: "A", CreatedAt: now},

		{StudentID: student.ID, QuestionID: q2.ID, IsCorrect: false, UserAnswer: "B", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q2.ID, IsCorrect: false, UserAnswer: "C", CreatedAt: now},

		{StudentID: student.ID, QuestionID: q3.ID, IsCorrect: false, UserAnswer: "Y", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q3.ID, IsCorrect: false, UserAnswer: "Y", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q3.ID, IsCorrect: false, UserAnswer: "Y", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q3.ID, IsCorrect: false, UserAnswer: "X", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q3.ID, IsCorrect: false, UserAnswer: "X", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q3.ID, IsCorrect: false, UserAnswer: "X", CreatedAt: now},

		{StudentID: student.ID, QuestionID: q4.ID, IsCorrect: false, UserAnswer: "学员甲的自由作答", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q4.ID, IsCorrect: false, UserAnswer: "学员甲的自由作答", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q4.ID, IsCorrect: false, UserAnswer: "学员甲的自由作答", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q4.ID, IsCorrect: false, UserAnswer: "学员甲的自由作答", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q4.ID, IsCorrect: false, UserAnswer: "学员甲的自由作答", CreatedAt: now},

		{StudentID: student.ID, QuestionID: q5.ID, IsCorrect: false, UserAnswer: "B", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q5.ID, IsCorrect: false, UserAnswer: "B", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q5.ID, IsCorrect: false, UserAnswer: "B", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q5.ID, IsCorrect: false, UserAnswer: "B", CreatedAt: now},

		{StudentID: student.ID, QuestionID: q6.ID, IsCorrect: false, UserAnswer: "", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q6.ID, IsCorrect: false, UserAnswer: "", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q6.ID, IsCorrect: false, UserAnswer: "", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q6.ID, IsCorrect: false, UserAnswer: "", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q6.ID, IsCorrect: false, UserAnswer: "", CreatedAt: now},
	}
	// q5 补 10 次做对（凑到总 14、判错 4）
	for i := 0; i < 10; i++ {
		records = append(records, model.QuestionPracticeRecord{
			StudentID: student.ID, QuestionID: q5.ID, IsCorrect: true, UserAnswer: "A", CreatedAt: now,
		})
	}
	for i := range records {
		if err := db.Create(&records[i]).Error; err != nil {
			t.Fatalf("建作答记录失败: %v", err)
		}
	}

	cases := []struct {
		name    string
		q       *model.Question
		want    *string
		wantAcc bool
	}{
		{"q1 众数", q1, strPtr("B"), true},
		{"q2 判错不足", q2, nil, false},
		{"q3 并列取字典序", q3, strPtr("X"), true},
		{"q4 简答题", q4, nil, true},
		{"q5 判错 4 / 总 14", q5, nil, true},
		{"q6 空答案不进候选", q6, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := questionStats(db, tc.q.ID)
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

// TestCommonWrongMatchesTopWrongOption 两出口逐字节相等——本票的核心断言。
func TestCommonWrongMatchesTopWrongOption(t *testing.T) {
	t.Parallel()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "cross_match_stu", "x")
	now := testutil.Now()

	// 覆盖：有值 / 并列 / 判错不足 / 简答 / 空答案 / 判错够但总作答也够
	qs := []*model.Question{
		testutil.SeedQuestion(t, db, "single_choice", "有值", "A"),
		testutil.SeedQuestion(t, db, "single_choice", "并列", "Z"),
		testutil.SeedQuestion(t, db, "single_choice", "判错不足", "A"),
		testutil.SeedQuestion(t, db, "short_answer", "简答", "参考答案"),
		testutil.SeedQuestion(t, db, "multi_choice", "多选", "AB"),
	}
	var records []model.QuestionPracticeRecord
	add := func(qID int, isCorrect bool, ans string, n int) {
		for i := 0; i < n; i++ {
			records = append(records, model.QuestionPracticeRecord{
				StudentID: student.ID, QuestionID: qID, IsCorrect: isCorrect, UserAnswer: ans, CreatedAt: now,
			})
		}
	}
	add(qs[0].ID, false, "B", 4)
	add(qs[0].ID, false, "C", 2)
	add(qs[0].ID, true, "A", 1)
	add(qs[1].ID, false, "Y", 3)
	add(qs[1].ID, false, "X", 3)
	add(qs[2].ID, false, "B", 2)
	add(qs[3].ID, false, "自由作答", 6)
	add(qs[4].ID, false, "AB", 3)
	add(qs[4].ID, false, "CD", 3)
	for i := range records {
		if err := db.Create(&records[i]).Error; err != nil {
			t.Fatalf("建作答记录失败: %v", err)
		}
	}

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

// TestCommonWrongImpliesAccuracyRate 不变式：有 common_wrong ⇒ 必有 accuracy_rate。
// 反向不成立（q5 那类），所以只断言这一个方向——它是「判错数 ≤ 总作答数」的直接推论，
// 实现若把两个门槛接反（易错项按总数、正确率按判错数）会在这里被抓住。
func TestCommonWrongImpliesAccuracyRate(t *testing.T) {
	t.Parallel()
	db := testutil.NewMemoryDB(t)
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

func strPtr(s string) *string { return &s }

func fmtPtr[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}
