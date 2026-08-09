# 07 — 部署配套与文档（Dockerfile / nginx / CI / README）

**What to build:** portal 仓库自持部署能力：多阶段 Dockerfile（Nitro standalone + node 运行时）、可选 nginx 反代模板（envsubst 域名/后端地址，沿用现有 `${DOMAIN}` 模式）、CI workflow（构建 + 推送镜像，部署步骤留占位）、README（环境变量清单/开发/构建/部署说明）。

**Blocked by:** 06 — 全站 SEO 收口：sitemap / robots / 百度验证 / canonical

**Status:** ready-for-agent

- [ ] `docker build` 成功；容器内首页/详情/归档可访问，`/api` 与 `/static` 代理可用（指向可配置后端地址）
- [ ] nginx 模板（如启用）支持 `${DOMAIN}` 与后端地址 envsubst，与现有部署模式一致
- [ ] CI workflow 能在干净环境完成安装 + 类型检查 + 测试 + 构建；推送/部署步骤为占位并注释说明需填写的密钥
- [ ] README 覆盖：环境变量清单（`apiInternalBase`/`NUXT_PUBLIC_SITE_URL`/`NUXT_PUBLIC_BAIDU_VERIFICATION`/`DOMAIN`）、本地开发、构建、部署步骤
- [ ] 构建期对后端的依赖（预渲染、sitemap）在 README/CI 中注明需注入 `apiInternalBase`
