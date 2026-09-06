// Package service 模型端口：单一 AIModelPort（ADR-0029 T2，#607）。
// Complete（阻塞补全，一次请求一次完整回复）与 Stream（流式回调）合一；端口承载
// 「凭证解析（经注入 resolver）+ client 签名缓存 + 调用 + 超时纪律（120s/300s 单点分化）」；
// prompt 组装、响应解析与持久化是各消费方的真语义，留在原服务。
// eino 为唯一生产 adapter，测试 fake 为第二 adapter（seam 坐实）。
package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// AISettings AI 配置快照。Source 标识配置来源，便于前端展示与诊断。
// （自 settings_service.go 迁入：AI 域类型归属 AI 域文件。）
type AISettings struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	Source  string `json:"source"` // "binding:*" | "user:*" | "custom" | "unbound" | "decrypt-failed"
}

// AIModelSelector 对话凭证选择子（Stream 路径解析输入）：调用方对「用哪个模型」的纯数据
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
// AISettings 的解析内聚在 *AIConfigService 单实现，端口两个方法按调用形态取用；
// 解析知识不泄出配置 service。
type AIConfigResolver interface {
	// ResolveFeatureSettings 阻塞补全流向：featureKey → 管理端单绑定凭证（空键/未绑定报错）。
	ResolveFeatureSettings(ctx context.Context, featureKey string) (AISettings, error)
	// ResolveChatSettings 流式流向：选择子 → 对话凭证（专项单绑定 → 双模式 → 旧来源）。
	ResolveChatSettings(ctx context.Context, sel AIModelSelector) (AISettings, error)
}

// AICompleteOptions Complete 的生成参数（对应原阻塞栈逐调用传入的 maxTokens/temperature；
// 消费方按语义给值：评分 1000/0.3、解析 800/0.5、章节生成 2000/0.5）。
type AICompleteOptions struct {
	MaxTokens   int
	Temperature float32
}

// AIModelPort 单一模型端口（ADR-0029 T2）：阻塞补全与流式回调合一，消费方只面对
// 一套消息类型（eino schema.Message）与一个端口。凭证解析、client 生命周期（签名缓存）、
// 超时与重试等深知识全部藏进 adapter 单点。*einoAIAdapter 为唯一生产 adapter，测试可注入 fake。
// 计量闸门（ADR-0031）作为装饰器挂在本端口上（meteredAIModel）：生产装配
// NewMeteredAIModel(NewEinoAIModel(...), meter)，消费方拿到的端口天然过闸。
type AIModelPort interface {
	// Complete 阻塞补全：服务评分/解析/章节内容生成等一次请求一次完整回复的调用。
	// featureKey 经注入 resolver 解析绑定配置；生命周期自持（不携带调用方 context，
	// 与原阻塞栈语义一致——章节生成在后台 goroutine 运行、评分不随请求中断）。
	Complete(featureKey string, msgs []*schema.Message, opts AICompleteOptions) (string, error)
	// Stream 流式调用：增量经 onChunk 回调透传，返回累积完整回复与计量产出
	// （*AIUsage，ADR-0031；免费/未登录/未扣费为 nil，内容错误时不扣费）。
	// 选择子经注入 resolver 解析凭证（调用方只传选择子）；沿用调用方 context（SSE
	// 随请求断连取消是既有语义），超时纪律在 adapter 内单点封顶。
	Stream(ctx context.Context, sel AIModelSelector, msgs []*schema.Message, onChunk func(string)) (string, *AIUsage, error)
}

// 超时纪律单点（ADR-0029 决策 3）：阻塞/流式按调用形态在此分化，不再各自为政。
const (
	// aiBlockingTimeout 阻塞补全总时长上限（评分/解析/章节生成等一次请求一次完整回复的调用）。
	aiBlockingTimeout = 120 * time.Second
	// aiStreamTimeout 流式总时长上限：比阻塞宽松（长对话），单点封顶。
	aiStreamTimeout = 300 * time.Second
)

// aiFinishReasonContentFilter 内容审查截断的 finish_reason（eino schema.ResponseMeta 透传）。
const aiFinishReasonContentFilter = "content_filter"

// aiBlockingContext 阻塞补全超时纪律包装单点（Complete 唯一消费）。
func aiBlockingContext() (context.Context, context.CancelFunc) {
	return withTimeout(aiBlockingTimeout)
}

// einoAIAdapter 单一生产 adapter（ADR-0029 T2）：eino ChatModel 同时承载 Generate 与 Stream。
// 凭证解析经注入 resolver；client 签名缓存收敛于此（阻塞/流式同一签名复用同一 client，
// 流式不再每请求重建——原两副面孔的单点归一）。
type einoAIAdapter struct {
	resolver AIConfigResolver
	logger   *zap.Logger

	mu        sync.Mutex            // 保护 client 重建并发安全
	client    *einoopenai.ChatModel // 当前签名的 eino client
	clientSig string                // "key|url|model" 签名，用于检测配置变化
}

// NewEinoAIModel 构建唯一生产 adapter（deps.go 构建一次，注入全部 AI 消费服务，
// 使阻塞/流式共享同一 client 缓存）。返回接口：caller 只依赖 AIModelPort，
// adapter 类型保持未导出（seam 收口在 port）。
func NewEinoAIModel(resolver AIConfigResolver, logger *zap.Logger) AIModelPort {
	return &einoAIAdapter{resolver: resolver, logger: logger}
}

var _ AIModelPort = (*einoAIAdapter)(nil)

// Complete 阻塞补全（AIModelPort 实现）：120s 超时纪律 → resolver 解析 → 签名缓存取 client
// → eino Generate，重试 2 次（错误文案与重试节奏与原阻塞栈逐字一致）。
func (a *einoAIAdapter) Complete(featureKey string, msgs []*schema.Message, opts AICompleteOptions) (string, error) {
	ctx, cancel := aiBlockingContext()
	defer cancel()

	mc, err := a.resolver.ResolveFeatureSettings(ctx, featureKey)
	if err != nil {
		return "", err
	}
	chatModel, err := a.ensureClient(ctx, mc, featureKey)
	if err != nil {
		return "", fmt.Errorf("构建模型失败: %w", err)
	}

	genOpts := []model.Option{
		model.WithMaxTokens(opts.MaxTokens),
		model.WithTemperature(opts.Temperature),
	}
	for attempt := 1; attempt <= 2; attempt++ {
		resp, err := chatModel.Generate(ctx, msgs, genOpts...)
		if err != nil {
			a.logger.Error("AI call failed", zap.Int("attempt", attempt), zap.Error(err))
			if attempt == 2 {
				return "", err
			}
			time.Sleep(time.Second)
			continue
		}
		content, finishReason := responseContent(resp)
		if content == "" {
			if finishReason == aiFinishReasonContentFilter {
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

// Stream 流式调用（AIModelPort 实现）：300s 超时纪律 → resolver 解析 → 签名缓存取 client
// → eino Stream → Recv 收集循环单点（错误文案与原流式栈逐字一致）。裸传输不产生计量产出
// （闸门在 meteredAIModel 装饰器内）。
func (a *einoAIAdapter) Stream(ctx context.Context, sel AIModelSelector, msgs []*schema.Message, onChunk func(string)) (string, *AIUsage, error) {
	ctx, cancel := context.WithTimeout(ctx, aiStreamTimeout)
	defer cancel()
	mc, err := a.resolver.ResolveChatSettings(ctx, sel)
	if err != nil {
		return "", nil, err
	}
	chatModel, err := a.ensureClient(ctx, mc, sel.FeatureKey)
	if err != nil {
		return "", nil, fmt.Errorf("构建模型失败: %w", err)
	}
	reader, err := chatModel.Stream(ctx, msgs)
	if err != nil {
		return "", nil, fmt.Errorf("调用模型失败: %w", err)
	}
	content, err := collectStreamReader(reader, onChunk)
	if err != nil {
		return content, nil, fmt.Errorf("流式接收失败: %w", err)
	}
	return content, nil, nil
}

// ensureClient 检查凭证签名是否变化，必要时重建 eino client（阻塞/流式共用，签名缓存单点）。
// 锁内调用 newEinoChatModel 的安全性前提：eino NewChatModel 是纯构造（装配 config，无 IO/
// 无拨号），持锁构建只串行化轻量对象创建；若未来该构建引入 IO，须先移出临界区。
func (a *einoAIAdapter) ensureClient(ctx context.Context, cur AISettings, featureKey string) (*einoopenai.ChatModel, error) {
	sig := cur.APIKey + "|" + cur.BaseURL + "|" + cur.Model

	a.mu.Lock()
	defer a.mu.Unlock()
	if sig == a.clientSig && a.client != nil {
		return a.client, nil
	}
	cli, err := newEinoChatModel(ctx, cur)
	if err != nil {
		return nil, err
	}
	a.client = cli
	a.clientSig = sig
	a.logger.Info("AI client 已重建", zap.String("base_url", cur.BaseURL), zap.String("model", cur.Model), zap.String("source", cur.Source), zap.String("feature", featureKey))
	return cli, nil
}

// responseContent 提取 eino Generate 响应的正文与 finish_reason（空响应防御）。
func responseContent(resp *schema.Message) (content, finishReason string) {
	if resp == nil {
		return "", ""
	}
	if resp.ResponseMeta != nil {
		finishReason = resp.ResponseMeta.FinishReason
	}
	return strings.TrimSpace(resp.Content), finishReason
}

// newEinoChatModel eino client 构建单点（adapter 签名缓存与 TestConfig 共用）。
func newEinoChatModel(ctx context.Context, mc AISettings) (*einoopenai.ChatModel, error) {
	return einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
		APIKey:  mc.APIKey,
		BaseURL: mc.BaseURL,
		Model:   mc.Model,
	})
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
