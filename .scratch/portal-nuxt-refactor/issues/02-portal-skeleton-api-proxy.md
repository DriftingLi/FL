# 02 — 门户骨架：Nuxt 应用 + API 代理 + 数据访问层

**What to build:** 本地 `portal/` 创建独立 Nuxt 4 应用（加入 monorepo `.gitignore`），打通与后端的第一条通路：`/api` 与 `/static` 由 Nitro 代理到后端内网地址（SSR 直连 / 浏览器同源共用），并实现数据访问层（公开列表、公开详情含 `no_view=1`、客户端计数、存储相对路径解析、分类标签），带单元测试。这是后续所有页面取数的唯一 seam。

**Blocked by:** 01 — 后端：阅读量语义拆分

**Status:** ready-for-agent

- [ ] `portal/` 为 Nuxt 4 + TypeScript 项目，已加入根 `.gitignore`，不进入 monorepo 版本控制
- [ ] `runtimeConfig` 预留 `apiInternalBase`（SSR 后端内网地址）与站点公共配置；本地一条命令启动
- [ ] 开发/生产下 `/api/**` 与 `/static/**` 请求正确转发到后端（curl 验证真实数据可达）
- [ ] 数据访问层单元测试（mock `$fetch`）覆盖：列表参数（分页/分类）、详情带 `no_view=1`、客户端计数端点、相对存储路径解析、分类中文标签
- [ ] 数据访问层类型定义与后端公开接口响应结构一致（`content_id`/`published_at` 等字段口径）
