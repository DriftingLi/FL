# ADR-0003: 生产 HTTP 环境下登录态跨子域不共享（约束记录）

- 状态：已接受（2026-08-05）——约束记录，非缺陷待办
- 领域：账号与认证 —— 会话（session）/ 登录态 Cookie

## 背景

三个模块共用 hrwai 登录体系：培训学习（`training.gccsmile.com`）、残值评估（`valuation.gccsmile.com`）、AI 助手与官网（`www.gccsmile.com`）。设计上通过父域 httpOnly Cookie（`hrwai_token`，`Domain=gccsmile.com`，ADR-0002 的会话模块实现）实现跨子域登录态共享，前端各子域启动时以 `/auth/me`（withCredentials）走 cookie 静默恢复登录（`stores/auth.ts validateToken`）。

## 问题

生产服务器公网 **443 端口不可用**，生产环境为 HTTP。`AUTH_COOKIE_SECURE` 生产强制 `true`（`config.go`），Secure cookie 在 HTTP 连接下**被浏览器拒绝接收与发送**，Cookie 通道完全失效，三个子域的登录态互不共享。

现状缓解（均已存在，非新增）：

- 前端主通道 `localStorage` token（跨子域不共享）；
- 跨子域跳转时 `auth_token` URL 一次性参数交接（`buildCrossDomainAuthUrl`），仅对"从已登录页面跳转"的导航生效；直接输入 URL / 新开标签无交接。

## 决策

- **不将 `AUTH_COOKIE_SECURE` 改为 `false` 迁就 HTTP**：登录 token 明文走 HTTP 会被中间人窃取，httpOnly 保护失效；宁可保持登录态不共享，也不开放此漏洞。
- 保持现状（localStorage + auth_token 交接）直至 443 可用。

## 后果与未来路径

- 三个模块间切换登录态需要重新登录（或经由跳转链路交接）。
- 443 恢复后启用 HTTPS（nginx 443 块 + TLS 证书，证书副本见 `docs/2026-08-04-清理报告.md`）即自动恢复 cookie 跨子域共享，**前端与后端代码无需改动**（`validateToken` 的 cookie 恢复逻辑已就绪，见 ADR-0002）。

## 相关

- `frontend/src/stores/auth.ts`（validateToken cookie 恢复路径）
- `frontend/src/utils/subdomain.ts`（auth_token 交接）
- `backend/internal/config/config.go`（AUTH_COOKIE_SECURE 生产强制）
- ADR-0002「会话（session）单一接口」
