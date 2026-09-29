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

- **两族 Cookie 并存时「族由 access 定」（2026-09-29，跨端对齐后补：#1376 评审 · 移动端 ADR-0030 ② 第 2 条与 ④ 第 1 项）**。决策 1 写下「优先读 Cookie」时默认了「同一浏览器同一时刻只有一族活跃会话」，而这个前提恰恰是 ADR-0022 已经否掉的东西——招聘者 access 收紧为 host-only 的原话理由就是「否则已登录学员的静默恢复路径会拿到 recruiter token、串角色」。两族 refresh cookie 在招聘者子域上**必然并存**（父域那枚学员的发得到），移动端两个角色打的又是同一个 `API_BASE_URL` ⇒ 按 Cookie 名序选族会让招聘者页的静默续期**轮换学员那一族**，客户端再把续出的令牌连同当前内存角色一起落盘 ⇒ 登录态被静默换成另一个身份而 UI 不动（新增的 7 条 Cookie 用例无一条覆盖并存）。
  落点：`Session.RefreshCookieForRequest` 只读**该族**那一枚 Cookie，族的判定与鉴权中间件同源——`ExtractToken` 的「Bearer 头优先，其次按 `CookieNames()` 顺序的 access cookie」里的 `role`（access cookie 的遍历实现上收为 `Session.AccessCookieValue`，中间件改调它，两处不许各有顺序）。两条线索都没有 ⇒ Cookie 通道视为**不可用**，而不是「任选一族」：端点据此回退请求体，两条都缺即 401。为此新增一个**只解 role、不做任何认证判定**的载荷解析（`accessRoleOf`：不验签、不看 `exp`、不看 `token_type`——必须如此，因为 `/api/auth/refresh` 恰恰只在 access 已死之后才被调用，任何「先验一遍再说」都会让浏览器侧永远续不了期），并由 AST 用例钉住它只被族判定这一条链调用。
  代价与边界（三条都要认清）：① Web 的刷新裸 client 从此必须发一条 `Authorization`（storage 里那支，**过期也发**）——它在这里不是凭证而是族声明；② `/logout` 与 `/refresh` 共用这把钥匙，所以**没有族线索的登出退化为「只清本地」**，不吊销任何一族（宁可如此，也不拿名序决定「谁的会话被终止」）；③ 通道归属由客户端容器行为决定、不由代码决定（DCloud `uni.request`：App/H5 自动带 cookie 且 H5 不可手动改，只有小程序不带），所以正确性由服务端这里的定族保证，**不靠**「请求体分支一定被走到」——本 ADR 之前那句「移动端继续用请求体通道」据此收窄。
  同步处置 **#1385 缺口 1**：登出吊销与 `Path` 最小暴露面互斥，本 ADR 选了前者（见上一条），`backend/internal/security/session.go` 的 `SignOut` 注释已按此改写；#1385 判据 1 的浏览器那一次由 `TestLogout_Cookie通道的refresh被吊销且响应清除Cookie`（先证投递可达、再证吊销）承担，移动端那一次属 #1386（本仓不改移动端源码）。

## 相关

- ADR-0016（被本 ADR 修订）；ADR-0066（XSS 的注入源层）；ADR-0022（招聘者 access 收紧 host-only —— 串角色缺陷的先例，本次族判定的理由与形状都取自它）
- 移动端 `docs/adr/0030-refresh令牌的通道归属与族判定口径`（本端不迁 Cookie 通道、④ 第 1 项要求服务端定族；两套 ADR 编号体系互不相关）
- `backend/internal/api/auth.go`（/refresh、/logout）、`backend/internal/security/session.go`（`RefreshCookieForRequest` / `accessRoleOf`）、`frontend/src/api/client.ts`、`frontend/src/utils/storage.ts`
- 票：#1363（实施）· #1376（跨端对齐与串族修复）· #1385（登出吊销与 `Path` 互斥）· #1386（移动端口径，另端）
