// 路径 ↔ 身份 / 子域名 知识单点。
// 「路径前缀 → 允许身份」是回跳白名单(isSafeRedirect)与子域名归属(getTargetSubdomainForPath)
// 共用的前缀表，避免两处分别硬编码道路径前缀造成漂移。
// 表按前缀长度降序排列：/training/tutor 必须先于 /training 匹配，否则会被吞掉。

import type { SubdomainType } from '@/utils/subdomain'

// 系统允许的角色（身份）
export type AllowedRole = 'admin' | 'tutor' | 'hrwai_user' | 'recruiter'

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
  { prefix: '/training', role: 'hrwai_user', subdomain: 'training' },
  { prefix: '/recruit', role: 'recruiter', subdomain: 'recruit' },
  { prefix: '/admin', role: 'admin', subdomain: 'admin' }
]

/** 返回目标路径命中的首条（最长）前缀条目；表已按前缀长度降序，首条匹配即最长前缀。 */
export function findPathEntry(path: string): PathAuthEntry | undefined {
  if (typeof path !== 'string') return undefined
  return PATH_AUTH_ENTRIES.find(entry => path.startsWith(entry.prefix))
}

// 同身份工作台内回跳白名单：仅当目标路径落在当前角色允许的前缀之内才放行，防止越权/钓鱼回跳。
// hrwai_user 允许 /training | /ai-assistant 前缀；admin 仅 /admin；tutor 仅 /training/tutor。
export function isSafeRedirect(role: string | undefined, target: string): boolean {
  if (!role || typeof target !== 'string') return false
  return PATH_AUTH_ENTRIES.some(entry => entry.role === role && target.startsWith(entry.prefix))
}

export function courseLearningReturn(target: unknown): string | undefined {
  if (typeof target !== 'string') return undefined
  const match = /^\/training\/course\/[1-9][0-9]*(?:\/chapter\/[1-9][0-9]*)?$/.exec(target)
  return match?.[0] === target ? target : undefined
}

/**
 * questionBankReturn 校验「登录后回学员题库并预选分类」的回跳目标（#2760 US-13）。
 *
 * 白名单是**形状**而不是前缀：只认 `/training/question-bank` 且查询串里只有
 * 一个合法的 `category_id`（正整数）。这样：
 *  - 少一档就不匹配（`/training/question-bank/../admin` 之类过不去）；
 *  - 夹带其它参数（`?category_id=1&next=https://evil`）一律整条拒，
 *    不做「留下合法的那几个」的裁剪——裁剪会让人以为参数生效了。
 *
 * 与 courseLearningReturn 并列而不是塞进 isSafeRedirect：后者是「同身份工作台内
 * 前缀白名单」，回答的是「这个人能不能去这个区域」；本函数回答的是
 * 「这条**带查询串**的地址是不是题库页的合法形态」——两个问题不同，判据也不该合并。
 */
export function questionBankReturn(target: unknown): string | undefined {
  if (typeof target !== 'string') return undefined
  const match = /^\/training\/question-bank(?:\?category_id=([1-9][0-9]*))?$/.exec(target)
  if (!match) return undefined
  return target
}

// 角色 → 默认工作区路径单点：admin→/admin/dashboard、tutor→/training/tutor、
// hrwai_user→/training；未知角色返回 '/'。
export function resolveWorkspaceForRole(role: string | undefined): string {
  if (role === 'admin') return '/admin/dashboard'
  if (role === 'tutor') return '/training/tutor'
  if (role === 'hrwai_user') return '/training'
  if (role === 'recruiter') return '/recruit'
  return '/'
}
