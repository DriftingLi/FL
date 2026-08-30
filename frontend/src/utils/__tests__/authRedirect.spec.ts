// authRedirect 单点表与回跳白名单单测
// TDD：先锁定「路径前缀 → 允许身份/子域名」与「角色 → 工作区」的既有语义，再收敛到单点函数。
import { describe, it, expect } from 'vitest'
import { isSafeRedirect, resolveWorkspaceForRole, findPathEntry, PATH_AUTH_ENTRIES } from '../authRedirect'
import { getTargetSubdomainForPath, type SubdomainType } from '../subdomain'

describe('PATH_AUTH_ENTRIES 单点表（路径前缀 → 允许身份）', () => {
  it('前缀按长度降序排列，保证最长前缀优先匹配', () => {
    const prefixes = PATH_AUTH_ENTRIES.map(e => e.prefix)
    const expectedDesc = [...prefixes].sort((a, b) => b.length - a.length)
    expect(prefixes).toEqual(expectedDesc)
  })

  it('内含全部六条路径前缀规约', () => {
    expect(PATH_AUTH_ENTRIES.map(e => e.prefix)).toEqual([
      '/training/tutor',
      '/ai-assistant',
      '/valuation',
      '/training',
      '/recruit',
      '/admin'
    ])
  })

  it('身份规约与票约定稿一致', () => {
    const byPrefix = Object.fromEntries(PATH_AUTH_ENTRIES.map(e => [e.prefix, e.role]))
    expect(byPrefix).toEqual({
      '/recruit': 'recruiter',
      '/training/tutor': 'tutor',
      '/admin': 'admin',
      '/training': 'hrwai_user',
      '/valuation': 'hrwai_user',
      '/ai-assistant': 'hrwai_user'
    })
  })
})

describe('findPathEntry（首个/最长前缀匹配）', () => {
  it('/training/tutor 先于 /training 命中，返回 tutor 条目', () => {
    expect(findPathEntry('/training/tutor/courses')?.role).toBe('tutor')
  })

  it('/training 命中培训条目', () => {
    expect(findPathEntry('/training/courses')?.subdomain).toBe('training')
  })

  it('未命中返回 undefined', () => {
    expect(findPathEntry('/')).toBeUndefined()
    expect(findPathEntry('/foo')).toBeUndefined()
  })
})

describe('isSafeRedirect（同身份工作台内回跳白名单）', () => {
  // 矩阵：各角色 × 各候选目标路径。期望值来自「路径前缀 → 允许身份」表规约，
  // 与 Login.vue 运行时回跳白名单既有语义保持一致（hrwai_user 允许 /training|/valuation|/ai-assistant）。
  const cases: Array<[string | undefined, string, boolean]> = [
    // ===== admin：仅 /admin 前缀 =====
    ['admin', '/admin', true],
    ['admin', '/admin/dashboard', true],
    ['admin', '/training', false],
    ['admin', '/training/tutor', false],
    ['admin', '/valuation', false],
    ['admin', '/ai-assistant', false],
    ['admin', '/foo', false],
    // ===== tutor：仅 /training/tutor 前缀 =====
    ['tutor', '/training/tutor', true],
    ['tutor', '/training/tutor/courses', true],
    ['tutor', '/training', false],
    ['tutor', '/admin/dashboard', false],
    ['tutor', '/valuation', false],
    ['tutor', '/ai-assistant', false],
    // ===== hrwai_user：允许 /training / valuation / ai-assistant 前缀 =====
    ['hrwai_user', '/training', true],
    ['hrwai_user', '/training/courses', true],
    ['hrwai_user', '/valuation', true],
    ['hrwai_user', '/valuation/history', true],
    ['hrwai_user', '/ai-assistant', true],
    ['hrwai_user', '/ai-assistant/chat', true],
    ['hrwai_user', '/admin/dashboard', false],
    ['hrwai_user', '/recruit', false],
    ['hrwai_user', '/foo', false],
    // ===== recruiter：仅 /recruit 前缀 =====
    ['recruiter', '/recruit', true],
    ['recruiter', '/recruit/resumes', true],
    ['recruiter', '/training', false],
    ['recruiter', '/admin', false],
    ['recruiter', '/valuation', false],
    // ===== 未知/空角色：一律不放行 =====
    ['manager', '/admin/dashboard', false],
    ['manager', '/training/courses', false],
    ['', '/training', false],
    [undefined, '/training', false]
  ]

  it.each(cases)('isSafeRedirect(%s, %s) === %s', (role, target, expected) => {
    expect(isSafeRedirect(role, target)).toBe(expected)
  })
})

describe('resolveWorkspaceForRole（角色 → 默认工作区单点）', () => {
  it('admin → /admin/dashboard', () => {
    expect(resolveWorkspaceForRole('admin')).toBe('/admin/dashboard')
  })

  it('tutor → /training/tutor', () => {
    expect(resolveWorkspaceForRole('tutor')).toBe('/training/tutor')
  })

  it('hrwai_user → /training', () => {
    expect(resolveWorkspaceForRole('hrwai_user')).toBe('/training')
  })

  it('recruiter → /recruit', () => {
    expect(resolveWorkspaceForRole('recruiter')).toBe('/recruit')
  })

  it('未知/空角色 → /', () => {
    expect(resolveWorkspaceForRole('manager')).toBe('/')
    expect(resolveWorkspaceForRole('')).toBe('/')
    expect(resolveWorkspaceForRole(undefined)).toBe('/')
  })
})

describe('getTargetSubdomainForPath（子域名归属，与旧行为一致）', () => {
  const cases: Array<[string, SubdomainType]> = [
    // 导师工作区：/training/tutor 必须在 /training 之前命中
    ['/training/tutor', 'tutor'],
    ['/training/tutor/courses', 'tutor'],
    // 学员培训
    ['/training', 'training'],
    ['/training/courses', 'training'],
    // 管理员后台
    ['/admin', 'admin'],
    ['/admin/dashboard', 'admin'],
    // 残值评估（含历史/报告/电池）
    ['/valuation', 'valuation'],
    ['/valuation/history', 'valuation'],
    // 企业招聘
    ['/recruit', 'recruit'],
    ['/recruit/resumes', 'recruit'],
    // AI 助手归 training 子域名
    ['/ai-assistant', 'training'],
    ['/ai-assistant/chat', 'training'],
    // 其余在主域名
    ['/', 'main'],
    ['/foo', 'main'],
    ['/login', 'main'],
    ['/dashboard', 'main']
  ]

  it.each(cases)('getTargetSubdomainForPath(%s) === %s', (path, expected) => {
    expect(getTargetSubdomainForPath(path)).toBe(expected)
  })
})
