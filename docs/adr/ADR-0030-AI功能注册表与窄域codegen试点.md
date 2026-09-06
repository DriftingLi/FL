# ADR-0030: AI 功能注册表与窄域 codegen 试点

- 状态：已接受（2026-09-05）
- 领域：AI 域 / 专项功能声明（后端注册表与前端配置的一致性；承接 ADR-0029 的接入层收敛）

## 背景

新增一个 AI 专项功能需要同步 5 处编辑点：功能常量、`AllAIFeatures` 切片、`FeatureLabel` switch、`featureChatKeys` map、`featureSystemPrompt` switch，散在 `ai_config_service.go` 与 `ai_assistant_service.go` 两个文件——漏改任何一处只在中期暴露（如 system prompt 静默落到通用兜底）。同时，前端 `frontend/src/config/aiFeatures.ts` 手工镜像后端功能集，两端各自维护。哪些功能计费、哪些免费目前是隐式惯例，没有声明位。

`ADR-0019` 曾推迟前端契约 codegen（输入 OpenAPI、范围整个 API 层），但明确了方向：下一次前端契约专项以 gin-swagger 注解为唯一输入做 codegen。

## 决策

1. **后端单张 `aiFeature` 注册表**：`{name, label, systemPrompt, bindingKind, billed}`，`AllAIFeatures` / `FeatureLabel` / `featureChatKeys` / `featureSystemPrompt` 四个导出面全部派生；新增功能 = 一行注册。`bindingKind` 落地为三值：`admin-single`（单绑定阻塞型）/ `assistant-mode`（助手对话模式绑定）/ `assistant-legacy`（遗留 `ai_assistant` 多绑定回退位——保持绑定有效与 label，但不进 AllAIFeatures 展示）。`featureChatKeys` 派生规则 = `admin-single ∧ billed`（会话键与计费声明互锁，一致性测试钉住）；自动命名无独立功能键，计费归属随其所属对话。
2. **`billed` 立为声明位**：本 ADR 落地时即声明每个功能的计费口径（助手对话 billed=true，其余免费）；闸门接线（预检/扣费由 module 内触发）另见 AI 计量闸门决策，不在本 ADR 范围。
3. **前端 `aiFeatures.ts` 走构建期窄域生成**：构建脚本读后端注册表（导出结构化常量）生成该文件（文件头标注生成勿改），契约测试保留、校验生成物与后端一致。
4. **与 ADR-0019 的关系**：本决策是 ADR-0019 契约专项的**窄域先行试点**——生成管道可被未来契约专项复用；ADR-0019 的推迟决策不重开，全量 API 层 codegen 仍等专项。

## 备选

- **前端常量表 + 契约测试**：最初推荐——两端各自单点、零工具链投资；用户决策选构建期 codegen，理由是消除人工同步而非捕获漂移。
- **API 运行时下发**（GET /ai-assistant/features）：拒绝——前端路由与文案仍需本地兜底，引入动态加载时序与降级复杂度。
- **仅物理归拢 5 处不派生**：拒绝——「漏一处中期才暴露」的本质不变。

## 后果

- 新增 AI 专项功能的后端成本 = 一行注册 + 一个 bindingKind；前端 `aiFeatures.ts` 不再手改。
- 仓库新增一个窄域生成脚本（Go 注册表 → TS 配置），CI 中生成物与注册表不一致时契约测试红。
- 未来契约专项可扩展同一管道（输入从注册表扩至 OpenAPI spec），避免第二套生成机制。

## 相关

- ADR：`ADR-0019-前端契约codegen推迟`（本决策是其窄域先行试点）、`ADR-0029-AI模型接入单port化`
- 代码：`backend/internal/service/ai_config_service.go`、`backend/internal/service/ai_assistant_service.go`、`frontend/src/config/aiFeatures.ts`
- 来源：architecture review（2026-09-05，十一候选评审）候选 5 决策记录
