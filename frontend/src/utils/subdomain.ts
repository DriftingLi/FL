// 子域名解析工具：通过 window.location.hostname 判断当前访问的子域名类型。
// 五类子域名约定：
//   - 主域名（www.example.com / example.com / localhost）：官网、派单等公共页面（无登录）
//   - training.主域名：学员培训 + AI 助手 + 学员登录
//   - valuation.主域名：残值评估（含历史）+ 学员登录（与 training 共享用户体系）
//   - mentor.主域名：导师登录与工作区
//   - admin.主域名：管理员登录与后台
//
// 跨子域名视为不同站点，localStorage 中存储的 token 不共享，
// 用户在不同子域名间切换时需要重新登录。

export type SubdomainType = 'main' | 'training' | 'valuation' | 'tutor' | 'admin'

// 子域名前缀到类型的映射（用于构建跨子域名 URL）
const SUBDOMAIN_PREFIX_MAP: Record<Exclude<SubdomainType, 'main'>, string> = {
  training: 'training',
  valuation: 'valuation',
  tutor: 'mentor', // 导师子域名前缀为 mentor
  admin: 'admin'
}

// 解析当前 hostname 对应的子域名类型
export function getSubdomain(): SubdomainType {
  if (typeof window === 'undefined') return 'main'
  const host = window.location.hostname.toLowerCase()
  if (host.startsWith('mentor.')) return 'tutor'
  if (host.startsWith('admin.')) return 'admin'
  if (host.startsWith('training.')) return 'training'
  if (host.startsWith('valuation.')) return 'valuation'
  return 'main'
}

export function isMainSubdomain(): boolean {
  return getSubdomain() === 'main'
}

// 推导当前 hostname 的根域名（用于跨子域名 URL 构建）。
// 例：mentor.example.com → example.com
//     mentor.example.top → example.top
//     example.top → example.top（2 段已是根域名，保持不变）
//     mentor.localhost → localhost（开发环境特殊处理）
//     localhost → localhost（无子域段可去，回退原值）
//     127.0.0.1 → 127.0.0.1
function getRootDomain(): string {
  const host = window.location.hostname.toLowerCase()
  // IPv4 或单段主机名（localhost）直接返回
  if (/^\d+\.\d+\.\d+\.\d+$/.test(host) || !host.includes('.')) {
    return host
  }
  const parts = host.split('.')
  // 开发环境 *.localhost：mentor.localhost → localhost
  if (parts.length === 2 && parts[1] === 'localhost') {
    return 'localhost'
  }
  // 2 段生产域名（如 example.top、example.com）已是根域名，保持不变
  if (parts.length <= 2) {
    return host
  }
  // 3 段以上：去掉第一个子域段
  // mentor.example.top → example.top
  // www.example.com → example.com
  const root = parts.slice(1).join('.')
  return root || host
}

// 根据路径推导应该所在的子域名类型。
// 用于路由守卫：当前子域名与目标子域名不一致时触发跨子域名跳转。
// 注意：/login 和 /register 不在此处理，由路由守卫特殊处理（每个子域名都可有自己的登录页）。
export function getTargetSubdomainForPath(path: string): SubdomainType {
  // 导师工作区（必须在 /training 之前匹配，否则会被 /training 吞掉）
  if (path.startsWith('/training/tutor')) return 'tutor'
  // 学员培训
  if (path.startsWith('/training')) return 'training'
  // AI 助手与培训共用 training 子域名
  if (path.startsWith('/ai-assistant')) return 'training'
  // 管理员后台
  if (path.startsWith('/admin')) return 'admin'
  // 残值评估所有界面（含历史、报告、电池评估）
  if (path.startsWith('/valuation')) return 'valuation'
  // 其它路径（/、/dispatch、/dashboard 兼容重定向等）在主域名
  return 'main'
}

// 构建跨子域名的绝对 URL。
// target 为 'main' 时构建主域名 URL（去掉子域名前缀），否则构建对应子域名 URL。
export function buildSubdomainUrl(target: SubdomainType, path: string): string {
  const protocol = window.location.protocol
  const port = window.location.port ? `:${window.location.port}` : ''
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  const rootDomain = getRootDomain()

  if (target === 'main') {
    return `${protocol}//${rootDomain}${port}${normalizedPath}`
  }

  const prefix = SUBDOMAIN_PREFIX_MAP[target]
  return `${protocol}//${prefix}.${rootDomain}${port}${normalizedPath}`
}

// 获取当前子域名对应的默认工作区路径（同子域名内的相对路径）
export function getDefaultWorkspaceBySubdomain(): string {
  const sub = getSubdomain()
  if (sub === 'tutor') return '/training/tutor'
  if (sub === 'admin') return '/admin/dashboard'
  if (sub === 'training') return '/training'
  if (sub === 'valuation') return '/valuation'
  return '/' // 主域名
}

// 获取当前子域名对应的登录角色
// training 和 valuation 子域名都是学员登录
export function getRoleForSubdomain(): 'student' | 'tutor' | 'admin' {
  const sub = getSubdomain()
  if (sub === 'tutor') return 'tutor'
  if (sub === 'admin') return 'admin'
  // training、valuation 都是学员登录
  return 'student'
}
