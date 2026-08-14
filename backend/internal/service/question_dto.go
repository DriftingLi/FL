package service

import (
	"forklift-training/internal/model"
)

// QuestionDTO 题目契约：JSON key 与历史 map 输出逐字一致（shape-lock 测试
// question_dto_shape_test.go 冻结）。字段声明按 key 字母序，与 map 序列化字节序一致。
type QuestionDTO struct {
	// 学员侧（includeAnswer=false）省略以下四个字段。
	Answer          *string `json:"answer,omitempty"`
	Content         string  `json:"content"`
	CreatedAt       string  `json:"created_at"`
	CreatedBy       *int    `json:"created_by"`
	CreatedByType   string  `json:"created_by_type"`
	Explanation     *string `json:"explanation,omitempty"`
	ID              int     `json:"id"`
	ImageURL        string  `json:"image_url"`
	Options         any     `json:"options"`
	ReferenceAnswer *string `json:"reference_answer,omitempty"`
	RejectReason    string  `json:"reject_reason"`
	Score           int     `json:"score"`
	ScoringCriteria *string `json:"scoring_criteria,omitempty"`
	Status          string  `json:"status"`
	// Tags 题库管理面附加（未设置时省略；设置后保留 null/[] 形态与历史一致）。
	Tags      any    `json:"tags,omitempty"`
	Type      string `json:"type"`
	UpdatedAt string `json:"updated_at"`
}

// newQuestionDTO 将题目转为契约 DTO。
// includeAnswer=false 时省略答案/解析/参考答案/评分标准（学员侧）。
func newQuestionDTO(q *model.Question, includeAnswer bool) QuestionDTO {
	var options any
	if len(q.Options) > 0 {
		_ = jsonUnmarshal(q.Options, &options)
	}
	d := QuestionDTO{
		Content:       q.Content,
		CreatedAt:     formatISO(q.CreatedAt),
		CreatedBy:     q.CreatedBy,
		CreatedByType: q.CreatedByType,
		ID:            q.ID,
		ImageURL:      q.ImageURL,
		Options:       options,
		RejectReason:  q.RejectReason,
		Score:         q.Score,
		Status:        q.Status,
		Type:          q.Type,
		UpdatedAt:     formatISO(q.UpdatedAt),
	}
	if includeAnswer {
		d.Answer = strPtr(q.Answer)
		d.Explanation = strPtr(q.Explanation)
		d.ReferenceAnswer = strPtr(q.ReferenceAnswer)
		d.ScoringCriteria = strPtr(q.ScoringCriteria)
	}
	return d
}

// 各答题流题型分值（单点定义）：分值差异是产品设定（mock 简答/识图与定级不同），勿对齐。
var questionScoreByFlow = map[string]map[string]float64{
	"level_exam": {"single_choice": 3, "multi_choice": 4, "true_false": 2, "fault_image": 6, "short_answer": 5},
	"mock_exam":  {"single_choice": 3, "multi_choice": 4, "true_false": 2, "fault_image": 4, "short_answer": 10},
}

// questionMaxScore 按流取题型满分；未知流/题型返回 0。
func questionMaxScore(flow, qType string) float64 {
	if scores, ok := questionScoreByFlow[flow]; ok {
		if v, ok := scores[qType]; ok {
			return v
		}
	}
	return 0
}

// shortAnswerPassRatio 简答题及格线：得分 ≥ 满分 × 0.6 记为正确（阅卷/练习共用）。
const shortAnswerPassRatio = 0.6

// shortAnswerPassed 简答题及格判定：score ≥ maxScore × shortAnswerPassRatio。
// 0.6 及格公式的唯一实现——阅卷/复核/AI 确认/练习提交均经此推导，不各自重写。
func shortAnswerPassed(score, maxScore float64) bool {
	return score >= maxScore*shortAnswerPassRatio
}

// aiGradeShortAnswer AI 简答评分的统一入口；ai 为 nil 时返回 nil（调用方降级）。
func aiGradeShortAnswer(ai *AIService, questionContent, referenceAnswer, scoringCriteria, studentAnswer string, maxScore float64, userID *int) *AIGradeResult {
	if ai == nil {
		return nil
	}
	return ai.GradeShortAnswer(questionContent, referenceAnswer, scoringCriteria, studentAnswer, maxScore, userID)
}

func strPtr(s string) *string { return &s }
