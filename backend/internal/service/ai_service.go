// Package service 实现业务服务层。
package service

import (
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/schema"
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// 评分/章节生成/题目解析的系统提示词在 ai_feature_registry.go 注册表声明（ADR-0030 决策 1），
// 经派生面 featureSystemPrompt(featureKey) 取用，不再本地持有常量。

// AIService 封装 AI 模型调用、文本生成与简答题评分。
// 模型传输统一经注入的 AIModelPort（eino 唯一生产 adapter，ADR-0029 T2）完成；
// 本服务只保留各消费功能的真语义：prompt 组装、响应解析与持久化。
type AIService struct {
	db     *gorm.DB
	port   AIModelPort // 单一模型端口（构造期注入不变量；测试可注入 fake）
	logger *zap.Logger
}

// NewAIService 创建 AI 服务。port 为单一模型端口（NewEinoAIModel 产物与流式侧共享），
// 必须非 nil：构造期注入是不变量。
func NewAIService(db *gorm.DB, port AIModelPort, logger *zap.Logger) *AIService {
	return &AIService{db: db, port: port, logger: logger}
}

// AIGradeResult 简答题 AI 评分结果。
type AIGradeResult struct {
	Score    float64 `json:"score"`
	Comment  string  `json:"comment"`
	Fallback bool    `json:"fallback,omitempty"`
}

// GradeShortAnswer 简答题 AI 评分。
func (s *AIService) GradeShortAnswer(questionContent, referenceAnswer, scoringCriteria, studentAnswer string, maxScore float64, userID *int) *AIGradeResult {
	if strings.TrimSpace(studentAnswer) == "" {
		return &AIGradeResult{Score: 0, Comment: "未作答，得0分"}
	}
	if referenceAnswer == "" && scoringCriteria == "" {
		return &AIGradeResult{Score: 0, Comment: "题目缺少参考答案和评分标准，无法AI评分，请等待导师人工评分", Fallback: true}
	}
	userPrompt := fmt.Sprintf("【题目】%s\n\n【参考答案】%s\n\n【评分标准】%s\n\n【满分】%g分\n\n【学员答案】%s\n\n请根据以上信息对学员答案进行评分，返回JSON格式。",
		questionContent, orDefault(referenceAnswer, "无"), orDefault(scoringCriteria, "无"), maxScore, studentAnswer)

	content, err := s.port.Complete(FeatureGradeShortAnswer, []*schema.Message{
		schema.SystemMessage(featureSystemPrompt(FeatureGradeShortAnswer)),
		schema.UserMessage(userPrompt),
	}, AICompleteOptions{MaxTokens: 1000, Temperature: 0.3})

	if err != nil || content == "" {
		s.logger.Error("AI grade_short_answer failed", zap.Error(err))
		return &AIGradeResult{Score: 0, Comment: "AI评分暂不可用，请等待导师人工评分", Fallback: true}
	}
	result := parseGradingResponse(content, maxScore)
	if result == nil {
		return &AIGradeResult{Score: 0, Comment: "AI评分结果解析失败，请等待导师人工评分", Fallback: true}
	}
	if userID != nil {
		s.saveLog(*userID, "admin", "content", map[string]any{
			"question":       truncate(questionContent, 100),
			"student_answer": truncate(studentAnswer, 100),
			"max_score":      maxScore,
		}, fmt.Sprintf("{\"score\":%g,\"comment\":%q}", result.Score, result.Comment), 1)
	}
	return result
}

// GenerateQuestionExplanation 为题目生成 AI 解析。
func (s *AIService) GenerateQuestionExplanation(questionContent, answer, explanation string) (string, error) {
	userPrompt := fmt.Sprintf("【题目】%s\n\n【正确答案】%s\n\n【参考解析】%s\n\n请生成本题的 AI 解析。", questionContent, orDefault(answer, "无"), orDefault(explanation, "无"))
	content, err := s.port.Complete(FeatureQuestionExplanation, []*schema.Message{
		schema.SystemMessage(featureSystemPrompt(FeatureQuestionExplanation)),
		schema.UserMessage(userPrompt),
	}, AICompleteOptions{MaxTokens: 800, Temperature: 0.5})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(content), nil
}

// GenerateChapterContent 为指定章节生成 Markdown 内容。
// 调用 LLM 根据课程上下文和章节标题生成培训内容，写入 ai_generation_log（generation_type=chapter_content）。
func (s *AIService) GenerateChapterContent(courseName, courseCategory, courseDescription, chapterTitle string, userID *int) (string, error) {
	userPrompt := fmt.Sprintf("【课程名称】%s\n【课程分类】%s\n【课程简介】%s\n【章节标题】%s\n\n请根据以上信息生成该章节的培训内容（Markdown 格式）。",
		courseName, orDefault(courseCategory, "无"), orDefault(courseDescription, "无"), chapterTitle)

	content, err := s.port.Complete(FeatureGenerateChapterContent, []*schema.Message{
		schema.SystemMessage(featureSystemPrompt(FeatureGenerateChapterContent)),
		schema.UserMessage(userPrompt),
	}, AICompleteOptions{MaxTokens: 2000, Temperature: 0.5})
	if err != nil {
		return "", err
	}
	if userID != nil {
		s.saveLog(*userID, "admin", "chapter_content", map[string]any{
			"course_name":   courseName,
			"chapter_title": chapterTitle,
		}, truncate(content, 5000), 1)
	}
	return content, nil
}

// saveLog 记录 AI 生成日志。
func (s *AIService) saveLog(userID int, userType, generationType string, inputParams interface{}, outputResult string, status int16) {
	var paramsBytes model.JSONB
	if inputParams != nil {
		if b, err := json.Marshal(inputParams); err == nil {
			paramsBytes = model.JSONB(b)
		}
	}
	out := outputResult
	if len(out) > 5000 {
		out = out[:5000]
	}
	log := model.AIGenerationLog{
		UserID:         userID,
		UserType:       userType,
		GenerationType: generationType,
		InputParams:    paramsBytes,
		OutputResult:   out,
		Status:         status,
		CreatedAt:      beijingNow(),
	}
	if err := s.db.Create(&log).Error; err != nil {
		s.logger.Error("saveLog failed", zap.Error(err))
	}
}

// parseGradingResponse 解析 AI 评分 JSON 响应。
func parseGradingResponse(content string, maxScore float64) *AIGradeResult {
	if content == "" {
		return nil
	}
	text := strings.TrimSpace(content)
	// 去除 ``` 代码块包裹
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) > 1 {
			end := len(lines) - 1
			if strings.TrimSpace(lines[end]) == "```" {
				text = strings.Join(lines[1:end], "\n")
			} else {
				text = strings.Join(lines[1:], "\n")
			}
		}
	}
	// 直接解析整段 JSON
	if r := tryParseScore(text, maxScore); r != nil {
		return r
	}
	// 正则匹配 {"score":...}
	if m := regexp.MustCompile(`\{.*?"score".*?\}`).FindString(text); m != "" {
		if r := tryParseScore(m, maxScore); r != nil {
			return r
		}
	}
	// 提取第一个含 score 的 {...}
	if r := extractBraceJSON(text, maxScore); r != nil {
		return r
	}
	// "score": 数字
	if m := regexp.MustCompile(`"score"\s*:\s*([\d.]+)`).FindStringSubmatch(text); len(m) > 1 {
		f, _ := parseFloat(m[1]) // AI 评分解析失败显式回退 0。
		score := clampFloat(f, 0, maxScore)
		comment := ""
		if cm := regexp.MustCompile(`"comment"\s*:\s*"((?:[^"\\]|\\.)*)"`).FindStringSubmatch(text); len(cm) > 1 {
			comment = strings.ReplaceAll(strings.ReplaceAll(cm[1], `\n`, "\n"), `\"`, `"`)
		}
		return &AIGradeResult{Score: score, Comment: comment}
	}
	// 数字/满分 形式
	if m := regexp.MustCompile(fmt.Sprintf(`(\d+(?:\.\d+)?)\s*/\s*%g`, maxScore)).FindStringSubmatch(text); len(m) > 1 {
		f, _ := parseFloat(m[1]) // AI 评分解析失败显式回退 0。
		return &AIGradeResult{Score: clampFloat(f, 0, maxScore), Comment: "AI评分"}
	}
	if m := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*分`).FindStringSubmatch(text); len(m) > 1 {
		f, _ := parseFloat(m[1]) // AI 评分解析失败显式回退 0。
		return &AIGradeResult{Score: clampFloat(f, 0, maxScore), Comment: "AI评分"}
	}
	return nil
}

func tryParseScore(s string, maxScore float64) *AIGradeResult {
	var obj map[string]any
	if err := json.Unmarshal([]byte(s), &obj); err != nil {
		return nil
	}
	score := toFloat(obj["score"])
	comment, _ := obj["comment"].(string)
	return &AIGradeResult{Score: clampFloat(score, 0, maxScore), Comment: comment}
}

func extractBraceJSON(text string, maxScore float64) *AIGradeResult {
	depth, start := 0, -1
	for i, ch := range text {
		switch ch {
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			depth--
			if depth == 0 && start >= 0 {
				candidate := text[start : i+1]
				if strings.Contains(candidate, `"score"`) {
					if r := tryParseScore(candidate, maxScore); r != nil {
						return r
					}
				}
			}
		}
	}
	return nil
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
