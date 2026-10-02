// mockexam 包测试：模拟考试总分口径（ADR-0068 决策 1-3）的行为锁：
// total_score 必须等于 Σ details[].score —— 多选半对与简答 AI 分都要进总分。
package mockexam

import (
	"encoding/json"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/aiassistant"
	"forklift-training/internal/clock"
	"forklift-training/internal/model"
	"forklift-training/internal/questionbank"
	"forklift-training/internal/testutil"
)

// seedInProgressMockExam 落一条进行中的模考记录（题目集 / 作答快照都经 JSONB，与线上同形）。
func seedInProgressMockExam(t *testing.T, db *gorm.DB, studentID int, ids []int, answers map[string]any) *model.MockExam {
	t.Helper()
	idsJSON, err := json.Marshal(ids)
	if err != nil {
		t.Fatalf("序列化题目集失败: %v", err)
	}
	answersJSON, err := json.Marshal(answers)
	if err != nil {
		t.Fatalf("序列化作答快照失败: %v", err)
	}
	now := clock.Now()
	mock := &model.MockExam{
		StudentID:     studentID,
		QuestionIDs:   model.JSONB(idsJSON),
		Answers:       model.JSONB(answersJSON),
		Duration:      90,
		Status:        StatusInProgress,
		StartTime:     &now,
		RemainingTime: 5400,
		CreatedAt:     now,
	}
	if err := db.Create(mock).Error; err != nil {
		t.Fatalf("插入进行中的模考失败: %v", err)
	}
	return mock
}

// assertTotalIsSumOfDetails ADR-0068 决策 2 的机器判据：总分 == 逐题得分之和。
func assertTotalIsSumOfDetails(t *testing.T, got *MockExamSubmitDTO) {
	t.Helper()
	sum := 0.0
	for _, d := range got.Details {
		sum += d.Score
	}
	if diff := got.TotalScore - sum; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("total_score = %v, 但 Σ details[].score = %v（同一份交卷结果自相矛盾）", got.TotalScore, sum)
	}
}

// TestMockExamSubmitTotalScoreCountsPartialAndAIScore 交卷总分覆盖三条分支：
// 多选半对（IsCorrect=false 但有部分分）、简答 AI 分（IsCorrect 恒 nil）、单选整题判对。
func TestMockExamSubmitTotalScoreCountsPartialAndAIScore(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewService(db, nil, zap.NewNop())
	svc.grader = &fakeGrader{res: &aiassistant.GradeResult{Score: 8, Comment: "回答到位"}}

	multi := testutil.SeedQuestion(t, db, "multi_choice", "多选", "A,B,C")
	short := testutil.SeedQuestion(t, db, "short_answer", "简答", "参考答案")
	single := testutil.SeedQuestion(t, db, "single_choice", "单选", "A")
	student := testutil.SeedStudent(t, db, "李四", "x")

	mock := seedInProgressMockExam(t, db, student.ID, []int{multi.ID, short.ID, single.ID}, map[string]any{
		questionbank.IntToString(multi.ID):  []string{"A"}, // 3 选 1 ⇒ 4×1/3×0.5 ≈ 0.7（round1）
		questionbank.IntToString(short.ID):  "我的作答",
		questionbank.IntToString(single.ID): "A",
	})

	got, err := svc.Submit(mock.ID, student.ID)
	if err != nil {
		t.Fatalf("交卷失败: %v", err)
	}
	assertTotalIsSumOfDetails(t, got)

	if got.TotalScore != 11.7 {
		t.Errorf("total_score = %v, want 11.7（0.7 半对 + 8 简答 + 3 单选）", got.TotalScore)
	}
	if got.MaxScore != 17 {
		t.Errorf("max_score = %v, want 17（4 + 10 + 3）", got.MaxScore)
	}
	// correct_count 仍只认整题判对：半对与短答（IsCorrect 恒 nil）都不计入。
	if got.CorrectCount != 1 {
		t.Errorf("correct_count = %d, want 1（只有单选整题判对）", got.CorrectCount)
	}
	if len(got.Details) != 3 {
		t.Fatalf("明细应有 3 条, got %d", len(got.Details))
	}

	half := got.Details[0]
	if half.Score != 0.7 || half.IsCorrect == nil || *half.IsCorrect {
		t.Errorf("多选半对明细 = {score:%v is_correct:%v}, want {0.7 false}", half.Score, boolPtrVal(half.IsCorrect))
	}
	sa := got.Details[1]
	if sa.IsCorrect != nil {
		t.Errorf("短答 is_correct 应保持 nil, got %v", boolPtrVal(sa.IsCorrect))
	}
	// 决策 3 明细同源：短答的 score 与 ai_score 是同一个数，不再出现「0 分」与「AI 给 8 分」并排。
	if sa.Score != 8 {
		t.Errorf("短答明细 score = %v, want 8（= AI 分）", sa.Score)
	}
	if sa.AIScore == nil || *sa.AIScore != 8 {
		t.Errorf("短答明细 ai_score = %v, want 8", sa.AIScore)
	}

	// 落库面：mock_exam.score 与 result JSON 的 total_score 同源。
	var saved model.MockExam
	if err := db.First(&saved, mock.ID).Error; err != nil {
		t.Fatalf("读回模考记录失败: %v", err)
	}
	if saved.Score == nil || *saved.Score != got.TotalScore {
		t.Errorf("mock_exam.score = %v, want %v", saved.Score, got.TotalScore)
	}
	var persisted MockExamSubmitDTO
	if err := json.Unmarshal(saved.Result, &persisted); err != nil {
		t.Fatalf("解析落库 result 失败: %v", err)
	}
	assertTotalIsSumOfDetails(t, &persisted)
}

// TestMockExamSubmitShortAnswerWithoutAIScoresZero AI 不可用时短答降级为 0 分（不是保留旧分），
// 且 ai_score 键缺席（omitempty 契约：无 AI 分就没有这个字段）。
func TestMockExamSubmitShortAnswerWithoutAIScoresZero(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewService(db, nil, zap.NewNop()) // ai=nil ⇒ grader=nil ⇒ 短答降级

	multi := testutil.SeedQuestion(t, db, "multi_choice", "多选", "A,B,C")
	short := testutil.SeedQuestion(t, db, "short_answer", "简答", "参考答案")
	student := testutil.SeedStudent(t, db, "王五", "x")

	mock := seedInProgressMockExam(t, db, student.ID, []int{multi.ID, short.ID}, map[string]any{
		questionbank.IntToString(multi.ID): []string{"A"},
		questionbank.IntToString(short.ID): "我的作答",
	})

	got, err := svc.Submit(mock.ID, student.ID)
	if err != nil {
		t.Fatalf("交卷失败: %v", err)
	}
	assertTotalIsSumOfDetails(t, got)
	if got.TotalScore != 0.7 {
		t.Errorf("total_score = %v, want 0.7（只有多选半对；短答无 AI 记 0）", got.TotalScore)
	}
	sa := got.Details[1]
	if sa.Score != 0 || sa.AIScore != nil || sa.AIComment != nil || sa.AIFallback != nil {
		t.Errorf("无 AI 的短答明细应 score=0 且三个 ai_* 字段缺席, got %+v", sa)
	}
	if sa.IsCorrect != nil {
		t.Errorf("短答 is_correct 应保持 nil, got %v", boolPtrVal(sa.IsCorrect))
	}
}

// fakeGrader 短答 AI 判分 adapter 的测试替身（留驻 service 侧的就地内联副本：原定义随判分内核
// 搬去 internal/practicemode/grading_test.go，ADR-0070 波 3c-2；生产侧接口现为
// practicemode.ShortAnswerGrader，本替身只对着它实现，别在两个包里各改一半）。
type fakeGrader struct {
	res              *aiassistant.GradeResult
	called           int
	gotStudentAnswer string
	gotMaxScore      float64
}

func (f *fakeGrader) GradeShortAnswer(_, _, _, studentAnswer string, maxScore float64) *aiassistant.GradeResult {
	f.called++
	f.gotStudentAnswer = studentAnswer
	f.gotMaxScore = maxScore
	return f.res
}
