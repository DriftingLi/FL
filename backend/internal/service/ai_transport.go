// Package service 传输端口：Blocking/Streaming 双 slot（#397）。
// 端口承载「凭证解析（经注入 resolver）+ 建 client + 调用 + 超时纪律」；prompt 组装、
// 响应解析与持久化是各栈的真语义，留在原服务。形状范本：ai_explanation.go 的 ExplanationGenerator。
package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
	"github.com/sashabaranov/go-openai"
)

// AISettings AI 配置快照。Source 标识配置来源，便于前端展示与诊断。
// （自 settings_service.go 迁入：AI 域类型归属 AI 域文件。）
type AISettings struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	Source  string `json:"source"` // "binding:*" | "user:*" | "custom" | "unbound" | "decrypt-failed"
}

// AIModelSelector 对话凭证选择子（流式栈解析输入）：调用方对「用哪个模型」的纯数据
// 声明（自 StreamChatReq 投影，不含解析知识）；专项单绑定 → Mode 双模式 → 旧
// ModelSource 的优先级与降级全部由 AIConfigResolver 实现承载（ADR-0029 决策 2）。
type AIModelSelector struct {
	FeatureKey    string          // 专项功能键（管理端单绑定，防绕过最优先）
	Mode          AIAssistantMode // 通用助手：normal | expert
	ModelSource   string          // 兼容旧： "admin" | "user" | "custom"
	ConfigID      int             // 兼容旧：ModelSource="admin" 时引用管理员配置
	UserModelID   int             // 兼容旧：ModelSource="user" 时引用用户自定义模型
	UserID        int             // 兼容旧：ModelSource="user" 的归属校验
	CustomAPIKey  string          // 兼容旧：ModelSource="custom" 时临时输入
	CustomBaseURL string          // 兼容旧：ModelSource="custom"
	CustomModel   string          // 兼容旧：ModelSource="custom"
}

// AIConfigResolver 凭证 resolver 端口（ADR-0029 决策 2 的注入通道）：featureKey/选择子 →
// AISettings 的解析内聚在 *AIConfigService 单实现，阻塞与流式两栈注入同一接口；
// 两个方法按调用形态分化（与超时纪律同则），解析知识不泄出配置 service。
type AIConfigResolver interface {
	// ResolveFeatureSettings 阻塞栈流向：featureKey → 管理端单绑定凭证（空键/未绑定报错）。
	ResolveFeatureSettings(ctx context.Context, featureKey string) (AISettings, error)
	// ResolveChatSettings 流式栈流向：选择子 → 对话凭证（专项单绑定 → 双模式 → 旧来源）。
	ResolveChatSettings(ctx context.Context, sel AIModelSelector) (AISettings, error)
}

// AIBlockingTransport 阻塞式传输端口（go-openai 槽位）：一次请求一次完整回复，
// 服务评分/解析/章节内容生成。*AIService 为默认 adapter，测试可注入 fake。
type AIBlockingTransport interface {
	CallModel(messages []openai.ChatCompletionMessage, maxTokens int, temperature float32, featureKey string) (string, error)
}

// AIStreamingTransport 流式传输端口（eino 槽位）：增量经 onChunk 回调透传，
// 返回累积完整回复。凭证解析收进端口内（经注入 resolver，调用方只传选择子）。
// *AIAssistantService 为默认 adapter，测试可注入 fake。
type AIStreamingTransport interface {
	StreamComplete(ctx context.Context, sel AIModelSelector, msgs []*schema.Message, onChunk func(string)) (string, error)
}

// blockingSlot 返回阻塞槽位（默认自实装；测试可注入 fake）。
func (s *AIService) blockingSlot() AIBlockingTransport {
	if s.blocking != nil {
		return s.blocking
	}
	return s
}

// streamingSlot 返回流式槽位（默认自实装；测试可注入 fake）。
func (s *AIAssistantService) streamingSlot() AIStreamingTransport {
	if s.streamer != nil {
		return s.streamer
	}
	return s
}

// 超时纪律单点（ADR-0029 决策 3）：阻塞/流式按调用形态在此分化，两栈不再各自为政。
const (
	// aiBlockingTimeout 阻塞栈总时长上限（评分/解析/章节生成等一次请求一次完整回复的调用）。
	aiBlockingTimeout = 120 * time.Second
	// aiStreamTimeout 流式栈总时长上限：流式此前无超时纪律（自动命名走
	// context.Background() 完全无界），在此单点封顶；比阻塞栈宽松（长对话）。
	aiStreamTimeout = 300 * time.Second
)

// aiBlockingContext 阻塞栈超时纪律包装单点（CallModel 唯一消费）。
func aiBlockingContext() (context.Context, context.CancelFunc) {
	return withTimeout(aiBlockingTimeout)
}

// StreamComplete 流式槽位默认实现：300s 超时纪律 → 注入 resolver 解析凭证（单点）→
// 建 eino client → 流式调用 → Recv 收集循环单点。
func (s *AIAssistantService) StreamComplete(ctx context.Context, sel AIModelSelector, msgs []*schema.Message, onChunk func(string)) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, aiStreamTimeout)
	defer cancel()
	mc, err := s.resolver.ResolveChatSettings(ctx, sel)
	if err != nil {
		return "", err
	}
	chatModel, err := newEinoChatModel(ctx, mc)
	if err != nil {
		return "", fmt.Errorf("构建模型失败: %w", err)
	}
	reader, err := chatModel.Stream(ctx, msgs)
	if err != nil {
		return "", fmt.Errorf("调用模型失败: %w", err)
	}
	content, err := collectStreamReader(reader, onChunk)
	if err != nil {
		return content, fmt.Errorf("流式接收失败: %w", err)
	}
	return content, nil
}

// newEinoChatModel eino 流式 client 构建单点。
func newEinoChatModel(ctx context.Context, mc AISettings) (*einoopenai.ChatModel, error) {
	return einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
		APIKey:  mc.APIKey,
		BaseURL: mc.BaseURL,
		Model:   mc.Model,
	})
}

// newOpenAIClient go-openai 阻塞 client 构建单点（ensureClient 与 TestConfig 共用）。
func newOpenAIClient(apiKey, baseURL string) *openai.Client {
	cfg := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	return openai.NewClientWithConfig(cfg)
}

// collectStreamReader 流式接收循环单点：EOF 正常结束，增量回调，返回累积内容
// （错误时返回已累积的部分内容，与既有语义一致）。reader 由此统一关闭。
func collectStreamReader(reader *schema.StreamReader[*schema.Message], onChunk func(string)) (string, error) {
	defer reader.Close()
	var sb strings.Builder
	for {
		msg, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			return sb.String(), nil
		}
		if err != nil {
			return sb.String(), err
		}
		if msg != nil && msg.Content != "" {
			sb.WriteString(msg.Content)
			if onChunk != nil {
				onChunk(msg.Content)
			}
		}
	}
}
