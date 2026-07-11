// 子域名配置常量。
// 生产环境通过 Vite 环境变量注入（VITE_MAIN_DOMAIN / VITE_*_SUBDOMAIN），
// 开发环境默认使用 *.localhost 模式。
//
// 部署时需要在 Cloudflare Pages 或 DNS 中为这五个域名配置 CNAME 指向同一前端站点。
// 后端 CORS_ORIGINS 也必须同时包含这五个域名。

// 主域名（用于跨子域名跳转时的根域名推导参考）
export const MAIN_DOMAIN = import.meta.env.VITE_MAIN_DOMAIN || 'localhost'

// 完整的子域名 URL 示例（用于文档展示与跨子域名提示）
export const TRAINING_SUBDOMAIN = import.meta.env.VITE_TRAINING_SUBDOMAIN || 'training.localhost'
export const VALUATION_SUBDOMAIN = import.meta.env.VITE_VALUATION_SUBDOMAIN || 'valuation.localhost'
export const MENTOR_SUBDOMAIN = import.meta.env.VITE_MENTOR_SUBDOMAIN || 'mentor.localhost'
export const ADMIN_SUBDOMAIN = import.meta.env.VITE_ADMIN_SUBDOMAIN || 'admin.localhost'
