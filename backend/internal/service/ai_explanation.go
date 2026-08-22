// Package service AI 解析 module：题目解析 get-or-generate 的唯一实现。
// 缓存（question.ai_explanation 列）与 LLM 生成器是两个 adapter 槽位；
// 练习提交与错题重做共用同一入口，降级策略单点承载（spec #295）。
package service

import (
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// ExplanationGenerator AI 解析生成 adapter：*AIService 满足此接口，测试可注入 fake。
type ExplanationGenerator interface {
	GenerateQuestionExplanation(questionContent, answer, explanation string) (string, error)
}

// explanationGeneratorOf 将 *AIService 归一为 adapter 接口
// （避免「nil 指针包进非 nil 接口」导致误判生成器可用）。
func explanationGeneratorOf(ai *AIService) ExplanationGenerator {
	if ai == nil {
		return nil
	}
	return ai
}

// QuestionExplanation AI 解析 module：小 interface 承载缓存/生成/降级全部策略。
type QuestionExplanation struct {
	db     *gorm.DB
	gen    ExplanationGenerator
	logger *zap.Logger
}

// NewQuestionExplanation 创建 AI 解析 module。ai 为 nil 时恒降级静态解析。
func NewQuestionExplanation(db *gorm.DB, ai *AIService, logger *zap.Logger) *QuestionExplanation {
	return &QuestionExplanation{db: db, gen: explanationGeneratorOf(ai), logger: logger}
}

// GetOrGenerate 返回题目解析：缓存命中直回 → miss 且生成器可用则同步生成并回写
// （回写失败记日志、不阻断响应）→ 不可用或失败降级静态解析。恒返回非空尽力而为结果。
func (m *QuestionExplanation) GetOrGenerate(q *model.Question) string {
	if q.AIExplanation != "" {
		return q.AIExplanation
	}
	if m.gen == nil {
		return q.Explanation
	}
	content, err := m.gen.GenerateQuestionExplanation(q.Content, q.Answer, q.Explanation)
	if err != nil || content == "" {
		return q.Explanation
	}
	if err := m.db.Model(&model.Question{}).Where("id = ?", q.ID).Update("ai_explanation", content).Error; err != nil {
		m.logger.Error("AI 解析缓存回写失败", zap.Int("question_id", q.ID), zap.Error(err))
	}
	return content
}
