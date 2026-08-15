// Package service 判分 module 表格测试（Ticket #231 C1）。
// 锁定判分编排现状行为：题目集 + flow + AI adapter → 分数 / IsCorrect / 错题入库 / 降级标记。
// 期望值为独立字面量，重构四流走 module 后仍须全绿。
package service

import (
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// fakeGrader 可注入的短答 AI adapter，记录调用参数并按预设结果返回。
type fakeGrader struct {
	res              *AIGradeResult
	called           int
	gotStudentAnswer string
	gotMaxScore      float64
	gotUserID        *int
}

func (f *fakeGrader) GradeShortAnswer(_, _, _, studentAnswer string, maxScore float64, userID *int) *AIGradeResult {
	f.called++
	f.gotStudentAnswer = studentAnswer
	f.gotMaxScore = maxScore
	f.gotUserID = userID
	return f.res
}

// maxScoreByFlowOf 按流返回满分 resolver（与四流各自分值表单点对接）。
func maxScoreByFlowOf(flow string) func(q *model.Question) float64 {
	switch flow {
	case "mock_exam":
		return mockExamMaxScore
	default: // level_exam
		return func(q *model.Question) float64 {
			return questionMaxScore("level_exam", q.Type)
		}
	}
}

func TestGradingEngineGradeSet(t *testing.T) {
	// 分值表（product 设定，勿动）：mock 简答 10、level 简答 5、单选均 3。
	type gradeCase struct {
		name       string
		flow       string
		qType      string
		answer     any
		userID     *int
		aiRes      *AIGradeResult // nil 表示 adapter 返回 nil（无 AI 分）
		wantResult GradeResult
		wantWrong  bool // 是否应落入错题库
		wantAICall bool // 短答是否触发 AI adapter
	}

	cases := []gradeCase{
		{
			name:       "客观题-单选答对",
			flow:       "level_exam",
			qType:      "single_choice",
			answer:     "A",
			wantResult: GradeResult{IsCorrect: boolPtr(true), Earned: 3, MaxScore: 3},
			wantWrong:  false,
		},
		{
			name:       "客观题-单选答错入错题库",
			flow:       "level_exam",
			qType:      "single_choice",
			answer:     "B",
			wantResult: GradeResult{IsCorrect: boolPtr(false), Earned: 0, MaxScore: 3},
			wantWrong:  true,
		},
		{
			name:   "短答-AI评分成功及格",
			flow:   "level_exam",
			qType:  "short_answer",
			answer: "我的作答",
			aiRes:  &AIGradeResult{Score: 4, Comment: "回答到位"},
			wantResult: GradeResult{
				IsCorrect:   nil,
				Earned:      0,
				MaxScore:    5,
				ShortAnswer: &ShortAnswerGrade{Score: 4, Comment: "回答到位", Fallback: false, Passed: true},
			},
			wantAICall: true,
		},
		{
			name:   "短答-AI评分不及格",
			flow:   "level_exam",
			qType:  "short_answer",
			answer: "答非所问",
			aiRes:  &AIGradeResult{Score: 2, Comment: "偏题"},
			wantResult: GradeResult{
				IsCorrect:   nil,
				Earned:      0,
				MaxScore:    5,
				ShortAnswer: &ShortAnswerGrade{Score: 2, Comment: "偏题", Fallback: false, Passed: false},
			},
			wantAICall: true,
		},
		{
			name:   "短答-AI降级-前缀与Fallback同写",
			flow:   "mock_exam",
			qType:  "short_answer",
			answer: "作答",
			aiRes:  &AIGradeResult{Score: 0, Comment: "AI评分暂不可用，请等待导师人工评分", Fallback: true},
			wantResult: GradeResult{
				IsCorrect: nil,
				Earned:    0,
				MaxScore:  10,
				ShortAnswer: &ShortAnswerGrade{
					Score:    0,
					Comment:  "[AI评分降级] AI评分暂不可用，请等待导师人工评分",
					Fallback: true,
					Passed:   false,
				},
			},
			wantAICall: true,
		},
		{
			name:   "短答-AI不可用-adapter返回nil",
			flow:   "level_exam",
			qType:  "short_answer",
			answer: "作答",
			aiRes:  nil,
			wantResult: GradeResult{
				IsCorrect:   nil,
				Earned:      0,
				MaxScore:    5,
				ShortAnswer: nil,
			},
			wantAICall: true,
		},
		{
			name:       "客观题-多选部分对",
			flow:       "level_exam",
			qType:      "multi_choice",
			answer:     []string{"A"},
			wantResult: GradeResult{IsCorrect: boolPtr(false), Earned: 1, MaxScore: 4},
			wantWrong:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testutil.NewMemoryDB(t)
			engine := newGradingEngine(db)

			q := testutil.SeedQuestion(t, db, tc.qType, "题干-"+tc.qType, answerForType(tc.qType))
			student := testutil.SeedStudent(t, db, "张三", "x")

			g := &fakeGrader{res: tc.aiRes}
			flow := gradingFlow{
				ai:       shortAnswerGraderOf(nil), // 非短答不触发；短答用 fake 覆盖
				maxScore: maxScoreByFlowOf(tc.flow),
				aiUserID: tc.userID,
			}
			if tc.qType == "short_answer" {
				flow.ai = g
			}

			results := engine.gradeSet(flow, map[int]*model.Question{q.ID: q}, []int{q.ID}, map[string]any{intToString(q.ID): tc.answer}, student.ID)
			if len(results) != 1 {
				t.Fatalf("判分结果应恰 1 条, got %d", len(results))
			}
			got := results[0]

			// 分数
			if got.Earned != tc.wantResult.Earned {
				t.Errorf("Earned = %v, want %v", got.Earned, tc.wantResult.Earned)
			}
			if got.MaxScore != tc.wantResult.MaxScore {
				t.Errorf("MaxScore = %v, want %v", got.MaxScore, tc.wantResult.MaxScore)
			}
			// IsCorrect
			if !boolPtrEq(got.IsCorrect, tc.wantResult.IsCorrect) {
				t.Errorf("IsCorrect = %v, want %v", boolPtrVal(got.IsCorrect), boolPtrVal(tc.wantResult.IsCorrect))
			}
			// 短答分支
			if tc.wantResult.ShortAnswer == nil {
				if got.ShortAnswer != nil {
					t.Errorf("ShortAnswer 应为 nil, got %+v", got.ShortAnswer)
				}
			} else {
				if got.ShortAnswer == nil {
					t.Fatalf("ShortAnswer 不应为 nil")
				}
				want := tc.wantResult.ShortAnswer
				if got.ShortAnswer.Score != want.Score {
					t.Errorf("ShortAnswer.Score = %v, want %v", got.ShortAnswer.Score, want.Score)
				}
				if got.ShortAnswer.Comment != want.Comment {
					t.Errorf("ShortAnswer.Comment = %q, want %q", got.ShortAnswer.Comment, want.Comment)
				}
				if got.ShortAnswer.Fallback != want.Fallback {
					t.Errorf("ShortAnswer.Fallback = %v, want %v", got.ShortAnswer.Fallback, want.Fallback)
				}
				if got.ShortAnswer.Passed != want.Passed {
					t.Errorf("ShortAnswer.Passed = %v, want %v", got.ShortAnswer.Passed, want.Passed)
				}
			}
			// AI adapter 触发
			if got.ShortAnswer != nil || tc.qType == "short_answer" {
				if (g.called > 0) != tc.wantAICall {
					t.Errorf("AI adapter 调用次数 = %d, want 触发 %v", g.called, tc.wantAICall)
				}
			}
			// 错题入库
			assertWrongQuestion(t, db, student.ID, q.ID, tc.wantWrong)
		})
	}
}

// TestGradeShortAnswerNilAdapter 短答 AI adapter 为 nil 时返回 nil（调用方降级）。
func TestGradeShortAnswerNilAdapter(t *testing.T) {
	q := &model.Question{Type: "short_answer", Content: "c", ReferenceAnswer: "ra", ScoringCriteria: "sc"}
	if got := gradeShortAnswer(nil, q, "作答", 5, nil); got != nil {
		t.Fatalf("nil adapter 应返回 nil, got %+v", got)
	}
	if got := gradeShortAnswer(nil, q, "", 5, nil); got != nil {
		t.Fatalf("nil adapter 应返回 nil, got %+v", got)
	}
}

// TestHasFallbackComment 降级前缀判定单点（读侧还原 ai_fallback）。
func TestHasFallbackComment(t *testing.T) {
	if !hasFallbackComment("[AI评分降级] 暂不可用") {
		t.Error("带降级前缀应判定为 true")
	}
	if hasFallbackComment("回答到位") {
		t.Error("无前缀应判定为 false")
	}
}

// answerForType 按题型构造正确答案（与 gradeQuestion 判定语义一致）。
func answerForType(qType string) string {
	switch qType {
	case "multi_choice":
		return "A,B"
	case "true_false":
		return "true"
	case "short_answer":
		return "参考答案"
	default:
		return "A"
	}
}

func boolPtrEq(a, b *bool) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func boolPtrVal(b *bool) any {
	if b == nil {
		return nil
	}
	return *b
}

func assertWrongQuestion(t *testing.T, db *gorm.DB, studentID, questionID int, wantExists bool) {
	t.Helper()
	var count int64
	db.Model(&model.WrongQuestion{}).Where("student_id = ? AND question_id = ?", studentID, questionID).Count(&count)
	exists := count > 0
	if exists != wantExists {
		t.Errorf("错题入库状态 = %v, want %v", exists, wantExists)
	}
}
