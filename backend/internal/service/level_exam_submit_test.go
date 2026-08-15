// Package service 测试：SubmitExam 交卷评分冻结语义（Ticket #216）。
// 交卷即「提交后待阅卷结算」：客观题即时判题回填 ObjectiveScore；简答题待阅卷，
// grader 未赋值前不计入总分；IsPassed 由阅卷端 GradingService.updateParticipantScore
// 在所有答题阅卷完成后置后重算。此处锁定现状行为，重构去除死代码后仍须全绿。
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

// examSubmitFixture 造一场次 + in_progress 参与记录 + 指定题型题目。
// 返回 (svc, db, participantID, studentID, questionIDs)，题目按 questionTypes 顺序建出，
// 测试依真实题目 ID 组装答案快照。
func examSubmitFixture(t *testing.T, questionTypes []string) (*LevelExamService, *gorm.DB, int, int, []int) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	svc := NewLevelExamService(db, nil, zap.NewNop())

	sess := model.ExamSession{
		Name: "定级考", StartTime: time.Now().Add(-time.Hour), EndTime: time.Now().Add(time.Hour),
		Duration: 90, Status: "ongoing", TotalScore: 100, PassScore: 60,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := db.Create(&sess).Error; err != nil {
		t.Fatalf("建场次失败: %v", err)
	}

	var qids []int
	for _, typ := range questionTypes {
		q := testutil.SeedQuestion(t, db, typ, "题干-"+typ, "A")
		qids = append(qids, q.ID)
	}
	idsJSON, _ := json.Marshal(qids)
	// 初始空快照；测试在调用 SubmitExam 前用 setAnswers 回填
	snapJSON, _ := json.Marshal(map[string]any{})

	student := testutil.SeedStudent(t, db, "张三", "x")
	now := time.Now()
	p := model.ExamParticipant{
		ExamSessionID:   sess.ID,
		StudentID:       student.ID,
		Status:          "in_progress",
		StartTime:       &now,
		RemainingTime:   sess.Duration * 60,
		QuestionIDs:     model.JSONB(idsJSON),
		AnswersSnapshot: model.JSONB(snapJSON),
		CreatedAt:       now,
	}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("建参与记录失败: %v", err)
	}
	return svc, db, p.ID, student.ID, qids
}

// setAnswers 覆写参与记录答案快照（键为题目 ID 字符串）。
func setAnswers(t *testing.T, db *gorm.DB, participantID int, answers map[string]any) {
	t.Helper()
	b, _ := json.Marshal(answers)
	var p model.ExamParticipant
	if err := db.First(&p, participantID).Error; err != nil {
		t.Fatal(err)
	}
	p.AnswersSnapshot = model.JSONB(b)
	if err := db.Save(&p).Error; err != nil {
		t.Fatal(err)
	}
}

// assertScoreNil 断言 Score 指针为 nil（待阅卷未结算）。
func assertScoreNil(t *testing.T, score *float64) {
	t.Helper()
	if score != nil {
		t.Fatalf("Score 应为 nil（待阅卷结算延后）, got %v", *score)
	}
}

// assertFloatPtrEq 断言浮点指针值相等。
func assertFloatPtrEq(t *testing.T, got *float64, want float64, field string) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s 不应为 nil", field)
	}
	if *got != want {
		t.Fatalf("%s = %v, want %v", field, *got, want)
	}
}

// TestSubmitExamShortAnswerDefersSettlement 提交含简答+客观题：SubjectiveScore 恒 0、
// Score 为 nil、IsPassed 不为 true（阅卷前）。结算延后由阅卷端 updateParticipantScore 完成。
func TestSubmitExamShortAnswerDefersSettlement(t *testing.T) {
	svc, db, participantID, studentID, qids := examSubmitFixture(t, []string{"short_answer", "single_choice"})

	// 简答写主观作答、单选答 A（正确，满分 3）
	setAnswers(t, db, participantID, map[string]any{
		intToString(qids[0]): "我的作答",
		intToString(qids[1]): "A",
	})

	dto, err := svc.SubmitExam(participantID, studentID, false)
	if err != nil {
		t.Fatalf("交卷失败: %v", err)
	}

	// 简答待阅卷：主观分提交时恒为 0（冻结现状）
	assertFloatPtrEq(t, dto.SubjectiveScore, 0, "SubjectiveScore")
	// 存在未阅卷答题（简答 grader 未赋值）→ Score 未结算为 nil
	assertScoreNil(t, dto.Score)
	// 阅卷前 IsPassed 不得为 true
	if dto.IsPassed {
		t.Error("阅卷前 IsPassed 不应为 true")
	}

	// 落库一致：participant 主观分 0、总分 nil、未通过
	var stored model.ExamParticipant
	if err := db.First(&stored, participantID).Error; err != nil {
		t.Fatal(err)
	}
	assertFloatPtrEq(t, stored.SubjectiveScore, 0, "DB SubjectiveScore")
	assertScoreNil(t, stored.Score)
	if stored.IsPassed {
		t.Error("DB IsPassed 阅卷前不应为 true")
	}

	// 简答答题记录存在，grader 未赋值（未阅卷）、得分 0
	var answers []model.ExamAnswer
	if err := db.Where("exam_participant_id = ? AND question_id = ?", participantID, qids[0]).Find(&answers).Error; err != nil {
		t.Fatal(err)
	}
	if len(answers) != 1 {
		t.Fatalf("简答答题记录应恰有 1 条, got %d", len(answers))
	}
	if answers[0].GraderID != nil {
		t.Error("简答 grader 未阅卷时应为 nil")
	}
	if answers[0].Score != 0 {
		t.Fatalf("简答未阅卷时 Score 应恒 0, got %v", answers[0].Score)
	}
}

// TestSubmitExamObjectiveStillDeferred 仅客观题提交：虽被 gradeQuestion 即时判题，
// 但答题记录的 grader 未赋值仍计未阅卷 → Score 为 nil、IsPassed false（冻结现状语义）。
func TestSubmitExamObjectiveStillDeferred(t *testing.T) {
	svc, db, participantID, studentID, qids := examSubmitFixture(t, []string{"single_choice", "single_choice"})

	setAnswers(t, db, participantID, map[string]any{
		intToString(qids[0]): "A",
		intToString(qids[1]): "A",
	})

	dto, err := svc.SubmitExam(participantID, studentID, false)
	if err != nil {
		t.Fatalf("交卷失败: %v", err)
	}

	// 客观题即时判题：满分 3 × 2 = 6 回填 ObjectiveScore
	assertFloatPtrEq(t, dto.ObjectiveScore, 6, "ObjectiveScore")
	// 客观题未人工阅卷（grader nil）→ 仍有未阅卷答题 → 总分不结算
	assertScoreNil(t, dto.Score)
	if dto.IsPassed {
		t.Error("存在未阅卷答题时 IsPassed 不应为 true")
	}
}
