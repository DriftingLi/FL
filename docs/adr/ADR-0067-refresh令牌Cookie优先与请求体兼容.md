# ADR-0067: refresh 令牌 Cookie 优先 + 请求体兼容（修订 ADR-0016）

- 状态：已接受（2026-09-28，真实缺陷评审 grilling 定案）；**实施另立**，不随本轮 XSS 修复同期
- 领域：账号与认证 —— 会话（session）/ 令牌存放
- 修订：ADR-0016 的两条口径——「前端 `storage.ts` 双 key、refresh 由前端持有」与「Cookie 通道只带 access」

## 背景

ADR-0016 决定双令牌并明确 refresh 由前端持有（localStorage）。这意味着**任何**同源脚本执行都能在 7 天窗口内取走 refresh 并轮换出可用令牌对——令牌的价值不取决于它多难猜，而取决于它可不可被脚本读到。本轮修掉的是唯一已知注入源（ADR-0066），但「下一个 XSS」无法用白名单排除；把长效凭证移出脚本可达范围，是唯一能降低**后果严重度**的一层。

## 决策

1. **浏览器侧 refresh 进 httpOnly Cookie**：`/api/auth/refresh` **优先**读 Cookie；请求体通道**保留**（移动端与非浏览器客户端继续用，避免跨端断供）。
2. **CSRF 口径一并定死**：Cookie 只用于该一个端点；`SameSite=Lax`（更严则 Strict）+ 仅同站来源；不允许跨站 CORS 触发刷新。刷新响应体仍返回新 access + 新 refresh（body 通道客户端需要）。
3. **access 仍由前端持有**：本轮不动 access 的存放（窗口 2h，且请求头携带是其唯一用法）；未来若要收敛，按同一 ADR 修订。
4. **实施独立立项**：落地涉及三端 + 移动端回归，不并入本轮缺陷修复的验收面。

## 被否备选

- **不做，显式接受残留风险**：成本最低，但「XSS ⇒ 7 天凭证失窃」这条后果链完整保留，且没有任何一处文档写下「这是被接受的」。
- **只走 Cookie、砍掉请求体通道**：语义最干净，但立刻打断移动端（另一负责人 / 另一套 ADR），代价需跨端排期。
- **不搬 Cookie，只缩短 refresh 生命周期**：治窗口长度不治可达性，且降级「7 天免登录」这一已上线体验。

## 相关

- ADR-0016（被本 ADR 修订）；ADR-0066（XSS 的注入源层）
- `backend/internal/api/auth.go`（/refresh、/logout）、`frontend/src/api/client.ts`、`frontend/src/utils/storage.ts`
