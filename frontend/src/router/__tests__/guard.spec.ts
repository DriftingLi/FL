// 全局守卫纯函数决策管线单测（#618 / spec #601）。
// 覆盖矩阵：工作区（training/tutor/valuation/recruit/manage/auth/未声明 404）× 角色
// （hrwai_user/tutor/admin/recruiter/未知）× 登录态（未登录/token 失效/已登录）×
// 无证件 onboarding 预筛（unloaded/none/present/error）× IP 直连旁路。
// prior art：authRedirect.spec.ts 的纯函数矩阵式用例。
import { describe, it, expect } from 'vitest'
import {
  resolveGuardDecision,
  resolveTargetSubdomain,
  workspaceOf,
  WORKSPACE_SUBDOMAIN,
  type GuardInput,
  type GuardState
} from '../guard'
import { routeNames } from '@/config/routeNames'

// ===== 构造器 =====

function input(overrides: Partial<GuardInput> = {}): GuardInput {
  return {
    path: '/training',
    fullPath: '/training',
    name: 'StudentDashboard',
    meta: { requiresAuth: true, role: 'hrwai_user', workspace: 'training' },
    matched: [{ requiresAuth: true, role: 'hrwai_user', workspace: 'training' }],
    ...overrides
  }
}

function state(overrides: Partial<GuardState> = {}): GuardState {
  return {
    isLoggedIn: true,
    role: 'hrwai_user',
    hasValidToken: true,
    subdomain: 'training',
    ipDirect: false,
    credential: 'present',
    ...overrides
  }
}

// ===== workspace 声明 → 子域名派生 =====

describe('workspaceOf / resolveTargetSubdomain（声明读法与派生）', () => {
  it('已声明路由读 meta.workspace', () => {
    expect(workspaceOf(input())).toBe('training')
    expect(workspaceOf(input({ meta: { workspace: 'recruit' } }))).toBe('recruit')
  })

  it('未声明路由（404）返回 undefined', () => {
    expect(workspaceOf(input({ meta: {}, matched: [] }))).toBeUndefined()
  })

  it.each([
    ['training', 'training'],
    ['tutor', 'tutor'],
    ['valuation', 'valuation'],
    ['recruit', 'recruit'],
    ['manage', 'admin'] // 管理后台子域名前缀 manage，内部类型 admin
  ] as const)('WORKSPACE_SUBDOMAIN[%s] === %s', (ws, sub) => {
    expect(WORKSPACE_SUBDOMAIN[ws]).toBe(sub)
  })

  it('auth 工作区无单一目标子域名 → main（由 subdomainBoundaryStep 特判）', () => {
    expect(resolveTargetSubdomain(input({ meta: { workspace: 'auth' } }))).toBe('main')
  })

  it('主路径读声明：/ai-assistant 声明 training，不再按前缀猜', () => {
    expect(
      resolveTargetSubdomain(input({ meta: { workspace: 'training' }, path: '/ai-assistant' }))
    ).toBe('training')
  })

  it('未声明路径（404）走前缀表兜底：/training/unknown → training；未知前缀 → main', () => {
    expect(
      resolveTargetSubdomain(input({ meta: {}, matched: [], path: '/training/unknown' }))
    ).toBe('training')
    expect(
      resolveTargetSubdomain(input({ meta: {}, matched: [], path: '/ai-assistant/x' }))
    ).toBe('training')
    expect(resolveTargetSubdomain(input({ meta: {}, matched: [], path: '/foo' }))).toBe('main')
  })
})

// ===== 步骤 1：子域边界（IP 直连旁路） =====

describe('subdomainBoundaryStep', () => {
  it('IP 直连旁路：整段跳过（跨工作区目标也不跳）', () => {
    expect(
      resolveGuardDecision(input({ meta: { workspace: 'valuation' }, path: '/valuation' }), state({ ipDirect: true, subdomain: 'training' }))
    ).toEqual({ action: 'allow' })
  })

  it('主体系认证页在 main 子域名 → 整页跳 training（主域名不再承载登录）', () => {
    const d = resolveGuardDecision(
      input({ meta: { workspace: 'auth' }, path: '/login', fullPath: '/login?x=1' }),
      state({ subdomain: 'main', isLoggedIn: false, hasValidToken: false, role: '' })
    )
    expect(d).toEqual({ action: 'external', target: 'training', path: '/login?x=1' })
  })

  it('主体系认证页在 valuation 子域名 → 内部重定向到估值认证页（三页各归各位）', () => {
    const st = state({ subdomain: 'valuation', isLoggedIn: false, hasValidToken: false, role: '' })
    expect(
      resolveGuardDecision(input({ meta: { workspace: 'auth' }, path: '/login' }), st)
    ).toEqual({ action: 'redirect', to: '/valuation/login' })
    expect(
      resolveGuardDecision(input({ meta: { workspace: 'auth' }, path: '/register' }), st)
    ).toEqual({ action: 'redirect', to: '/valuation/register' })
    expect(
      resolveGuardDecision(input({ meta: { workspace: 'auth' }, path: '/forgot-password' }), st)
    ).toEqual({ action: 'redirect', to: '/valuation/forgot-password' })
  })

  it('主体系认证页在功能子域名（training/tutor/admin/recruit）自留 → 放行', () => {
    for (const sub of ['training', 'tutor', 'admin', 'recruit'] as const) {
      expect(
        resolveGuardDecision(
          input({ meta: { workspace: 'auth' }, path: '/login', matched: [{ workspace: 'auth' }] }),
          state({ subdomain: sub, isLoggedIn: false, hasValidToken: false, role: '' })
        )
      ).toEqual({ action: 'allow' })
    }
  })

  it('已声明工作区与当前子域名一致 → 放行', () => {
    expect(resolveGuardDecision(input(), state({ subdomain: 'training' }))).toEqual({ action: 'allow' })
  })

  it('已声明工作区跨子域名 → 整页跳转（带 fullPath 交接）', () => {
    const d = resolveGuardDecision(
      input({ meta: { workspace: 'recruit' }, path: '/recruit', fullPath: '/recruit' }),
      state({ subdomain: 'training' })
    )
    expect(d).toEqual({ action: 'external', target: 'recruit', path: '/recruit' })
  })

  it('manage 工作区派生 admin 子域名：training 子域名访问 /admin → 跳 manage', () => {
    const d = resolveGuardDecision(
      input({ meta: { workspace: 'manage', role: 'admin' }, path: '/admin/dashboard', fullPath: '/admin/dashboard' }),
      state({ subdomain: 'training', role: 'admin' })
    )
    expect(d).toEqual({ action: 'external', target: 'admin', path: '/admin/dashboard' })
  })

  it('未声明路径（404）兜底：非 main 子域名访问未知前缀 → 留在当前子域名默认工作区', () => {
    const d = resolveGuardDecision(
      input({ meta: {}, matched: [], path: '/foo', name: undefined }),
      state({ subdomain: 'valuation' })
    )
    expect(d).toEqual({ action: 'workspace-home' })
  })

  it('未声明路径（404）兜底：main 子域名访问未知前缀 → 后续步骤放行', () => {
    expect(
      resolveGuardDecision(
        input({ meta: {}, matched: [], path: '/foo', name: undefined }),
        state({ subdomain: 'main' })
      )
    ).toEqual({ action: 'allow' })
  })

  it('未声明但前缀表命中（404 挂在工作区前缀下）→ 按前缀子域名整页跳转', () => {
    const d = resolveGuardDecision(
      input({ meta: {}, matched: [], path: '/training/unknown', fullPath: '/training/unknown', name: undefined }),
      state({ subdomain: 'valuation' })
    )
    expect(d).toEqual({ action: 'external', target: 'training', path: '/training/unknown' })
  })
})

// ===== 步骤 2：已登录访问认证页 =====

describe('authPageStep', () => {
  it('未登录 / 无角色 → 不干预（认证页公开，无 requiresAuth）', () => {
    expect(
      resolveGuardDecision(
        input({ meta: { workspace: 'auth' }, path: '/login', matched: [{ workspace: 'auth' }] }),
        state({ subdomain: 'training', isLoggedIn: false, hasValidToken: false, role: '' })
      )
    ).toEqual({ action: 'allow' })
  })

  it('已登录访问主体系认证页 → 回当前子域名默认工作区', () => {
    const d = resolveGuardDecision(
      input({ meta: { workspace: 'auth' }, path: '/login' }),
      state({ subdomain: 'training', role: 'admin' })
    )
    expect(d).toEqual({ action: 'workspace-home' })
  })

  it('已登录学员访问估值认证页 → 回评估历史', () => {
    const d = resolveGuardDecision(
      input({
        meta: { workspace: 'valuation', authPage: true },
        path: '/valuation/login',
        matched: [{ authPage: true, workspace: 'valuation' }]
      }),
      state({ subdomain: 'valuation', role: 'hrwai_user' })
    )
    expect(d).toEqual({ action: 'redirect', to: '/valuation/history' })
  })

  it('已登录非学员角色访问估值认证页 → 不按学员回跳', () => {
    expect(
      resolveGuardDecision(
        input({
          meta: { workspace: 'valuation', authPage: true },
          path: '/valuation/login',
          matched: [{ authPage: true, workspace: 'valuation' }]
        }),
        state({ subdomain: 'valuation', role: 'tutor' })
      )
    ).toEqual({ action: 'allow' })
  })

  it('已登录访问业务页（非认证页）→ 不干预', () => {
    expect(resolveGuardDecision(input(), state())).toEqual({ action: 'allow' })
  })
})

// ===== 步骤 3：需登录而未登录 =====

describe('authRequiredStep', () => {
  it('公开路由（无 requiresAuth）→ 决策 allow（短路后续角色/预筛步骤）', () => {
    expect(
      resolveGuardDecision(
        input({ meta: { requiresAuth: false, workspace: 'training' }, matched: [{ requiresAuth: false }] }),
        state({ isLoggedIn: false, hasValidToken: false, role: '', credential: 'none' })
      )
    ).toEqual({ action: 'allow' })
  })

  it('需登录且 token 有效 → 交给后续步骤', () => {
    expect(resolveGuardDecision(input(), state())).toEqual({ action: 'allow' })
  })

  it('需登录而 token 缺失 → 清登录态回主登录页（带 redirect 回跳）', () => {
    const d = resolveGuardDecision(
      input({ fullPath: '/training/courses?a=1' }),
      state({ isLoggedIn: false, hasValidToken: false, role: '' })
    )
    expect(d).toEqual({
      action: 'redirect',
      to: { path: '/login', query: { redirect: '/training/courses?a=1' } },
      clearAuth: true
    })
  })

  it('需登录而 token 缺失（估值工作区）→ 清登录态回估值登录页', () => {
    const d = resolveGuardDecision(
      input({ meta: { requiresAuth: true, roles: ['hrwai_user'], workspace: 'valuation' }, path: '/valuation/history', fullPath: '/valuation/history' }),
      state({ subdomain: 'valuation', isLoggedIn: false, hasValidToken: false, role: '' })
    )
    expect(d).toEqual({
      action: 'redirect',
      to: { path: '/valuation/login', query: { redirect: '/valuation/history' } },
      clearAuth: true
    })
  })
})

// ===== 步骤 4：角色校验（工作区 × 角色） =====

describe('roleStep', () => {
  it('meta.role 单角色匹配 / meta.roles 多角色匹配 / 无角色声明 → 放行', () => {
    expect(resolveGuardDecision(input(), state({ role: 'hrwai_user' }))).toEqual({ action: 'allow' })
    expect(
      resolveGuardDecision(
        input({ meta: { requiresAuth: true, roles: ['hrwai_user', 'tutor'], workspace: 'training' } }),
        state({ role: 'tutor' })
      )
    ).toEqual({ action: 'allow' })
    expect(
      resolveGuardDecision(
        input({ meta: { requiresAuth: true, workspace: 'training' } }),
        state({ role: 'recruiter' })
      )
    ).toEqual({ action: 'allow' })
  })

  it('admin 闯学员区 → 回管理员工作台', () => {
    expect(
      resolveGuardDecision(input(), state({ role: 'admin' }))
    ).toEqual({ action: 'redirect', to: '/admin/dashboard' })
  })

  it('tutor 闯学员区 → 回导师工作台', () => {
    expect(
      resolveGuardDecision(input(), state({ role: 'tutor' }))
    ).toEqual({ action: 'redirect', to: '/training/tutor' })
  })

  it('recruiter 闯估值受限页 → 回估值首页（公开）', () => {
    expect(
      resolveGuardDecision(
        input({ meta: { requiresAuth: true, roles: ['hrwai_user'], workspace: 'valuation' }, path: '/valuation/history' }),
        state({ subdomain: 'valuation', role: 'recruiter' })
      )
    ).toEqual({ action: 'redirect', to: '/valuation' })
  })

  it('学员闯招聘工作区 → 回学员工作区', () => {
    expect(
      resolveGuardDecision(
        input({ meta: { requiresAuth: true, role: 'recruiter', workspace: 'recruit' }, path: '/recruit' }),
        state({ subdomain: 'recruit', role: 'hrwai_user' })
      )
    ).toEqual({ action: 'redirect', to: '/training' })
  })

  it('未知角色闯受限页 → 回学员工作区', () => {
    expect(
      resolveGuardDecision(input(), state({ role: 'manager' }))
    ).toEqual({ action: 'redirect', to: '/training' })
  })
})

// ===== 步骤 5：无证件 onboarding 预筛 =====

describe('credentialStep', () => {
  it('非 hrwai_user / IP 直连 / 非 training 工作区 → 不预筛', () => {
    // 非学员角色（路由不声明角色时角色步放行，预筛也不介入）
    expect(
      resolveGuardDecision(
        input({ meta: { requiresAuth: true, workspace: 'training' } }),
        state({ role: 'tutor', credential: 'none' })
      )
    ).toEqual({ action: 'allow' })
    expect(resolveGuardDecision(input(), state({ ipDirect: true, credential: 'none' }))).toEqual({ action: 'allow' })
    expect(
      resolveGuardDecision(
        input({ meta: { requiresAuth: true, roles: ['hrwai_user'], workspace: 'valuation' }, path: '/valuation/history' }),
        state({ subdomain: 'valuation', credential: 'none' })
      )
    ).toEqual({ action: 'allow' })
  })

  it('证件未加载 → load-credential（orchestrator 补数据后重跑）', () => {
    expect(
      resolveGuardDecision(input(), state({ credential: 'unloaded' }))
    ).toEqual({ action: 'load-credential' })
  })

  it('无证件访问 training 业务页 → 强制进 onboarding', () => {
    expect(
      resolveGuardDecision(input(), state({ credential: 'none' }))
    ).toEqual({ action: 'redirect', to: { name: routeNames.CredentialOnboarding } })
  })

  it('无证件访问 onboarding 本身 → 放行', () => {
    expect(
      resolveGuardDecision(
        input({ name: routeNames.CredentialOnboarding, path: '/training/onboarding/credential' }),
        state({ credential: 'none' })
      )
    ).toEqual({ action: 'allow' })
  })

  it('已有证件访问 onboarding → 回 /training', () => {
    expect(
      resolveGuardDecision(
        input({ name: routeNames.CredentialOnboarding, path: '/training/onboarding/credential' }),
        state({ credential: 'present' })
      )
    ).toEqual({ action: 'redirect', to: '/training' })
  })

  it('证件加载失败 → orchestrator 映射为 none → 跳 onboarding（与既有行为一致）', () => {
    expect(resolveGuardDecision(input(), state({ credential: 'none' }))).toEqual({
      action: 'redirect',
      to: { name: routeNames.CredentialOnboarding }
    })
  })
})

// ===== 管线编排：步骤顺序即既有守卫求值序 =====

describe('resolveGuardDecision（顺序与优先级）', () => {
  it('子域边界先于登录态：main 子域名访问 /login（即使未登录）→ 先整页跳 training', () => {
    const d = resolveGuardDecision(
      input({ meta: { workspace: 'auth' }, path: '/login', fullPath: '/login', matched: [{ workspace: 'auth' }] }),
      state({ subdomain: 'main', isLoggedIn: false, hasValidToken: false, role: '' })
    )
    expect(d).toEqual({ action: 'external', target: 'training', path: '/login' })
  })

  it('未登录短路先于角色校验：token 缺失的 admin 访问学员区 → 回登录而非管理员工作台', () => {
    const d = resolveGuardDecision(
      input(),
      state({ role: 'admin', isLoggedIn: false, hasValidToken: false })
    )
    expect(d).toEqual({
      action: 'redirect',
      to: { path: '/login', query: { redirect: '/training' } },
      clearAuth: true
    })
  })

  it('角色校验先于预筛：无证件学员闯招聘区 → 先按角色回学员工作区', () => {
    const d = resolveGuardDecision(
      input({ meta: { requiresAuth: true, role: 'recruiter', workspace: 'recruit' }, path: '/recruit' }),
      state({ subdomain: 'recruit', credential: 'none' })
    )
    expect(d).toEqual({ action: 'redirect', to: '/training' })
  })

  it('全步骤放行 → allow', () => {
    expect(resolveGuardDecision(input(), state())).toEqual({ action: 'allow' })
  })
})
