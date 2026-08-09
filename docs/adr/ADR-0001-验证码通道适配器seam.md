# ADR-0001: 验证码通道适配器 seam

- 状态：已接受（2026-08-05，refactor/architecture-deepening）
- 领域：账号与认证 —— 验证码（code）

## 背景

邮箱验证码（email_auth_service.go）与手机验证码（phone_auth_service.go）曾是两个独立 module：sendCode / verifyCode / 注册流程逐行孪生，唯一差异是 key 前缀、账号查询与发送通道。节流、尝试上限、绑定流的任何修改都要动两处以上。

## 决策

验证码状态机收敛为单一 module（`service.VerifyCodeService`），邮箱与短信降级为 `CodeChannel` adapter：

- 状态机（generate → store → throttle → verify → register/bind）只有一份；
- 通道差异（归一化、账号唯一性查询、文案、发送）收敛到 `EmailChannel` / `SmsChannel`；
- 生产未配置通道时发送器为 nil，`SenderReady` 最先拦截且不落任何存储。

## 约束

- Redis key 前缀（`email_code:*` / `phone_code:*`）与 `authCodeValue` JSON 结构保持不变，线上存量验证码兼容（`TestCodeKey_CompatibleWithLegacy` 锁定）。

## 后果

- 改节流/尝试上限只需动一处；新增通道（如 TTS）只需实现 `CodeChannel`。
- 测试一套路径覆盖全部通道。
- 代价：`CodeChannel` 接口有 9 个方法，接口面略宽，但每个通道实现仍是声明式的。

## 相关

- `backend/internal/service/code_service.go`
- CONTEXT.md「验证码」「验证码通道（channel）」词条
