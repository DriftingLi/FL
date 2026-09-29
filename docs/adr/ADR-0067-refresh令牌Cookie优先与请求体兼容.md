# ADR-0067: refresh 令牌 Cookie 优先 + 请求体兼容（修订 ADR-0016）

- 状态：已接受（2026-09-28，真实缺陷评审 grilling 定案）；**实施另立**，不随本轮 XSS 修复同期
- 领域：账号与认证 —— 会话（session）/ 令牌存放
- 修订：ADR-0016 的两条口径——「前端 `storage.ts` 双 key、refresh 由前端持有」与「Cookie 通道只带 access」

## 背景

ADR-0016 决定双令牌并明确 refresh 由前端持有（localStorage）。这意味着**任何**同源脚本执行都能在 7 天窗口内取走 refresh 并轮换出可用令牌对——令牌的价值不取决于它多难猜，而取决于它可不可被脚本读到。本轮修掉的是唯一已知注入源（ADR-0066），但「下一个 XSS」无法用白名单排除；把长效凭证移出脚本可达范围，是唯一能降低**后果严重度**的一层。

## 决策

1. **浏览器侧 refresh 进 httpOnly Cookie**：`/api/auth/refresh` **优先**读 Cookie；请求体通道**保留**（移动端与非浏览器客户端继续用，避免跨端断供）。
2. **CSRF 口径一并定死**：Cookie 只用于认证族那两个端点（`Path=/api/auth`，覆盖 `/api/auth/refresh` 与 `/api/auth/logout`；原口径「只用于该一个端点」在实施当日被推翻，理由见「实施回记」）；`SameSite=Lax`（更严则 Strict）+ 仅同站来源；不允许跨站 CORS 触发刷新。刷新响应体仍返回新 access + 新 refresh（body 通道客户端需要）。
3. **access 仍由前端持有**：本轮不动 access 的存放（窗口 2h，且请求头携带是其唯一用法）；未来若要收敛，按同一 ADR 修订。
4. **实施独立立项**：落地涉及三端 + 移动端回归，不并入本轮缺陷修复的验收面。

## 被否备选

- **不做，显式接受残留风险**：成本最低，但「XSS ⇒ 7 天凭证失窃」这条后果链完整保留，且没有任何一处文档写下「这是被接受的」。
- **只走 Cookie、砍掉请求体通道**：语义最干净，但立刻打断移动端（另一负责人 / 另一套 ADR），代价需跨端排期。
- **不搬 Cookie，只缩短 refresh 生命周期**：治窗口长度不治可达性，且降级「7 天免登录」这一已上线体验。

## 实施回记

- **`Path` 从「单个端点」放宽到认证族前缀 `/api/auth`（2026-09-29，实施当日）**。原口径站不住的原因不是安全性，而是它会静默废掉一条已上线的产品语义：Path 收在 `/api/auth/refresh` 时浏览器不会把这枚 Cookie 发到 `/api/auth/logout`，而自本 ADR 起前端不再从存储取 refresh ⇒ 登出退化成「只清本地」，`CONTEXT.md`「会话」词条的**单会话终止（手上这一枚 refresh 失效）**不再成立，只剩改密/注销那族全会话吊销有效。第一版实现其实带了一条登出用例，但它手工 `AddCookie` 造出浏览器根本发不出的请求形状，于是 CI 全绿也测不到这件事（2026-09-29 双轴代码审查抓出）。现在由两条判据共同把守：Cookie 属性锁逐字断言 `Path=/api/auth`；登出用例**先**按浏览器的路径前缀规则证明「送得到」、**再**谈吊销（把 Path 收回单端点即刻判红）。CSRF 面不放宽：`SameSite=Lax` 下跨站 POST 不带 Cookie，两个消费点都只接受同站 POST。
- **一条已接受的破坏面（维护者 2026-09-29 裁定：不加迁移）**：部署时存量 Web 会话的 refresh 还躺在 localStorage，而新链路只认 Cookie ⇒ 这些会话会在下一次续期时失败、约 2h 内被登出。依据是「没有多少存量会话，踢出就行」。⇒ 这是**必须随发布说明公开**的降级，不能让它以「用户莫名掉线」的形式被发现。启动时清掉 legacy refresh（`frontend/src/stores/auth.ts`）是这条裁定的组成部分：不留第二条续期路径，否则「refresh 不进 JS 可达存储」只是纸面成立。

## 相关

- ADR-0016（被本 ADR 修订）；ADR-0066（XSS 的注入源层）
- `backend/internal/api/auth.go`（/refresh、/logout）、`frontend/src/api/client.ts`、`frontend/src/utils/storage.ts`
