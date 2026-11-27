# ADR-0109：ReleaseEvidence 以双成功为唯一绿判据

- 状态：已接受
- 日期：2026-11-27
- 领域：CI/CD / 发布证据 / 合并门禁

## 背景

`checkDeployment` 与 `gateVerdict` 各自实现查询和轮询；前者在 `TIMEOUT=0` 时把 deployment 记录存在视为绿，后者要求 deployment `success`。同一 ReleaseEvidence 因此对 testing 与 production 给出不同 verdict，且已有失败部署可能因“记录存在”被放行。

## 决策

1. **green 的唯一判据是 SHA 对齐的 run `success` 且最新一次 deployment `success`**。记录存在只是中间状态。
2. **同一 SHA 多条 deployment 取最新一次**，读取其最新状态；旧记录保留审计，但不覆盖最新结果。
3. **`failure` / `error` / `cancelled` 立即 red**；`pending` / `in_progress` 继续轮询；超时、无记录或查询失败最终 fail closed。
4. **ReleaseEvidence 是一个 verdict module**。workflow 只提供 SHA、run、environment 与时限，不自行轮询或解释状态。

## 后果

- testing 检查与 production gate 共享同一证据语义，消除“记录存在即绿”的旁路。
- 测试需覆盖 composite action 与 workflow 的真实组合，以及已有 failure/cancelled 记录、最新部署替换旧记录等场景。
- 轮询实现集中后，`waitForRun` / `checkDeployment` / `gateVerdict` 应作为同一 module 的内部步骤或薄 adapter，不再各自拥有状态机。

## 关联

- `CONTEXT.md`：ReleaseEvidence
- ADR-0108：发布计划与回滚生命周期
- ADR-0047：部署事实源与 codegen 约束
