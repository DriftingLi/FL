// 子域名解析工具：通过 window.location.hostname 判断当前访问的子域名类型。
// 三类子域名约定：
//   - 主域名（www.example.com / example.com / localhost）：学员入口
//   - mentor.主域名：导师登录与工作区
//   - admin.主域名：管理员登录与后台
//
// 跨子域名视为不同站点，localStorage 中存储的 token 不共享，
// 用户在不同子域名间切换时需要重新登录。

export type SubdomainType = 'student' | 'tutor' | 'admin'

// 从根域名构建子域名 URL 时使用的子域名前缀
export type SubdomainPrefix = 'mentor' | 'admin'

// 解析当前 hostname 对应的子域名类型
export function getSubdomain(): SubdomainType {
  if (typeof window === 'undefined') return 'student'
  const host = window.location.hostname.toLowerCase()
  if (host.startsWith('mentor.')) return 'tutor'
  if (host.startsWith('admin.')) return 'admin'
  return 'student'
}

export function isMentorSubdomain(): boolean {
  return getSubdomain() === 'tutor'
}

export function isAdminSubdomain(): boolean {
  return getSubdomain() === 'admin'
}

export function isMainSubdomain(): boolean {
  return getSubdomain() === 'student'
}

// 推导当前 hostname 的根域名（去掉第一个子域段）。
// 例：mentor.example.com → example.com
//     admin.www.example.com → www.example.com
//     localhost → localhost（无子域段可去，回退原值）
//     127.0.0.1 → 127.0.0.1
function getRootDomain(): string {
  const host = window.location.hostname.toLowerCase()
  // IPv4 或单段主机名（localhost）直接返回
  if (/^\d+\.\d+\.\d+\.\d+$/.test(host) || !host.includes('.')) {
    return host
  }
  const parts = host.split('.')
  const root = parts.slice(1).join('.')
  return root || host
}

// 构建跨子域名的绝对 URL。
// 例：在 mentor.example.com 上调用 buildSubdomainUrl('admin', '/admin/dashboard')
//     → https://admin.example.com/admin/dashboard
export function buildSubdomainUrl(prefix: SubdomainPrefix, path: string): string {
  const protocol = window.location.protocol
  const rootDomain = getRootDomain()
  const port = window.location.port ? `:${window.location.port}` : ''
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  return `${protocol}//${prefix}.${rootDomain}${port}${normalizedPath}`
}

// 获取当前角色对应的默认工作区路径（同子域名内的相对路径）
export function getDefaultWorkspaceBySubdomain(): string {
  const sub = getSubdomain()
  if (sub === 'tutor') return '/training/tutor'
  if (sub === 'admin') return '/admin/dashboard'
  return '/training'
}

// 获取角色对应的登录端点路径（与后端路由一致）
export function getLoginApiPathBySubdomain(): string {
  const sub = getSubdomain()
  if (sub === 'tutor') return '/api/auth/tutor-login'
  if (sub === 'admin') return '/api/auth/admin-login'
  return '/api/auth/login'
}
