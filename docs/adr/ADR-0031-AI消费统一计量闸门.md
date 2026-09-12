# ADR-0031: AI 消费统一计量闸门

- 状态：已接受（2026-09-05）
- 领域：AI 域 / 计量治理（所有 LLM 消费过同一道闸；承接 ADR-0029 单 port、ADR-0030 billed 声明）

## 背景

AI 计费的金额口径单点收敛在积分域（`estimateAITokens` / `aiPointsForTokens` / `AIPreflight` / `DeductAI`，ADR-0023），生产调用点也仅助手对话一处。但「传什么事实」的决策住在 handler：`promptChars = len(最后一条用户消息.Content)`（system prompt、历史消息、图片均不计）、requestID 丢失时的降级键生成策略也在 handler。同时 `maybeGenerateSessionTitle`（会话自动命名）在 service 内部用真凭证直接调 `StreamComplete`，零计量、零审计——消费事实产生点（service）与计费决策点（handler）分离，第二个消费点已经漏网。CONTEXT.md「AI 计费」的免费清单也未收录会话自动命名。

## 决策

1. **AIMeter adapter 挂在 AIModelPort（ADR-0029）**：生产 adapter = 积分域实现（复用 `AIPreflight` / `DeductAI`，余额预检与扣费下限同源，不另立常量），测试 fake 为第二个 adapter。所有 port 消费过同一道闸，不依赖调用方自觉。
2. **billed 声明驱动（ADR-0030 注册表）**：`billed=true` 过预检+扣费；`billed=false` 显式免费。会话自动命名登记为 `billed=false`，从隐式漏网转为声明；免费消费面 = 注册表 `billed=false` 清单，即 CONTEXT.md「AI 计费」词条的机器可读版本。
3. **口径不变等价迁移**：meter 输入事实与现 handler 完全一致（promptChars=最后一条用户消息长度；tokens 按 /4 估算；ceil(tokens/1000)×10，下限 5 上限 100）；迁移 PR 以「计费金额 diff=0」验证，用户侧无感。
4. **改计费口径属产品决策**：把 system/历史/图片计入 prompt 等口径变化单独立项（与「扩计费面需单独立项」同理），不随本次架构迁移夹带。
5. requestID 降级键生成随口径内移进 meter，handler 只负责透传请求标识。

## 备选

- **meter 挂 AIAssistantService**：拒绝——覆盖面窄，AIService 侧消费（章节生成等）不经闸，下一消费点继续漏。
- **handler 保留编排、只堵标题生成漏网**：拒绝——口径住 HTTP 层的本质不变，第二个对话面出现时必然复制。
- **顺带把 system/历史计入 prompt**：拒绝——用户成本上涨属产品决策，与架构迁移分离。

## 后果

- `api/ai_assistant.go` 的预检/扣费编排段删除；「什么算 prompt」从 HTTP 层消失。
- 所有 LLM 消费（含自动命名）经闸门可审计；新增消费点默认过闸，免费需显式声明。
- 移动端与 Web 契约无感（SSE `usage` 事件与扣费结果不变）。

## 相关

- ADR：`ADR-0023-积分簿记收敛与幂等占坑`（幂等键与金额口径）、`ADR-0029-AI模型接入单port化`（port 位置）、`ADR-0030-AI功能注册表与窄域codegen试点`（billed 声明）
- 代码：`backend/internal/service/ai_transport.go`、`backend/internal/service/ai_assistant_service.go`、`backend/internal/api/ai_assistant.go`、`backend/internal/service/points_service.go`
- 来源：architecture review（2026-09-05，十一候选评审）候选 9 决策记录
