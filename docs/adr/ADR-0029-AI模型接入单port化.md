# ADR-0029: AI 模型接入单 port 化

- 状态：已接受（2026-09-05）
- 领域：AI 域 / 模型接入层（阻塞与流式双栈合一；AI 计费口径不变，见 ADR-0023）

## 背景

「可配置 OpenAI 兼容模型」的能力早已收敛为配置数据（`AISettings{APIKey, BaseURL, Model}`，provider 差异只是 BaseURL），但模型接入的 seam 却裂成两条平行浅栈：

- **阻塞栈** `AIBlockingTransport`：go-openai SDK、消息类型 `openai.ChatCompletionMessage`、120s 超时、`ensureClient` 带 `clientSig` 签名缓存、featureKey 作尾参在内部解析凭证；
- **流式栈** `AIStreamingTransport`：eino SDK、消息类型 `[]*schema.Message`、300s 超时、每请求重建 client、caller 先自行 `resolveModelConfig`（66 行：专项功能单绑定 → assistant Mode 双模式 → 兼容旧 ModelSource 的三重 switch）再传 `AISettings`。

同一件事（拿凭证）两种流向，连接管理两副面孔，caller 必须学「哪个功能走哪套」。消费方横跨计费面（助手对话）与免费面（刷题解析、简答评分、章节生成、会话自动命名），interface 与 implementation 一样复杂——shallow。

## 决策

1. **单一 `AIModelPort`**：`Complete(featureKey, msgs, opts)`（阻塞补全，内部流式收集为完整文本）+ `Stream(ctx, selector, msgs, onChunk)` 两个方法。Complete **不收调用方 ctx**、自持生命周期（内部 120s）——沿用原 CallModel 语义：全部阻塞消费方（评分/解析/章节生成）本无 ctx 可传导，章节生成跑在后台循环逐章容错，传 ctx 只会制造假可取消性；Stream 沿用调用方 ctx（SSE 断连取消是既有语义）+ 300s 封顶。流式侧收 `AIModelSelector` 而非字面 featureKey（Mode/用户自定义凭证是请求态数据，无法折叠进 featureKey——T1 落地时的裁决）。eino 作唯一生产 adapter（其 Generate 支持非流式），测试 fake 为第二个 adapter——两个 adapter 坐实 seam。
2. **ConfigResolver 注入**：featureKey → AISettings 的解析（专项功能单绑定 → assistant Mode 双模式 → 旧 ModelSource 兼容）内聚为一个注入的 resolver implementation，由 AIConfigService 提供；`ResolveAssistantPair` 降级阶梯仍是 AIConfigService 的知识，不泄进 port。
3. **纪律单点**：client 签名缓存迁入 adapter；超时口径（阻塞 120s / 流式 300s）按调用形态在 adapter 内分化，不再两套栈各自为政。
4. **分步迁移**：T1 统一 ConfigResolver 注入与超时纪律（两栈共用，独立有价值可单独合并）；T2 合并 port、迁移调用方、删除 go-openai 依赖与 `aiService` 的 `client/clientSig/mu` 三字段。每步独立 CI 全绿、可独立回滚。
5. **计费面不动**：`AIPreflight` / `DeductAI` 调用点与幂等键口径（ADR-0023）原样保留；本 ADR 只深化接入层，不动计量语义。

## 备选

- **反向统一到 go-openai（自写 SSE 解析）**：拒绝——把 eino 已隐藏的流式行为重新裸露，Go 侧自维护 SSE 解析/重连细节，前端 `api/aiAssistant.ts` 的手写解析不应在 Go 侧重演。
- **仅对齐纪律不合并 port**：拒绝——两套 SDK、两套消息类型依旧，caller 仍要学「哪个功能走哪套」，deletion test 不过。
- **一次性切换**：拒绝——AI 域是生产活跃区（唯一计费面 + 练习主链路），分步使风险分散、每步可回滚。

## 后果

- `go.mod` 删除 `sashabaranov/go-openai`；`ai_dual_stack_test.go` 改写为单 port 契约测试。
- caller 只学一套消息类型；新增 AI 消费功能只面对一个 port，T2 后接入成本为「一个 featureKey + 一组消息」。
- eino 成为 AI 域唯一允许的模型 SDK 入口（生产 adapter + 测试 fake 两个 adapter 位）。

## 相关

- ADR：`ADR-0023-积分簿记收敛与幂等占坑`（AI 计费幂等口径）、`ADR-0007`（统一 zap 日志基础设施）
- 代码：`backend/internal/service/ai_transport.go`、`backend/internal/service/ai_service.go`、`backend/internal/service/ai_assistant_service.go`
- 来源：architecture review（2026-09-05，十一候选评审）候选 2 决策记录
