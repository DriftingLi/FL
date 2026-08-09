# 06 — 全站 SEO 收口：sitemap / robots / 百度验证 / canonical

**What to build:** 构建时生成 sitemap.xml（首页/归档页/全部已发布文章，与公开列表接口口径一致）、robots.txt 引用 sitemap；百度站长验证 meta 经环境变量注入（未配置则不出）；全站 canonical 统一为 `https://www.<domain>/...` 固定版，消除重复内容风险。

**Blocked by:** 03 — 官网首页预渲染 + /dispatch 占位页；04 — 内容详情页 SSR；05 — 精选内容归档页

**Status:** ready-for-agent

- [ ] 构建产物中 sitemap.xml 含首页、`/news`、四类归档页、全部已发布文章 URL，数量与公开列表接口一致
- [ ] robots.txt 允许抓取并引用 sitemap.xml
- [ ] `NUXT_PUBLIC_BAIDU_VERIFICATION` 设置后首页出现百度验证 meta；未设置时不输出
- [ ] 全站页面（首页/详情/归档/占位页）canonical 均为 `https://www.<domain>/` 对应路径
- [ ] 构建期 sitemap 生成在无后端可达环境下不阻塞构建（明确报错或降级，行为有文档说明）
