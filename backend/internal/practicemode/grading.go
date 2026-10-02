// 本文件：判分 module：练习（含错题重做）/模拟考试共享的判分编排。
// 吸收「逐题判分 → 按流分值累分 → 错题入库 → 短答 AI 分支」，并单点生成短答降级语义。
package practicemode

import (
	"gorm.io/gorm"

	"forklift-training/internal/aiassistant"
	"forklift-training/internal/model"
	"forklift-training/internal/questionbank"
)

// fallbackCommentPrefix 短答 AI 评分降级注释统一前缀（四流单点）。
const fallbackCommentPrefix = "[AI评分降级] "

// ShortAnswerGrader AI 简答判分 adapter。测试可注入 fake；
// *aiassistant.GenerationService 经 aiShortAnswerGrader 包装接入（剥离其 userID 日志形参，练习/重做流恒传 nil）。
type ShortAnswerGrader interface {
	GradeShortAnswer(questionContent, referenceAnswer, scoringCriteria, studentAnswer string, maxScore float64) *aiassistant.GradeResult
}

// aiShortAnswerGrader *aiassistant.GenerationService 的判分 adapter 包装。
type aiShortAnswerGrader struct {
	ai *aiassistant.GenerationService
}

func (a aiShortAnswerGrader) GradeShortAnswer(questionContent, referenceAnswer, scoringCriteria, studentAnswer string, maxScore float64) *aiassistant.GradeResult {
	return a.ai.GradeShortAnswer(questionContent, referenceAnswer, scoringCriteria, studentAnswer, maxScore, nil)
}

// ShortAnswerGraderOf 将 *aiassistant.GenerationService 转换为 adapter 接口，nil 归一为 nil 接口
// （避免「nil 指针包进非 nil 接口」导致 gradeShortAnswer 误判可用）。
func ShortAnswerGraderOf(ai *aiassistant.GenerationService) ShortAnswerGrader {
	if ai == nil {
		return nil
	}
	return aiShortAnswerGrader{ai: ai}
}

// ShortAnswerGrade 短答统一判分结果（降级语义单点生成）。
type ShortAnswerGrade struct {
	Score    float64
	Comment  string // fallback 时已统一加「[AI评分降级] 」前缀
	Fallback bool
	Passed   bool // 由 questionbank.ShortAnswerPassed 单点推导（score ≥ maxScore × 0.6）
}

// gradeShortAnswer 短答 AI 判分单点入口：grader 为 nil 或 AI 返回 nil 时返回 nil（调用方降级，不产生 AI 分）。
// 统一生成降级语义——fallback 时注释加前缀且 Fallback 同写，Passed 由 shortAnswerPassed 推导。
func gradeShortAnswer(grader ShortAnswerGrader, q *model.Question, studentAnswer string, maxScore float64) *ShortAnswerGrade {
	if grader == nil {
		return nil
	}
	res := grader.GradeShortAnswer(q.Content, q.ReferenceAnswer, q.ScoringCriteria, studentAnswer, maxScore)
	if res == nil {
		return nil
	}
	comment := res.Comment
	if res.Fallback {
		comment = fallbackCommentPrefix + comment
	}
	return &ShortAnswerGrade{
		Score:    res.Score,
		Comment:  comment,
		Fallback: res.Fallback,
		Passed:   questionbank.ShortAnswerPassed(res.Score, maxScore),
	}
}

// GradeResult 单题判分结果（判分 module 统一产出，各流据此构造各自 DTO / 落库）。
type GradeResult struct {
	Question   *model.Question
	UserAnswer any
	IsCorrect  *bool
	Earned     float64
	MaxScore   float64
	// ShortAnswer 短答 AI 分支（含降级语义）；非短答或 AI 不可用时为 nil。
	ShortAnswer *ShortAnswerGrade
}

// GradingFlow 单流判分配置：分值表 resolver + AI adapter。分值表为 product 设定勿动。
type GradingFlow struct {
	AI       ShortAnswerGrader
	MaxScore func(q *model.Question) float64 // 满分解析（各流分值差异在此单点注入）
}

// GradingEngine 判分 module。
type GradingEngine struct {
	db *gorm.DB
}

// NewGradingEngine 创建判分 module。
func NewGradingEngine(db *gorm.DB) *GradingEngine {
	return &GradingEngine{db: db}
}

// GradeOne 单题判分：按流分值取满分 → gradeQuestion → 短答 AI 分支 → 错题入库。
// 错题入库规则单点：仅客观题判错（isCorrect 非 nil 且 false）入库，与四流现状语义一致。
//
// Earned 是**本题得分**的唯一出口（ADR-0068 决策 1）：客观题取判分结果（含多选部分分），
// 简答题取 AI 评分（AI 不可用或降级评分为 0）。装配层不得再用 IsCorrect 过滤 Earned ——
// 那正是「半对与简答都进不了总分」的旧口径。IsCorrect 三态语义不变。
func (e *GradingEngine) GradeOne(f GradingFlow, q *model.Question, userAnswer any, studentID int) GradeResult {
	maxScore := f.MaxScore(q)
	isCorrect, earned := questionbank.GradeQuestion(q, userAnswer, maxScore)

	var sa *ShortAnswerGrade
	if q.Type == "short_answer" {
		sa = gradeShortAnswer(f.AI, q, questionbank.StringifyAnswer(userAnswer), maxScore)
		if sa != nil {
			earned = sa.Score
		}
	}
	if isCorrect != nil && !*isCorrect {
		_ = questionbank.AddToWrongQuestions(e.db, studentID, q.ID)
	}
	return GradeResult{
		Question:    q,
		UserAnswer:  userAnswer,
		IsCorrect:   isCorrect,
		Earned:      earned,
		MaxScore:    maxScore,
		ShortAnswer: sa,
	}
}

// GradeSet 有序题目集逐题判分：保持输入顺序，跳过缺失题目；错题入库与短答 AI 分支已内含。
func (e *GradingEngine) GradeSet(f GradingFlow, qMap map[int]*model.Question, ids []int, answers map[string]any, studentID int) []GradeResult {
	results := make([]GradeResult, 0, len(ids))
	for _, qid := range ids {
		q, ok := qMap[qid]
		if !ok {
			continue
		}
		results = append(results, e.GradeOne(f, q, answers[questionbank.IntToString(qid)], studentID))
	}
	return results
}
