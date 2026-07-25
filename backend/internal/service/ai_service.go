// Package service 实现业务服务层。
package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// 智谱 GLM 默认模型与提示词。
const (
	gradingSystemPrompt = `你是一名专业的叉车维修培训考试阅卷专家。请根据参考答案和评分标准，对学员的简答题答案进行评分。
要求：
1. 严格按照评分标准逐项评分，意思正确但表述不同也应给分
2. 评分应客观公正，不苛求表述完全一致
3. 给出具体得分和简要评语，评语需指出得分点和失分点
4. 只返回JSON格式，不要返回其他内容：{"score": 分数值, "comment": "评语"}
5. 分数值为数字类型，不要加引号`
)

// AIService 封装智谱 GLM 调用、文本生成与简答题评分。
type AIService struct {
	db      *gorm.DB
	client  *openai.Client
	apiKey  string
	baseURL string
	model   string
}

// NewAIService 创建 AI 服务。apiKey 为空时 client 为 nil，调用时降级返回 fallback。
func NewAIService(db *gorm.DB, apiKey, baseURL, modelName string) *AIService {
	svc := &AIService{db: db, apiKey: apiKey, baseURL: baseURL, model: modelName}
	if apiKey != "" {
		cfg := openai.DefaultConfig(apiKey)
		if baseURL != "" {
			cfg.BaseURL = baseURL
		}
		svc.client = openai.NewClientWithConfig(cfg)
	}
	return svc
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

	content, err := s.callModel([]openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: gradingSystemPrompt},
		{Role: openai.ChatMessageRoleUser, Content: userPrompt},
	}, 1000, 0.3)

	if err != nil || content == "" {
		slog.Error("AI grade_short_answer failed", "error", err)
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

// callModel 调用模型，重试 2 次。
func (s *AIService) callModel(messages []openai.ChatCompletionMessage, maxTokens int, temperature float32) (string, error) {
	if s.client == nil {
		return "", fmt.Errorf("AI服务未配置，请设置ZHIPU_API_KEY")
	}
	ctx, cancel := withTimeout(120 * time.Second)
	defer cancel()

	for attempt := 1; attempt <= 2; attempt++ {
		req := openai.ChatCompletionRequest{
			Model:       s.model,
			Messages:    messages,
			MaxTokens:   maxTokens,
			Temperature: temperature,
		}
		resp, err := s.client.CreateChatCompletion(ctx, req)
		if err != nil {
			slog.Error("AI call failed", "attempt", attempt, "error", err)
			if attempt == 2 {
				return "", err
			}
			time.Sleep(time.Second)
			continue
		}
		if len(resp.Choices) == 0 {
			if attempt == 2 {
				return "", nil
			}
			time.Sleep(time.Second)
			continue
		}
		content := strings.TrimSpace(resp.Choices[0].Message.Content)
		if content == "" {
			if resp.Choices[0].FinishReason == "content_filter" {
				return "", nil
			}
			if attempt == 2 {
				return "", nil
			}
			time.Sleep(time.Second)
			continue
		}
		return content, nil
	}
	return "", nil
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
		slog.Error("saveLog failed", "error", err)
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
		score := clampFloat(parseFloat(m[1]), 0, maxScore)
		comment := ""
		if cm := regexp.MustCompile(`"comment"\s*:\s*"((?:[^"\\]|\\.)*)"`).FindStringSubmatch(text); len(cm) > 1 {
			comment = strings.ReplaceAll(strings.ReplaceAll(cm[1], `\n`, "\n"), `\"`, `"`)
		}
		return &AIGradeResult{Score: score, Comment: comment}
	}
	// 数字/满分 形式
	if m := regexp.MustCompile(fmt.Sprintf(`(\d+(?:\.\d+)?)\s*/\s*%g`, maxScore)).FindStringSubmatch(text); len(m) > 1 {
		return &AIGradeResult{Score: clampFloat(parseFloat(m[1]), 0, maxScore), Comment: "AI评分"}
	}
	if m := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*分`).FindStringSubmatch(text); len(m) > 1 {
		return &AIGradeResult{Score: clampFloat(parseFloat(m[1]), 0, maxScore), Comment: "AI评分"}
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
