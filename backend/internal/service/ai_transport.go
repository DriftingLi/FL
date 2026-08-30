// Package service 传输端口：Blocking/Streaming 双 slot（#397）。
// 端口只承载「建 client + 调用 + 超时纪律」；prompt 组装、响应解析与持久化
// 是各栈的真语义，留在原服务。形状范本：ai_explanation.go 的 ExplanationGenerator。
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

// AIBlockingTransport 阻塞式传输端口（go-openai 槽位）：一次请求一次完整回复，
// 服务评分/解析/章节内容生成。*AIService 为默认 adapter，测试可注入 fake。
type AIBlockingTransport interface {
	CallModel(messages []openai.ChatCompletionMessage, maxTokens int, temperature float32, featureKey string) (string, error)
}

// AIStreamingTransport 流式传输端口（eino 槽位）：增量经 onChunk 回调透传，
// 返回累积完整回复。*AIAssistantService 为默认 adapter，测试可注入 fake。
type AIStreamingTransport interface {
	StreamComplete(ctx context.Context, mc AISettings, msgs []*schema.Message, onChunk func(string)) (string, error)
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

// aiStreamTimeout 流式栈总时长上限：流式此前无超时纪律（自动命名走
// context.Background() 完全无界），在此单点封顶；比阻塞栈 120s 宽松（长对话）。
const aiStreamTimeout = 300 * time.Second

// StreamComplete 流式槽位默认实现：建 eino client（单点）→ 300s 超时纪律 →
// 流式调用 → Recv 收集循环单点。
func (s *AIAssistantService) StreamComplete(ctx context.Context, mc AISettings, msgs []*schema.Message, onChunk func(string)) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, aiStreamTimeout)
	defer cancel()
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
