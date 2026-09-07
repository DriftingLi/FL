# ADR-0032: 外部诊断 RAG 服务作为专项功能传输通道

- 状态：已接受（2026-09-07）
- 领域：AI 域 / 外部服务接入（第二个 AIModelPort 生产 adapter + 功能下线 + 公网入口收口；承接 ADR-0029 单 port、ADR-0030 注册表、ADR-0031 计量闸门）

## 背景

交付包 forklift-assistant（FastAPI + bge-m3 本地向量检索 + LLM，部署 pve-01 lxc101）提供了工业叉车智能维修诊断能力：文本对话 /chat 与图片诊断 /chat/with-image（阻塞 JSON，**不支持 SSE**）、品牌/车型两级目录、故障码知识库（精确命中秒回）、手册静态资源。2026-09-05 之前该包以独立公网子域 assistant.gccsmile.com 由 nginx 直接反代（自建 Basic Auth 保护运维接口）。现在要把该能力接入本项目学员端：作为 AI 功能注册表新专项功能，经 FL 后端代理（JWT + 计量闸门 + 限免声明位），取代既有两个纯 prompt 故障功能（fault_consult / fault_code_query），并收口公网入口。

## 决策

1. **第二个 AIModelPort 生产 adapter（diagnosisAssistantAdapter）**：调用入口沿用 ADR-0029 单一模型端口——Stream 把助手的阻塞 JSON 响应按段落切块伪流式转译为 onChunk 回调；图片从多模态 part 还原字节 re-post multipart；FL 持久化会话轮次转译为助手 chat_history（with-image 端点为 JSON 字符串，契约坑已钉）。凭证不入 AIConfigResolver（baseURL 来自 `DIAGNOSIS_ASSISTANT_URL` 注入），routing adapter 按 FeatureKey 分发，eino 路径零改动；计量闸门仍在最外层单点。
2. **freePreview 限免声明位**：注册表加 `freePreview`，`aiFeatureChatBilled` 随 `&& !freePreview` 放行——限免期免预付免扣费，结束限免 = 翻声明位 + 再生成（前端限免角标随 codegen 透出）。扩展计费面仍需单独立项的产品口径不变。
3. **下线纯 prompt 故障功能（fault_consult / fault_code_query）**：被同一用户入口（智能维修诊断）取代——故障码查询由知识库精确命中秒回（不走 LLM），故障咨询由 RAG 手册检索回答，纯 prompt 形态无保留价值。注册表删行 + 存量绑定行迁移清理；历史会话行保留（不再展示，不炸 UI）。
4. **公网入口收口**：删除 nginx assistant 子域 server 块与 compose 挂载。运维面（Swagger docs / kb 管理 API）改经 PVE 内网直连或 SSH 隧道；业务入口只经 FL 学员端（OptionalAuth + 可选 JWT）。理由：双公网面并存（assistant.gccsmile.com 与 training 学员端）扩大攻击面且运维负担翻倍；收口后所有流量过同一密钥体系。
5. **周边只读代理**：`/api/ai-assistant/diagnosis/{brands,models,fault-codes,manual/*}` 由后端直连 lxc101（绕过公网 nginx Basic Auth 层），鉴权与 chat 一致（OptionalAuth）；手册子路径白名单做强校验防 SSRF。

## 备选

- **不接入、保留独立公网子域**：拒绝——学员端作为唯一产品入口，双面并存维护两套 UI 与鉴权；包前端无法复刻 FL 的登录/积分/限免体系。
- **把 RAG 能力搬进本项目后端**：拒绝——交付包以镜像交付（2.4GB CPU 镜像 + SQLite 向量库），迁移成本高，且知识库更新由供应商维护；以外部服务契约接入保持解耦。
- **改助手支持 SSE**：拒绝——助手为交付物不便改动；阻塞 JSON 伪流式对用户侧打字机效果等价（整包 18s 到达后按段切块）。
- **计费照常运行（不限免）**：拒绝——新产品引导期限免是产品决策，freePreview 声明位让结束限免 = 一行翻转。

## 后果

- 模型端口出现第二个生产 adapter，routing adapter 成为分发点：新增外部服务 = 新 adapter + 注册表行 + routing 分支（保持单端口语义，消费方无感）。
- SSE 契约新增 `sources` 事件（answer_sources 透传），移动端未来批次需同步解析；历史消息不持久化 sources，回看不显示（与 usage 脚注同策）。
- 限免期成本敞口：学员用量无上限直烧外部 LLM key；现有全局限流 middleware 继续生效，每用户每日额度作为 fast-follow（积分体系现成）。
- 公网子域下线后，DNS 记录与服务器静态目录/凭据清理属运维动作；外部服务单点（lxc101）不可用时前端表现为 SSE error 友好提示（与既有外部调用先例一致，不做熔断）。

## 相关

- ADR：`ADR-0029-AI模型接入单port化`（端口单一化，本 ADR 是其第二实现接入点）、`ADR-0030-AI功能注册表与窄域codegen试点`（功能键 / freePreview 透出）、`ADR-0031-AI消费统一计量闸门`（billed 判定与限免放行）
- 代码：`backend/internal/service/ai_diagnosis_adapter.go`、`backend/internal/service/ai_diagnosis_proxy.go`、`backend/internal/service/ai_feature_registry.go`、`backend/internal/api/diagnosis.go`、`frontend/nginx-host.conf`、`docker-compose.prod.yml`
- 迁移：`backend/migrations/000022_retire_fault_ai_features.up.sql`
- 契约事实：forklift-assistant 交付包 openapi 实测（2026-09-05），契约样本见 `.scratch/`