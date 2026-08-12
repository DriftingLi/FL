// Package service 实现业务服务层。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// AI 评分系统提示词。
const gradingSystemPrompt = `你是一名专业的叉车维修培训考试阅卷专家。请根据参考答案和评分标准，对学员的简答题答案进行评分。
要求：
1. 严格按照评分标准逐项评分，意思正确但表述不同也应给分
2. 评分应客观公正，不苛求表述完全一致
3. 给出具体得分和简要评语，评语需指出得分点和失分点
4. 只返回JSON格式，不要返回其他内容：{"score": 分数值, "comment": "评语"}
5. 分数值为数字类型，不要加引号`

// 章节内容生成系统提示词。
const chapterContentSystemPrompt = `你是一名叉车维修培训内容编写专家。请根据课程信息和章节标题，生成适合培训学员的章节内容。
要求：
1. 内容使用 Markdown 格式
2. 包含概述、核心知识点、操作要点、安全注意事项、小结等部分
3. 内容专业、准确、实用，字数 800-1500 字
4. 不要在内容开头重复章节标题（前端会自动显示）
5. 可适当使用 Markdown 标题（##、###）、列表、加粗等格式增强可读性`

// AIService 封装 OpenAI 兼容 API 调用、文本生成与简答题评分。
// 调用时按 feature_key 查找绑定配置（AIConfigService），未绑定时返回错误。
type AIService struct {
	db          *gorm.DB
	aiConfigSvc *AIConfigService
	client      *openai.Client
	clientSig   string // 当前 client 使用的 "key|url|model" 签名，用于检测配置变化
	apiKey      string
	baseURL     string
	model       string
	mu          sync.Mutex // 保护 client 重建并发安全
	logger      *zap.Logger
}

// NewAIService 创建 AI 服务。aiConfigSvc 用于按功能查找绑定配置。
func NewAIService(db *gorm.DB, aiConfigSvc *AIConfigService, logger *zap.Logger) *AIService {
	return &AIService{db: db, aiConfigSvc: aiConfigSvc, logger: logger}
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
	}, 1000, 0.3, FeatureGradeShortAnswer)

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

// GenerateChapterContent 为指定章节生成 Markdown 内容。
// 调用 LLM 根据课程上下文和章节标题生成培训内容，写入 ai_generation_log（generation_type=chapter_content）。
func (s *AIService) GenerateChapterContent(courseName, courseCategory, courseDescription, chapterTitle string, userID *int) (string, error) {
	userPrompt := fmt.Sprintf("【课程名称】%s\n【课程分类】%s\n【课程简介】%s\n【章节标题】%s\n\n请根据以上信息生成该章节的培训内容（Markdown 格式）。",
		courseName, orDefault(courseCategory, "无"), orDefault(courseDescription, "无"), chapterTitle)

	content, err := s.callModel([]openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: chapterContentSystemPrompt},
		{Role: openai.ChatMessageRoleUser, Content: userPrompt},
	}, 2000, 0.5, FeatureGenerateChapterContent)
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

// ensureClient 检查 AI 配置是否变化，必要时重建 openai.Client。
// featureKey 用于查找该功能绑定的配置；未绑定时返回错误（不再降级到环境变量）。
func (s *AIService) ensureClient(ctx context.Context, featureKey string) error {
	if featureKey == "" || s.aiConfigSvc == nil {
		return fmt.Errorf("AI 功能 %q 未绑定配置，请在管理员后台 AI 配置页面绑定", featureKey)
	}
	cur := s.aiConfigSvc.ResolveConfig(ctx, featureKey)
	sig := cur.APIKey + "|" + cur.BaseURL + "|" + cur.Model

	s.mu.Lock()
	defer s.mu.Unlock()
	if sig == s.clientSig && s.client != nil {
		s.model = cur.Model
		return nil
	}
	if cur.APIKey == "" {
		return fmt.Errorf("AI 功能 %q 未绑定配置，请在管理员后台 AI 配置页面绑定", featureKey)
	}
	cfg := openai.DefaultConfig(cur.APIKey)
	if cur.BaseURL != "" {
		cfg.BaseURL = cur.BaseURL
	}
	s.client = openai.NewClientWithConfig(cfg)
	s.clientSig = sig
	s.apiKey, s.baseURL, s.model = cur.APIKey, cur.BaseURL, cur.Model
	s.logger.Info("AI client 已重建", zap.String("base_url", cur.BaseURL), zap.String("model", cur.Model), zap.String("source", cur.Source), zap.String("feature", featureKey))
	return nil
}

// callModel 调用模型，重试 2 次。
// featureKey 用于按功能解析绑定的 AI 配置。
func (s *AIService) callModel(messages []openai.ChatCompletionMessage, maxTokens int, temperature float32, featureKey string) (string, error) {
	ctx, cancel := withTimeout(120 * time.Second)
	defer cancel()

	if err := s.ensureClient(ctx, featureKey); err != nil {
		return "", err
	}

	for attempt := 1; attempt <= 2; attempt++ {
		req := openai.ChatCompletionRequest{
			Model:       s.model,
			Messages:    messages,
			MaxTokens:   maxTokens,
			Temperature: temperature,
		}
		resp, err := s.client.CreateChatCompletion(ctx, req)
		if err != nil {
			s.logger.Error("AI call failed", zap.Int("attempt", attempt), zap.Error(err))
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
