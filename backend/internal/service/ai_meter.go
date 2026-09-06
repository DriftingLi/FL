// Package service AI 计量闸门（ADR-0031，#619）：AIMeter adapter 挂在 AIModelPort 上，
// 所有经端口的 LLM 消费过同一道闸——是否计费由功能注册表 billed 声明驱动（ADR-0030），
// 「传什么事实」（promptChars=最后一条用户消息长度）与请求标识降级键生成内移进本文件，
// HTTP 层不再持有计费编排。计费金额口径单点仍在积分域（estimateAITokens / aiPointsForTokens /
// AIPreflight / DeductAI），本文件零口径知识、零常量。
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// AIMetering 计量闸门 seam（ADR-0031 决策 1）：生产实现 = 积分域 *PointsService
// （AIPreflight/DeductAI 方法签名原样满足——余额预检与扣费下限同源，不另立实现），
// 测试 fake 为第二实现。meter 只认这两个动作，金额换算与幂等全部留在积分域。
type AIMetering interface {
	AIPreflight(userID int) error
	DeductAI(ctx context.Context, userID int, requestID string, promptChars, completionChars int) (*AITokensResult, error)
}

// 积分域实现即生产 meter：签名同源，零适配代码。
var _ AIMetering = (*PointsService)(nil)

// AIUsage 单次经闸调用的计量产出（仅计费调用非 nil）。
// Res = 扣费数据面（即 SSE usage 事件负载，json 形状与迁移前逐字节一致，移动端契约无感）；
// Err = 扣费失败（ErrInsufficientPoints → 调用方映射既有「积分不足」文案；其余错误沿用
// 迁移前行为：静默跳过 usage 事件）。内容错误与扣费失败互斥（内容失败不扣费）。
type AIUsage struct {
	Res *AITokensResult
	Err error
}

// ---- 请求级透传通道（handler → service → port，meter 消费）----

// aiRequestIDCtxKey ctx 键：稳定请求标识（幂等键事实）。
type aiRequestIDCtxKey struct{}

// aiMeterFreeCtxKey ctx 键：显式免费声明（内部二次消费）。
type aiMeterFreeCtxKey struct{}

// WithAIRequestID 透传请求标识（handler 从 RequestID 中间件取值后注入调用 ctx）；
// meter 取之作幂等键事实，缺失时由 meter 生成降级键（aiFallbackRequestID）。
func WithAIRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, aiRequestIDCtxKey{}, requestID)
}

// aiRequestIDFrom 读取透传的请求标识。
func aiRequestIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(aiRequestIDCtxKey{}).(string)
	return v
}

// WithAIMeterFree 显式声明本次 port 调用免费（ADR-0031 决策 2：免费消费显式声明）。
// 供 service 内部追加的第二次 port 调用使用（会话自动命名：无独立功能键，若随注册表
// 默认按其所属对话 billed=true 处理会与主对话双扣）。这是注册表 billed 声明之外唯一的
// 免费通道，声明在调用点可见可审计；调用方无法借它把注册表 billed=false 的功能改为计费。
func WithAIMeterFree(ctx context.Context) context.Context {
	return context.WithValue(ctx, aiMeterFreeCtxKey{}, true)
}

// aiMeterFreeDeclared 读取显式免费声明。
func aiMeterFreeDeclared(ctx context.Context) bool {
	v, _ := ctx.Value(aiMeterFreeCtxKey{}).(bool)
	return v
}

// ---- billed 判定路径（调用方声明 > 注册表默认）----

// aiStreamBilledDefault 流式调用的 billed 注册表默认（ADR-0031 决策 2）：
// 对话形态的注册行（双模式 / 遗留兼容位 / 专项聊天 = featureChatKeys）以其 billed 声明为准；
// 空键 / 未知键经凭证解析阶梯回退为通用对话，billed=false 的阻塞功能键（评分/章节生成/解析）
// 借键发起的对话同样回退为通用对话——均按通用对话计费（CONTEXT.md「AI 计费」：仅助手对话
// 计费），防止借免费功能键逃费。迁移时全部对话行 billed=true，故本函数现值为恒真，
// 结构上保留注册表驱动：产品决策把某对话行改为 billed=false 时闸门即跟随。
func aiStreamBilledDefault(featureKey string) bool {
	f, ok := lookupAIFeature(aiFeatureRegistry, featureKey)
	if ok && (f.bindingKind == bindingAssistantMode || f.bindingKind == bindingAssistantLegacy || featureChatKeys[featureKey]) {
		return f.billed
	}
	return true
}

// aiCompleteBilled 阻塞补全的 billed 注册表默认：按 featureKey 查声明。
// 未注册键属注册遗漏，按免费放行不阻断业务，由调用方记告警日志暴露。
func aiCompleteBilled(featureKey string) bool {
	f, ok := lookupAIFeature(aiFeatureRegistry, featureKey)
	if !ok {
		return false
	}
	return f.billed
}

// ---- 计费口径事实单点（与迁移前 handler 逐字同口径）----

// aiPromptChars 「什么算 prompt」的唯一实现（ADR-0031 决策 3）：只算最后一条用户消息的
// 文本长度，system 提示词、历史消息与图片一律不计。多模态消息的请求原文在首个文本 part
// （Content 为空），纯图片消息计 0。
func aiPromptChars(msgs []*schema.Message) int {
	for i := len(msgs) - 1; i >= 0; i-- {
		msg := msgs[i]
		if msg == nil || msg.Role != schema.User {
			continue
		}
		if msg.Content != "" {
			return len(msg.Content)
		}
		for _, part := range msg.UserInputMultiContent {
			if part.Type == schema.ChatMessagePartTypeText {
				return len(part.Text)
			}
		}
		return 0
	}
	return 0
}

// aiFallbackRequestID 请求标识降级键（迁移前 handler 现场生成策略内移，格式逐字不变）：
// 中间件未覆盖时回退现场生成，仅丧失重试幂等。
func aiFallbackRequestID(userID int) string {
	return fmt.Sprintf("ai-%d-%d", userID, time.Now().UnixNano())
}

// ---- metered adapter：闸门即端口装饰器 ----

// meteredAIModel 计量闸门 adapter（ADR-0031 决策 1）：包装任一 AIModelPort，所有经端口的
// 消费先过闸再传输。生产装配在 deps.go 单点：NewMeteredAIModel(NewEinoAIModel(...), pointsSvc)，
// 不依赖调用方自觉；测试注入 fake inner + fake meter。
type meteredAIModel struct {
	next   AIModelPort
	meter  AIMetering
	logger *zap.Logger
}

// NewMeteredAIModel 构建计量闸门端口。next 为裸传输 adapter（NewEinoAIModel 产物），
// meter 为积分域实现（生产 = *PointsService），均必须非 nil：构造期注入是不变量。
func NewMeteredAIModel(next AIModelPort, meter AIMetering, logger *zap.Logger) AIModelPort {
	return &meteredAIModel{next: next, meter: meter, logger: logger}
}

var _ AIModelPort = (*meteredAIModel)(nil)

// Complete 阻塞补全过闸：注册表按 featureKey 查 billed；billed=false（评分/章节生成/解析，
// 现状全部阻塞消费）直接放行。billed=true 而端口签名又无计费主体（user/requestID）时，
// 显式报错拒绝静默免费——迫使新计费功能先给端口补主体，堵住第二个漏网消费点。
func (m *meteredAIModel) Complete(featureKey string, msgs []*schema.Message, opts AICompleteOptions) (string, error) {
	if !aiCompleteBilled(featureKey) {
		if _, ok := lookupAIFeature(aiFeatureRegistry, featureKey); !ok {
			m.logger.Warn("阻塞补全使用未注册功能键，按免费放行（请核对功能注册表）", zap.String("feature", featureKey))
		}
		return m.next.Complete(featureKey, msgs, opts)
	}
	return "", fmt.Errorf("AI 功能 %q 声明计费但阻塞补全无计费主体，拒绝静默免费（见 ADR-0031）", featureKey)
}

// Stream 流式调用过闸：预检（发起前）→ 裸传输 → 扣费（成功后）。billed 判定路径 =
// ctx 显式免费声明（内部二次消费）> 注册表 billed 默认；计费主体与消费事实口径见
// aiPromptChars / aiFallbackRequestID。usage 事件数据面原样回传，形状不变。
func (m *meteredAIModel) Stream(ctx context.Context, sel AIModelSelector, msgs []*schema.Message, onChunk func(string)) (string, *AIUsage, error) {
	billed := !aiMeterFreeDeclared(ctx) && aiStreamBilledDefault(sel.FeatureKey)

	// 余额预检（迁移前 handler 原位语义）：billed 且已登录才预检，不足即阻断且不发起传输
	if billed && sel.UserID > 0 {
		if err := m.meter.AIPreflight(sel.UserID); err != nil {
			return "", nil, err
		}
	}

	content, _, err := m.next.Stream(ctx, sel, msgs, onChunk)
	if err != nil {
		return content, nil, err
	}
	// 未登录（游客）/ 免费声明 / 空回复不产生扣费（与迁移前 handler 条件逐字一致）
	if !billed || sel.UserID <= 0 || content == "" {
		return content, nil, nil
	}

	requestID := aiRequestIDFrom(ctx)
	if requestID == "" {
		requestID = aiFallbackRequestID(sel.UserID)
	}
	res, err := m.meter.DeductAI(ctx, sel.UserID, requestID, aiPromptChars(msgs), len(content))
	if err != nil {
		return content, &AIUsage{Err: err}, nil
	}
	return content, &AIUsage{Res: res}, nil
}
