// 子域名配置常量。
// 生产环境通过 Vite 环境变量注入（VITE_MAIN_DOMAIN / VITE_MENTOR_SUBDOMAIN / VITE_ADMIN_SUBDOMAIN），
// 开发环境默认使用 *.localhost 模式。
//
// 部署时需要在 Cloudflare Pages 或 DNS 中为这三个域名配置 CNAME 指向同一前端站点。
// 后端 CORS_ORIGINS 也必须同时包含这三个域名。

// 主域名（用于跨子域名跳转时的根域名推导参考）
export const MAIN_DOMAIN = import.meta.env.VITE_MAIN_DOMAIN || 'localhost'

// 完整的子域名 URL 示例（用于文档展示与跨子域名提示）
export const MENTOR_SUBDOMAIN = import.meta.env.VITE_MENTOR_SUBDOMAIN || 'mentor.localhost'
export const ADMIN_SUBDOMAIN = import.meta.env.VITE_ADMIN_SUBDOMAIN || 'admin.localhost'

// 各子域名对应的入口描述（用于登录页底部引导文案）
export const SUBDOMAIN_ENTRANCES = {
  student: {
    label: '学员入口',
    url: MAIN_DOMAIN,
    workspace: '/training'
  },
  tutor: {
    label: '导师入口',
    url: MENTOR_SUBDOMAIN,
    workspace: '/training/tutor'
  },
  admin: {
    label: '管理员入口',
    url: ADMIN_SUBDOMAIN,
    workspace: '/admin/dashboard'
  }
} as const
