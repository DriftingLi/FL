# 03 — 官网首页预渲染 + /dispatch 占位页

**What to build:** 官网首页（`/`）全部区块（Hero/公司介绍/创始人/核心服务/合作模式/服务保障/内容精选轮播/CTA）与现网视觉交互一致，构建时预渲染为静态 HTML；导航/页脚/滚动动画从 Vue SPA 移植；核心服务卡片跨模块跳转正确；`/dispatch`「即将上线」占位页；首页 SEO（title/description/OG/Organization JSON-LD/canonical）。

**Blocked by:** 02 — 门户骨架：Nuxt 应用 + API 代理 + 数据访问层

**Status:** ready-for-agent

- [ ] 构建产物中 `/` 为静态 HTML，含全部区块文本与精选内容（公开列表前 6 条标题/摘要/分类），爬虫无需 JS 即可读
- [ ] 首页视觉与交互与现网一致：服务保障轮播、内容精选轮播（自动播放/悬停暂停/无缝循环）、滚动动画、锚点导航
- [ ] 核心服务卡片跳转正确：叉车维修培训 → training. 子域名、残值评估 → valuation.、二手叉车交易 → 外链（mall.gccsmile.com）、AI 叉车助手 → training. 子域名 AI 助手
- [ ] 导航项与现网一致，且「内容精选」指向 `/news`（页面由 05 实现，先允许 404）
- [ ] 页脚完整（联系方式/二维码/备案信息），公众号/小程序图片正常显示
- [ ] `/` 头部含 title/description/OG/Twitter Card/`Organization` JSON-LD/canonical（www 固定版）
- [ ] `/dispatch` 为非 404 的「即将上线」占位页（预渲染）
