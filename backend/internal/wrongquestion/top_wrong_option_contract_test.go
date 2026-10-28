// 错题易错项契约（#2068）：top_wrong_option = 该题历史上最多人选错的选项。
//
// 口径（与实现注释同一句）：分母 = 该题全部作答次数；分子 = 按答案分组的 is_correct=false 次数，
// 空答案不计；正确项不进候选（判错行的答案本来就不是正确答）。并列按答案字典序升序取第一条；
// 该题作答总数 < 5（topWrongOptionMinSample）时字段缺省——缺省不是空串。
package wrongquestion

import (
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestWrongQuestionTopWrongOptionContract(t *testing.T) {
	t.Parallel()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "top_wrong_stu", "x")
	now := testutil.Now()

	// q1：7 次作答，B×4 / C×2 / 正确 A×1 ⇒ 易错项 B
	q1 := testutil.SeedQuestion(t, db, "single_choice", "易错题一", "A")
	// q2：样本不足（2 次） ⇒ 字段缺省
	q2 := testutil.SeedQuestion(t, db, "single_choice", "样本不足题", "A")
	// q3：并列（X×3 / Y×3，无正确） ⇒ 字典序升序取 X
	q3 := testutil.SeedQuestion(t, db, "single_choice", "并列题", "Z")

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
		{StudentID: student.ID, QuestionID: q3.ID, IsCorrect: false, UserAnswer: "X", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q3.ID, IsCorrect: false, UserAnswer: "X", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q3.ID, IsCorrect: false, UserAnswer: "X", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q3.ID, IsCorrect: false, UserAnswer: "Y", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q3.ID, IsCorrect: false, UserAnswer: "Y", CreatedAt: now},
		{StudentID: student.ID, QuestionID: q3.ID, IsCorrect: false, UserAnswer: "Y", CreatedAt: now},
	}
	for i := range records {
		if err := db.Create(&records[i]).Error; err != nil {
			t.Fatalf("建作答记录失败: %v", err)
		}
	}
	for _, q := range []*model.Question{q1, q2, q3} {
		wq := model.WrongQuestion{
			StudentID: student.ID, QuestionID: q.ID, WrongCount: 1,
			LastWrongAt: now, CreatedAt: now,
		}
		if err := db.Create(&wq).Error; err != nil {
			t.Fatalf("建错题失败: %v", err)
		}
	}

	svc := NewService(db, nil, nil)
	page, err := svc.GetWrongQuestions(student.ID, 1, 20, "", nil, false, "", nil)
	if err != nil {
		t.Fatalf("GetWrongQuestions 失败: %v", err)
	}
	got := map[int]*string{}
	for i := range page.Items {
		got[page.Items[i].QuestionID] = page.Items[i].TopWrongOption
	}
	if v := got[q1.ID]; v == nil || *v != "B" {
		t.Fatalf("q1 易错项应为 B（正确项 A 不进候选）, got %v", v)
	}
	if v := got[q2.ID]; v != nil {
		t.Fatalf("q2 样本不足应缺省（不出空串）, got %v", v)
	}
	if v := got[q3.ID]; v == nil || *v != "X" {
		t.Fatalf("q3 并列应按字典序取 X, got %v", v)
	}
}
