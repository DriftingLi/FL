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
