# ADR-0019: 前端契约 codegen 推迟

- 状态：已接受（2026-08-22）
- 领域：前端契约 / DTO 单一事实源（学员端 SPA 与跨端 App 双应用）

## 背景

架构评审（2026-08-22，`/tmp/architecture-review-20260822-162113.html` candidate 4）发现同一后端 DTO 在两个前端应用里手写至少三份且已漂移出真实矛盾——`ForumTopic` 计数字段名颠倒（`views_count/replies_count` vs 后端 `view_count/reply_count`，同一 App 内 `types` 文件与其 `api` adapter 互斥）、`Question`/`SubmitResult` 结构在 `training-app` 与 `frontend` 间字段名与形态全不同（`is_correct` vs `correct`、解析五模块字段缺失）、`training-app` 的 `api` 层以注释自称「推断」并用 `extract*` 猜测兼容。

`gin-swagger` 已在 #282 迁至 `www` 子域并以 `BasicAuth` 暴露，后端注解可作为 OpenAPI spec 的单一来源，具备 `codegen` 的技术前提。

同期第八波深化（spec #294）的 5 个后端 module 收敛（答题会话 / 会话 / 论坛计数 / AI 解析 / 打卡）已占满本迭代改动面与发布风险预算；`training-app` 侧 `ADR-0018` 互动能力与契约对齐本身仍在滞后状态，双应用同时改契约层会放大联调成本。

## 决策

- **本次不做**由 OpenAPI `spec` 生成前端类型的单一事实源收敛（`training-app` 的类型滞后、影子判分、单令牌等问题亦一并推迟）。
- 后端 `DTO` 仍以手写契约为事实源；`frontend` / `training-app` 各自维护的副本矛盾保持现状，直到下一次前端契约专项再一次性收敛。

## 后果

- 后端 `DTO` 改动仍需人工同步三处，手写漂移风险持续存在；但本波后端收敛先行，不放大发布面。
- 下一次前端契约专项时以 `gin-swagger` 注解为唯一输入做 `codegen`，两个应用的 `API` 层降为薄 `adapter`。

## 相关

- 架构评审 `candidate 4`（`Worth exploring`）
- spec #294 `Out of Scope` 首条
- `gin-swagger` 迁移 #282 / `docs/docs/reference/微信小程序登录-文档说明.md` 等契约文档

## 专项第一步（2026-09-13，spec #940 片五）：先修输入面，再试点一个域

「下一次前端契约专项」的启动条件在本次复核时已经具备一半：**生成管道**有 ADR-0030 / ADR-0047 §1 §5 三条先例（声明表 → 渲染 → 字节级同步契约），**输入面**却不可信 —— `backend/docs/swagger.json` 停在 #531，ADR-0044 / ADR-0045 两个功能波的字段一个都没有，且没有新鲜度门禁。故本片按「先输入面、后试点」推进：

1. **输入面可信化**：按钉住的 swag 版本（v1.16.4）再生成产物，并在 CI 的 backend-lint 加**新鲜度锁**（再生成后工作树必须干净）。产物从 #531 一次追平到当日注解。
2. **覆盖审计（结论：缺口跨多域，立后续片）**：注解侧 197 个端点里，**只有 54 个在 `@Success` 里指认了 data DTO**，其余 143 个仍是裸 `{object} response.R "success"`；与手写契约文档 `API.md` 的端点差集另有 30 处（swagger 有而 API.md 无，集中在招聘 / 积分 / 真题 / 打卡 / 证件这几波的增量）。缺口不在一个域里 —— 按 spec #940 片五② 的约定，补齐**不在本片硬塞**，作为后续片（清单见 spec 的验收证据）。
3. **单域试点**：新增 `internal/apitypes`（域声明表 + 渲染器）与 `cmd/gen-apitypes`，把**打卡域**三个端点的响应类型从注解生成到 `frontend/src/api/generated/checkin.ts`，原手写 API 层降为薄 adapter（只留请求壳与端点装配）；同步契约由 `internal/apitypes/codegen_test.go` 字节级钉住。
4. **可空性进注解层**：注解层原本不表达 Go 指针语义，试点把 `x-nullable` 扩展落到 `CheckInRankResult.Me`，生成物据此渲染 `T | null` —— 这是「要精确类型就先把事实写进注解」的第一个样例，后续域照此补齐。swag 对 `map[string]int64` 的 `format` 推断不稳定（同一份代码在不同环境产物不同），会让新鲜度锁变成随机红，故对这类字段用 `swaggertype` 钉住值类型。
5. **手写契约文档的定位（本片定案）**：根 `API.md` 保留为**人类可读叙述面**（业务语义、示例、调用顺序），字段级契约以注解产物为准；文档顶部已写明这条优先级与冲突处置。**移动端契约**由移动端自己的 ADR 体系决定，不在本专项内（AGENTS.md 架构评审范围）。
6. **审计可复现**：覆盖审计固化为 `node scripts/audit-api-annotations.mjs`（只读，输出「未指认 data DTO / swagger 有而 API.md 无 / API.md 有而 swagger 无」三段清单），审计结论与差集清单即本片 PR 的验收证据。

**本 ADR 的推迟结论不重开**：全量 API 层 codegen 仍未开工；片五只交付「可信输入面 + 一个可复制的试点」。
