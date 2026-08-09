# 05 — 精选内容归档页（/news + /news/[category]）

**What to build:** 新增精选内容归档页体系：`/news`（全部）+ `/news/[category]`（公司动态/行业新闻/产品资讯/资讯）分页列表页，卡片链接到详情；页头 `BreadcrumbList` JSON-LD 与归档页 SEO 元数据；SWR 60s 定时重新验证（后台发布后自动可见）；导航「内容精选」指向 `/news`。

**Blocked by:** 02 — 门户骨架：Nuxt 应用 + API 代理 + 数据访问层

**Status:** ready-for-agent

- [ ] `/news` 分页列出全部已发布内容；`/news/[category]` 按四类过滤，URL 使用分类 key（company/industry/product/news）
- [ ] 卡片含标题/摘要/分类标签/发布日期/封面，链接到 `/content/[id]`
- [ ] 无效分类返回 404；无数据时展示合理空态；分页边界（首页/末页）正确
- [ ] SWR 60s：后台发布新内容后 1 分钟内归档页可见，无需重新构建
- [ ] 页头含 `BreadcrumbList` JSON-LD 与归档页 title/description/canonical
- [ ] 门户导航「内容精选」指向 `/news`（与 03 首页导航联动验证）
