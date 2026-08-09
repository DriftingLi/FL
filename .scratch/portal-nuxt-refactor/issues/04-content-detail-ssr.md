# 04 — 内容详情页 SSR + 客户端计数 + 404

**What to build:** `/content/[id]` 内容详情页 SSR 渲染完整文章（Markdown 服务端渲染、上/下一篇、相关资讯、正文图片），SSR 请求不计阅读量（`no_view=1`），真实浏览器 hydration 后经数据访问层计数端点累加阅读量；不存在/未发布内容返回真实 404；文章级 SEO（标题/OG 封面/`Article` JSON-LD/canonical）。

**Blocked by:** 02 — 门户骨架：Nuxt 应用 + API 代理 + 数据访问层

**Status:** ready-for-agent

- [ ] 直接访问 `/content/[id]` 返回含文章完整正文 HTML（非 JS 渲染）：标题/分类/来源/发布日期/阅读量可见，正文 Markdown 渲染正确
- [ ] 详情 SSR 请求不改变后端 `view_count`；真实浏览器访问后 `view_count + 1`（hydration 后调用计数端点）
- [ ] 不存在或未发布文章返回 404 状态码与友好错误页（区别于现状的 200 + 空态）
- [ ] 上/下一篇与相关资讯链接可跳转；正文与封面图片（相对存储路径）正常显示
- [ ] 头部含文章级 title/description/OG（`og:image` 用封面或默认图、`article:published_time`）/`Article` JSON-LD/canonical
- [ ] 组件级测试（`@nuxt/test-utils` mock 数据访问层）断言 SSR 输出的 HTML 含标题、meta 与上/下一篇
