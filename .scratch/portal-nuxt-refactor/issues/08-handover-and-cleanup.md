# 08 — 上线移交：www 切流 + monorepo 清理 + AI 助手迁移

**What to build:** 门户仓库发布为独立 GitHub 仓库（用户执行），www 域名切流到门户部署；验证通过后回 monorepo 清理 Vue 官网代码（PortalHome/ContentDetail/Portal 布局/PortalNavbar/PortalFooter/`/portal` 与 `/dispatch` 路由/`portalNav`/`featured.ts` 公开接口部分），并把 AI 助手从 www 迁移到 training. 子域名（旧 `/ai-assistant` 301、子域名守卫与导航链接更新）。本 ticket 由用户主导执行，agent 可辅助清理改动。

**Blocked by:** 07 — 部署配套与文档（Dockerfile / nginx / CI / README）

**Status:** ready-for-agent

- [ ] 门户已推送独立 GitHub 仓库，www 由门户接管（用户执行）
- [ ] 线上验证六类行为：首页/详情/归档/404/微信分享卡片/百度抓取（返回 200 + 全量 HTML）
- [ ] monorepo 删除官网代码后，training./valuation./mentor./manage. 子域名功能不受影响（回归验证）
- [ ] AI 助手可在 training. 子域名访问；旧 www `/ai-assistant` 地址跳转或 301
- [ ] monorepo `CONTEXT.md`「门户与内容」小节与 AI 助手条目与实际状态一致
