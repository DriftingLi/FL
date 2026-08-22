// Package service 练习提交（SubmitAnswer）直接单测：客观判分落库、
// 简答 AI 及格覆写 IsCorrect 的二次 Save 语义、AI 未配置降级、解析缓存命中。
package service

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestSubmitAnswer_ObjectiveCorrect(t *testing.T) {
	svc, db := newPracticeSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "单选", "A")
	student := testutil.SeedStudent(t, db, "李四", "x")

	result, err := svc.SubmitAnswer(student.ID, q.ID, "A", "free")
	if err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	if result.IsCorrect == nil || !*result.IsCorrect {
		t.Fatalf("应判定正确, got %v", boolPtrVal(result.IsCorrect))
	}
	if result.AIExplanation != "" {
		t.Fatalf("无 AI 配置时不应有 AI 解析: %+v", result)
	}
	var rec model.QuestionPracticeRecord
	if err := db.First(&rec, "student_id = ? AND question_id = ?", student.ID, q.ID).Error; err != nil {
		t.Fatalf("应落练习记录: %v", err)
	}
	if !rec.IsCorrect || rec.PracticeType != "free" {
		t.Fatalf("记录口径不符: %+v", rec)
	}
}

func TestSubmitAnswer_ShortAnswer_AIPassed_OverridesRecord(t *testing.T) {
	svc, db := newPracticeSvc(t)
	q := testutil.SeedQuestion(t, db, "short_answer", "简答", "参考答案")
	db.Model(&model.Question{}).Where("id = ?", q.ID).Updates(map[string]any{"reference_answer": "参考答案", "scoring_criteria": "要点齐全", "score": 5})
	student := testutil.SeedStudent(t, db, "王五", "x")

	svc.grader = &fakeGrader{res: &AIGradeResult{Score: 4, Comment: "回答到位"}} // 题目自定义满分 5，4≥3 及格

	result, err := svc.SubmitAnswer(student.ID, q.ID, "我的作答", "free")
	if err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	if result.AIScore == nil || *result.AIScore != 4 || result.AIComment != "回答到位" {
		t.Fatalf("AI 评分字段不符: %+v", result)
	}
	if result.MaxScore != 5 {
		t.Fatalf("应取题目自定义满分 5, got %d", result.MaxScore)
	}
	if result.IsCorrect == nil || !*result.IsCorrect {
		t.Fatal("AI 及格应覆写 IsCorrect=true")
	}
	var rec model.QuestionPracticeRecord
	if err := db.First(&rec, "student_id = ? AND question_id = ?", student.ID, q.ID).Error; err != nil {
		t.Fatalf("应落练习记录: %v", err)
	}
	if !rec.IsCorrect {
		t.Fatal("二次 Save 应把练习记录同步为正确")
	}
}

func TestSubmitAnswer_ShortAnswer_NoAI_FallsBack(t *testing.T) {
	svc, db := newPracticeSvc(t)
	q := testutil.SeedQuestion(t, db, "short_answer", "简答", "参考答案")
	db.Model(&model.Question{}).Where("id = ?", q.ID).Update("explanation", "静态解析")
	student := testutil.SeedStudent(t, db, "赵六", "x")

	result, err := svc.SubmitAnswer(student.ID, q.ID, "我的作答", "free")
	if err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	if result.IsCorrect != nil {
		t.Fatalf("无 AI 时简答应保持 is_correct=null, got %v", boolPtrVal(result.IsCorrect))
	}
	if result.AIExplanation != "静态解析" || result.MaxScore != 10 {
		t.Fatalf("应降级静态解析且简答默认满分 10: %+v", result)
	}
}

func TestSubmitAnswer_AIExplanation_CacheHit(t *testing.T) {
	svc, db := newPracticeSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "单选", "A")
	db.Model(&model.Question{}).Where("id = ?", q.ID).Update("ai_explanation", "缓存解析")
	student := testutil.SeedStudent(t, db, "孙七", "x")

	result, err := svc.SubmitAnswer(student.ID, q.ID, "B", "free")
	if err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	if result.AIExplanation != "缓存解析" {
		t.Fatalf("应返回缓存解析, got %q", result.AIExplanation)
	}
}

func TestSubmitAnswer_QuestionMissing(t *testing.T) {
	svc, _ := newPracticeSvc(t)
	if _, err := svc.SubmitAnswer(1, 9999, "A", "free"); err == nil {
		t.Fatal("题目不存在应返回错误")
	}
}

// TestSubmitAnswer_AIExplanation_GeneratedAndPersisted 生成分支穿过 service seam：
// miss → 同步生成 → 回写缓存列（spec #294 Testing Decisions 的 AI 三分支之一）。
func TestSubmitAnswer_AIExplanation_GeneratedAndPersisted(t *testing.T) {
	svc, db := newPracticeSvc(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "单选", "A")
	student := testutil.SeedStudent(t, db, "周八", "x")

	gen := &fakeExplGen{content: "现场生成的解析"}
	svc.explainer = &QuestionExplanation{db: db, gen: gen, logger: zap.NewNop()}

	result, err := svc.SubmitAnswer(student.ID, q.ID, "A", "free")
	if err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	if result.AIExplanation != "现场生成的解析" {
		t.Fatalf("应返回生成解析, got %q", result.AIExplanation)
	}
	if gen.called != 1 {
		t.Fatalf("应恰好调用一次生成器, called=%d", gen.called)
	}
	var cached string
	db.Model(&model.Question{}).Where("id = ?", q.ID).Select("ai_explanation").Scan(&cached)
	if cached != "现场生成的解析" {
		t.Fatalf("生成内容应回写缓存列, got %q", cached)
	}
}

// TestPracticeMaxScore 练习流满分解析锁定：简答题目自定义分优先（缺省 10），客观走 practice 表。
func TestPracticeMaxScore(t *testing.T) {
	if got := practiceMaxScore(&model.Question{Type: "short_answer", Score: 7}); got != 7 {
		t.Errorf("简答自定义分优先: got %v want 7", got)
	}
	if got := practiceMaxScore(&model.Question{Type: "short_answer"}); got != 10 {
		t.Errorf("简答缺省满分: got %v want 10", got)
	}
	for qType, want := range map[string]float64{"single_choice": 3, "multi_choice": 4, "true_false": 2, "fault_image": 6} {
		if got := practiceMaxScore(&model.Question{Type: qType}); got != want {
			t.Errorf("practice 表 %s: got %v want %v", qType, got, want)
		}
	}
}

// TestQuestionMaxScore_TablesLocked 两张分值表逐字锁定（product 设定，勿动）。
func TestQuestionMaxScore_TablesLocked(t *testing.T) {
	want := map[string]map[string]float64{
		"practice":  {"single_choice": 3, "multi_choice": 4, "true_false": 2, "fault_image": 6, "short_answer": 5},
		"mock_exam": {"single_choice": 3, "multi_choice": 4, "true_false": 2, "fault_image": 4, "short_answer": 10},
	}
	for flow, rows := range want {
		for qType, v := range rows {
			if got := questionMaxScore(flow, qType); got != v {
				t.Errorf("questionMaxScore(%q,%q) = %v want %v", flow, qType, got, v)
			}
		}
	}
}
