# 01 — 后端：阅读量语义拆分

**What to build:** 后端把「阅读量」与「详情请求次数」分离——公开详情接口支持 `no_view=1` 跳过计数（供门户 SSR/爬虫路径使用），并新增客户端计数端点（真实浏览器 hydration 后调用）。既有不带参数的行为与管理端接口保持不变，兼容当前 Vue SPA。

**Blocked by:** None — can start immediately

**Status:** ready-for-agent

- [ ] `GET /api/featured-content/:id?no_view=1` 不改变该文章 `view_count`，其余响应与不带参数时完全一致
- [ ] `POST /api/featured-content/:id/view` 使该文章 `view_count + 1` 并返回最新值；对不存在或未发布的内容返回 404
- [ ] 不带 `no_view` 的公开详情 GET 保持既有计数行为（兼容现状）；管理端列表/详情/发布不受影响
- [ ] `API.md` 更新两个端点的说明（含「阅读量 vs 详情请求次数」语义）
- [ ] 既有 Go 测试与构建通过；新增端点有 handler 级测试
