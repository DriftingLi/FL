// router/guard.ts：全局路由守卫的纯函数决策管线（#618）。
// 「这条路由属于哪个工作区」的领域事实单点在路由 meta.workspace 声明（与 role/roles 并列，
// 工作区语义而非子域名字面——IP 直连部署形态下子域概念不存在）；守卫主路径读声明，
// 子域前缀表（authRedirect.PATH_AUTH_ENTRIES）降级为未匹配路径（404）的兜底。
//
// 管线：(目标路由, 认证/环境状态) ⇒ 决策。五个决策步骤各管一件事，顺序即既有守卫的求值序：
//   1. subdomainBoundaryStep —— 子域边界（IP 直连旁路：无 DNS 子域名环境整段跳过）
//   2. authPageStep          —— 已登录访问认证页的回跳
//   3. authRequiredStep      —— 需登录而未登录（决策附带清登录态标记）
//   4. roleStep              —— 角色校验
//   5. credentialStep        —— 无证件 onboarding 预筛（ADR-0020；前置 3 已放行需登录路由）
// 纯函数约束：不触 window / store / router——环境事实（当前子域名、IP 直连、证件状态）由
// orchestrator（router/index.ts beforeEach）注入；跨子域名整页跳转 URL 与「当前子域名默认
// 工作区」由 orchestrator 构建/解析。证件数据未加载时经 load-credential 决策交还
// orchestrator 补一次数据后重跑管线（管线纯函数，重跑无副作用）。
import type { RouteLocationRaw } from 'vue-router'
import type { SubdomainType } from '@/utils/subdomain'
import { getTargetSubdomainForPath } from '@/utils/subdomain'
import { resolveWorkspaceForRole } from '@/utils/authRedirect'
import { routeNames } from '@/config/routeNames'

/**
 * 工作区：产品区语义（≠子域名）。
 * training 学员培训（含 AI 助手）；tutor 导师工作区（mentor. 子域名）；valuation 残值评估；
 * recruit 企业招聘；manage 管理后台（manage. 子域名，内部子域类型 admin）；
 * auth 主体系认证页（/login /register /forgot-password，无单一目标子域名）。
 */
export type Workspace = 'training' | 'tutor' | 'valuation' | 'recruit' | 'manage' | 'auth'

/** 工作区 → 目标子域名（auth 无单一目标子域名，由 subdomainBoundaryStep 特判） */
export const WORKSPACE_SUBDOMAIN: Record<Exclude<Workspace, 'auth'>, SubdomainType> = {
  training: 'training',
  tutor: 'tutor',
  valuation: 'valuation',
  recruit: 'recruit',
  manage: 'admin'
}

/** 守卫输入：目标路由的纯数据切片（orchestrator 从 to 投影；测试直接构造字面量） */
export interface GuardInput {
  path: string
  fullPath: string
  /** to.name（credentialStep 的 onboarding 判定用） */
  name: unknown
  /** to.meta（vue-router 已合并父链 meta，子记录覆盖父记录） */
  meta: Record<string, unknown>
  /** 逐条匹配记录的 meta（保持既有 some() 语义：requiresAuth/authPage 按逐条判断） */
  matched: Array<Record<string, unknown>>
}

/** 认证 + 环境状态（orchestrator 注入） */
export interface GuardState {
  isLoggedIn: boolean
  /** authStore.userInfo?.role ?? '' */
  role: string
  /** token && isLoggedIn && userInfo && userInfo.role（既有 hasValidToken 口径） */
  hasValidToken: boolean
  /** 当前子域名（getSubdomain()） */
  subdomain: SubdomainType
  /** IP 直连模式（isIpDirectMode()）：无 DNS 子域名环境，所有工作区经路径访问 */
  ipDirect: boolean
  /** 当前证件状态（credential store；'unloaded' = 未加载，'error' = 加载失败放行） */
  credential: 'unloaded' | 'none' | 'present' | 'error'
}

/** 守卫决策（orchestrator 执行） */
export type GuardDecision =
  /** 放行 */
  | { action: 'allow' }
  /** 路由内重定向；clearAuth = 先清登录态（token 失效口径） */
  | { action: 'redirect'; to: RouteLocationRaw; clearAuth?: boolean }
  /** 跨子域名整页跳转（orchestrator 经 buildCrossDomainAuthUrl 附带 auth_token 交接） */
  | { action: 'external'; target: SubdomainType; path: string }
  /** 留在当前子域名的默认工作区（orchestrator 经 getDefaultWorkspaceBySubdomain 解析） */
  | { action: 'workspace-home' }
  /** 证件预筛需要数据而未加载：orchestrator 拉取一次后重跑管线 */
  | { action: 'load-credential' }

type GuardStep = (input: GuardInput, state: GuardState) => GuardDecision | null

/** 目标路由声明的工作区（未声明 = 未匹配路径） */
export function workspaceOf(input: GuardInput): Workspace | undefined {
  return input.meta?.workspace as Workspace | undefined
}

/**
 * 目标子域名：主路径读 meta.workspace 声明；未匹配路径（404，无声明）走前缀表兜底
 * （getTargetSubdomainForPath 未命中归 main，与既有行为一致）。
 */
export function resolveTargetSubdomain(input: GuardInput): SubdomainType {
  const ws = workspaceOf(input)
  if (ws === undefined) return getTargetSubdomainForPath(input.path)
  if (ws === 'auth') return 'main'
  return WORKSPACE_SUBDOMAIN[ws]
}

/** 步骤 1：子域边界。IP 直连旁路（无 DNS 子域名环境，路径直接访问所有工作区）。 */
export const subdomainBoundaryStep: GuardStep = (input, state) => {
  if (state.ipDirect) return null
  const ws = workspaceOf(input)

  // 主体系认证页（/login /register /forgot-password）：无单一目标子域名——
  // main → 跳 training（主域名不再承载登录）；valuation → 内部重定向到估值认证页；其余子域名自留登录页
  if (ws === 'auth') {
    if (state.subdomain === 'main') {
      return { action: 'external', target: 'training', path: input.fullPath }
    }
    if (state.subdomain === 'valuation') {
      if (input.path === '/register') return { action: 'redirect', to: '/valuation/register' }
      if (input.path === '/forgot-password') return { action: 'redirect', to: '/valuation/forgot-password' }
      return { action: 'redirect', to: '/valuation/login' }
    }
    return null
  }

  const target = resolveTargetSubdomain(input)
  if (state.subdomain === target) return null
  if (target === 'main') {
    // 未匹配路径（404）兜底：留在当前子域名的默认工作区
    return { action: 'workspace-home' }
  }
  // 路径属于另一个功能子域名 → 跨子域名整页跳转（不同 origin，token 不共享）
  return { action: 'external', target, path: input.fullPath }
}

/** 步骤 2：已登录访问认证页的回跳。 */
export const authPageStep: GuardStep = (input, state) => {
  if (!state.isLoggedIn || !state.role) return null
  const ws = workspaceOf(input)
  // 主体系认证页 → 按当前子域名回对应工作区
  if (ws === 'auth') return { action: 'workspace-home' }
  // 估值认证页 → 已登录学员回评估历史
  const isAuthPage = input.matched.some(record => record.authPage === true)
  if (isAuthPage && ws === 'valuation' && state.role === 'hrwai_user') {
    return { action: 'redirect', to: '/valuation/history' }
  }
  return null
}

/** 步骤 3：需登录而未登录 → 清登录态并回认证页（估值工作区回估值登录页）。 */
export const authRequiredStep: GuardStep = (input, state) => {
  const requiresAuth = input.matched.some(record => record.requiresAuth === true)
  if (!requiresAuth) return { action: 'allow' }
  if (state.hasValidToken) return null
  const to = workspaceOf(input) === 'valuation'
    ? { path: '/valuation/login', query: { redirect: input.fullPath } }
    : { path: '/login', query: { redirect: input.fullPath } }
  return { action: 'redirect', to, clearAuth: true }
}

/** 步骤 4：角色校验（meta.role 单角色 / meta.roles 多角色，最终合并 meta 生效）。 */
export const roleStep: GuardStep = (input, state) => {
  const requiredRole = input.meta?.role as string | undefined
  const requiredRoles = input.meta?.roles as string[] | undefined
  const roleMatched = requiredRoles
    ? requiredRoles.includes(state.role)
    : (requiredRole ? requiredRole === state.role : true)
  if (roleMatched) return null
  // 管理员/导师回各自工作台（角色 → 默认工作区单点）
  if (state.role === 'admin' || state.role === 'tutor') {
    return { action: 'redirect', to: resolveWorkspaceForRole(state.role) }
  }
  // 学员/未知角色访问估值受限页 → 回估值首页（公开，无需登录）
  if (workspaceOf(input) === 'valuation') return { action: 'redirect', to: '/valuation' }
  // 其余 → 学员工作区
  return { action: 'redirect', to: '/training' }
}

/**
 * 步骤 5：无证件 onboarding 预筛（ADR-0020）——hrwai_user 在 training 工作区且未选证件时
 * 强制进 onboarding；已选证件访问 onboarding → 回 /training。IP 直连旁路。
 * 求值序前置：authRequiredStep 已对无需登录路由放行（预筛不适用于公开页）。
 * 证件未加载 → load-credential 决策交还 orchestrator；加载失败（'error'）放行，避免卡死登录。
 */
export const credentialStep: GuardStep = (input, state) => {
  if (state.role !== 'hrwai_user' || state.ipDirect) return null
  if (workspaceOf(input) !== 'training') return null
  if (state.credential === 'unloaded') return { action: 'load-credential' }
  const isOnboarding = input.name === routeNames.CredentialOnboarding
  if (state.credential === 'none' && !isOnboarding) {
    return { action: 'redirect', to: { name: routeNames.CredentialOnboarding } }
  }
  if (state.credential === 'present' && isOnboarding) {
    return { action: 'redirect', to: '/training' }
  }
  return null
}

/**
 * 决策管线：按既有守卫求值序执行各步骤，首个产出决策的步骤生效；全步骤放行 = allow。
 * 步骤顺序 = 行为不变承诺的一部分，不可调整。
 */
export function resolveGuardDecision(input: GuardInput, state: GuardState): GuardDecision {
  const steps: GuardStep[] = [subdomainBoundaryStep, authPageStep, authRequiredStep, roleStep, credentialStep]
  for (const step of steps) {
    const decision = step(input, state)
    if (decision) return decision
  }
  return { action: 'allow' }
}
