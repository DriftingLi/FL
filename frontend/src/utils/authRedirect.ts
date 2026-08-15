// 路径 ↔ 身份 / 子域名 知识单点。
// 「路径前缀 → 允许身份」是回跳白名单(isSafeRedirect)与子域名归属(getTargetSubdomainForPath)
// 共用的前缀表，避免两处分别硬编码道路径前缀造成漂移。
// 表按前缀长度降序排列：/training/tutor 必须先于 /training 匹配，否则会被吞掉。

import type { SubdomainType } from '@/utils/subdomain'

// 系统允许的角色（身份）
export type AllowedRole = 'admin' | 'tutor' | 'hrwai_user'

export interface PathAuthEntry {
  /** 路径前缀（表按前缀长度降序，长前缀优先匹配） */
  prefix: string
  /** 允许访问该前缀的角色 */
  role: AllowedRole
  /** 该前缀归属的子域名（getTargetSubdomainForPath 派生，/ai-assistant 归 training） */
  subdomain: SubdomainType
}

// 「路径前缀 → 允许身份」单点表：getTargetSubdomainForPath 从 subdomain 派生，
// isSafeRedirect 从 role 派生。两函数必须共享本表以保持前缀数据单一来源。
export const PATH_AUTH_ENTRIES: PathAuthEntry[] = [
  { prefix: '/training/tutor', role: 'tutor', subdomain: 'tutor' },
  { prefix: '/ai-assistant', role: 'hrwai_user', subdomain: 'training' },
  { prefix: '/valuation', role: 'hrwai_user', subdomain: 'valuation' },
  { prefix: '/training', role: 'hrwai_user', subdomain: 'training' },
  { prefix: '/admin', role: 'admin', subdomain: 'admin' }
]

/** 返回目标路径命中的首条（最长）前缀条目；表已按前缀长度降序，首条匹配即最长前缀。 */
export function findPathEntry(path: string): PathAuthEntry | undefined {
  if (typeof path !== 'string') return undefined
  return PATH_AUTH_ENTRIES.find(entry => path.startsWith(entry.prefix))
}

// 同身份工作台内回跳白名单：仅当目标路径落在当前角色允许的前缀之内才放行，防止越权/钓鱼回跳。
// hrwai_user 允许 /training | /valuation | /ai-assistant 前缀；admin 仅 /admin；tutor 仅 /training/tutor。
export function isSafeRedirect(role: string | undefined, target: string): boolean {
  if (!role || typeof target !== 'string') return false
  return PATH_AUTH_ENTRIES.some(entry => entry.role === role && target.startsWith(entry.prefix))
}

// 角色 → 默认工作区路径单点：admin→/admin/dashboard、tutor→/training/tutor、
// hrwai_user→/training；未知角色返回 '/'。
export function resolveWorkspaceForRole(role: string | undefined): string {
  if (role === 'admin') return '/admin/dashboard'
  if (role === 'tutor') return '/training/tutor'
  if (role === 'hrwai_user') return '/training'
  return '/'
}
