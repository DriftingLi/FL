// router/guard.ts：全局路由守卫的纯函数决策管线（#618）。
// 「这条路由属于哪个工作区」的领域事实单点在路由 meta.workspace 声明（与 role/roles 并列，
// 工作区语义而非子域名字面——IP 直连部署形态下子域概念不存在）；守卫主路径读声明，
// 子域前缀表（authRedirect.PATH_AUTH_ENTRIES）降级为未匹配路径（404）的兜底。
//
// 管线：(目标路由, 认证/环境状态) ⇒ 决策。各决策步骤各管一件事，顺序即既有守卫的求值序：
//   1. subdomainBoundaryStep —— 子域边界（IP 直连旁路：无 DNS 子域名环境整段跳过）
//   2. authPageStep          —— 已登录访问认证页的回跳
//   3. authRequiredStep      —— 需登录而未登录（决策附带清登录态标记）
//   4. subdomainRoleStep     —— 专属子域身份匹配（#1614）：已登录但角色与该子域缺席角色
//                               不符 → 清登录态回本子域登录页。父域 Cookie 使学员登录态在
//                               mentor./manage. 生效，须先判「不是本子域的人」，否则角色
//                               回落会被子域边界整页弹去 training
//                               （#1651 补全：共号子域上的同一不变式落在回落出口，见 ADR-0075）
//   5. roleStep              —— 角色校验
//   6. credentialStep        —— 无证件 onboarding 预筛（ADR-0020；前置 3 已放行需登录路由）
// 另有一条**状态就绪**步骤：4b capabilityLoadStep —— 动态角色（管理端）的能力集尚未装载时，
// 先交还 orchestrator 补拉一次再重跑（#1645）。它不是授权判定，而是「判定所需的事实还没到」，
// 故排在 roleStep 之前：roleStep 的回落与 capabilityStep 都要读能力集。
// 纯函数约束：不触 window / store / router——环境事实（当前子域名、IP 直连、证件状态）由
// orchestrator（router/index.ts beforeEach）注入；跨子域名整页跳转 URL 与「当前子域名默认
// 工作区」由 orchestrator 构建/解析。证件数据未加载时经 load-credential 决策交还
// orchestrator 补一次数据后重跑管线；动态角色的能力集未落定时同理走 load-capabilities
// （管线纯函数，重跑无副作用）。
import type { RouteLocationRaw } from 'vue-router'
import type { SubdomainType } from '@/utils/subdomain'
import { getTargetSubdomainForPath } from '@/utils/subdomain'
import { resolveWorkspaceForRole } from '@/utils/authRedirect'
import { adminLandingPage } from '@/config/navigation'
import type { AuthzCapability } from '@/config/authz'
import { holdsCapability, isDynamicRole } from '@/utils/authzRuntime'
import type { RouteName } from '@/config/pages'

/** onboarding 预筛的落点：由页面描述符表派生的 RouteName 收窄（名单已无第二处手工表，写错即编译报错）。 */
const CREDENTIAL_ONBOARDING: RouteName = 'CredentialOnboarding'

/**
 * 工作区：产品区语义（≠子域名）。
 * training 学员培训（含 AI 助手）；tutor 导师工作区（mentor. 子域名）；
 * recruit 企业招聘；manage 管理后台（manage. 子域名，内部子域类型 admin）；
 * auth 主体系认证页（/login /register /forgot-password，无单一目标子域名）。
 *
 * 残值评估（valuation）本来也是一个工作区，2026-10 起公开界面迁到门户仓
 * （valuation.hrwai.cn/valuation），本仓不再有该工作区的任何路由。
 */
export type Workspace = 'training' | 'tutor' | 'recruit' | 'manage' | 'auth'

/** 工作区 → 目标子域名（auth 无单一目标子域名，由 subdomainBoundaryStep 特判） */
export const WORKSPACE_SUBDOMAIN: Record<Exclude<Workspace, 'auth'>, SubdomainType> = {
  training: 'training',
  tutor: 'tutor',
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
  /** 当前证件状态（credential store；'unloaded' = 未加载） */
  credential: 'unloaded' | 'none' | 'present'
  /**
   * 运行时能力集（#1618 段1）：动态角色（管理端）经 GET /admin/me/capabilities 下发，
   * 静态角色恒为空数组（它们的可达面由生成的能力表回答）。判定见 utils/authzRuntime。
   */
  capabilities: readonly string[]
  /**
   * 能力集是否**已落定**（ready 或 failed，见 stores/auth 的 CapabilityLoadState，#1645）。
   *
   * 未落定时 `capabilities` 是空数组，但它表达的是「还不知道」而不是「没有」——
   * 可见性判定不得据此下结论，否则全权限账号在能力集晚到一拍时也会被判无权限。
   */
  capabilitiesSettled: boolean
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
  /** 动态角色的能力集尚未落定：orchestrator 补拉一次后重跑管线（装载必然落定，故只触发一次） */
  | { action: 'load-capabilities' }

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
  // main → 跳 training（主域名不再承载登录）；其余子域名自留登录页
  if (ws === 'auth') {
    if (state.subdomain === 'main') {
      return { action: 'external', target: 'training', path: input.fullPath }
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
  // 主体系认证页 → 按当前子域名回对应工作区（到不了的角色由下一步的子域身份匹配拦回登录页）
  if (ws === 'auth') return { action: 'workspace-home' }
  return null
}

/** 步骤 3：需登录而未登录 → 清登录态并回认证页。 */
export const authRequiredStep: GuardStep = (input, state) => {
  const requiresAuth = input.matched.some(record => record.requiresAuth === true)
  if (!requiresAuth) return { action: 'allow' }
  if (state.hasValidToken) return null
  return { action: 'redirect', to: { path: '/login', query: { redirect: input.fullPath } }, clearAuth: true }
}

/**
 * 步骤 4：专属子域身份匹配（#1614 / ADR-0071）——父域 Cookie（hrwai_token / refresh 同 Domain）
 * 使登录态在所有子域生效，「已登录」不再等价「是本子域的人」。tutor/admin/recruit 三个专属
 * 子域上，登录角色与缺席角色（getRoleForSubdomain 口径：tutor/admin/recruiter）不符时，
 * 清登录态回**本子域**登录页并携带回跳。放在角色校验之前：否则学员会被 workspaceFallback
 * 弹向 /training，再被子域边界整页跳去 training 子域（原缺陷路径），永远到不了本子域登录页。
 * clearAuth 先清再重定向：管线重跑时未登录 → authRequiredStep 常规引导登录；登录页属 auth
 * 工作区不再触发本步骤，无循环。IP 直连（无子域概念）与共号子域（training/main）
 * 旁路，身份语义交给既有角色/能力校验。
 */
export const subdomainRoleStep: GuardStep = (input, state) => {
  if (state.ipDirect) return null
  const sub = state.subdomain
  if (sub !== 'tutor' && sub !== 'admin' && sub !== 'recruit') return null
  if (!state.isLoggedIn || !state.role) return null
  const expectedRole = sub === 'tutor' ? 'tutor' : sub === 'admin' ? 'admin' : 'recruiter'
  if (state.role === expectedRole) return null
  return { action: 'redirect', to: { path: '/login', query: { redirect: input.fullPath } }, clearAuth: true }
}

/**
 * 步骤 4b：动态角色的能力集装载（#1645）。
 *
 * 为什么必须等：管理端的能力集是**登录之后**才拉的（GET /admin/me/capabilities），而登录页
 * 紧接着就 push 到 /admin/dashboard —— 守卫若在这一拍就用能力集判可见性，拿到的是**尚未装载**
 * 的空集，「装载中」被当成「没有能力」，于是全权限的账号也被判无权限（#1638 的落点判定据此
 * 把超管送到「无管理权限」页：侧栏 20 项、固定签「仪表盘」，内容区却写着"你没有权限"）。
 *
 * 本步只报事实（这个角色的能力集还没落定），补拉与重跑交给 orchestrator —— 与 credentialStep
 * 的 `load-credential` 同形：管线保持纯函数，重跑无副作用（`loadCapabilities` 承诺必然落定）。
 *
 * 触发面与 capabilityStep **同界**：只有「需要登录」的页面才判能力（公开页对任意角色开放，
 * 不因能力位被锁死），因此也只有那些页面值得等能力集 —— 管理员打开公开的 /ai-assistant 时
 * 不必为一次装载把导航拖住。静态角色完全不适用：它们的可达面由生成的能力表回答。
 */
export const capabilityLoadStep: GuardStep = (input, state) => {
  if (!isDynamicRole(state.role) || state.capabilitiesSettled) return null
  const requiresAuth = input.matched.some(record => record.requiresAuth === true)
  if (!requiresAuth) return null
  return { action: 'load-capabilities' }
}

/** 步骤 5：角色校验（meta.role 单角色 / meta.roles 多角色，最终合并 meta 生效）。 */
export const roleStep: GuardStep = (input, state) => {
  const requiredRole = input.meta?.role as string | undefined
  const requiredRoles = input.meta?.roles as string[] | undefined
  const roleMatched = requiredRoles
    ? requiredRoles.includes(state.role)
    : (requiredRole ? requiredRole === state.role : true)
  if (roleMatched) return null
  return workspaceFallback(input, state)
}

/**
 * 可见性不足时的统一回落：管理员回**自己看得见的**管理页、导师回导师工作台、
 * 其余回学员工作区。
 *
 * 管理员这一支不再是「回工作台」写死的那一条（#1638）：dashboard 自己也要能力位，
 * 一个能力集为空的账号会在 dashboard 上无限重定向，vue-router 中止导航 ⇒ 整页空白。
 * 落点改由 adminLandingPage 按**运行时能力集**算：仪表盘可达 → 仪表盘；
 * 否则侧栏顺序的第一个可达页；一个都没有 → 「无管理权限」页（真的会渲染）。
 */
function workspaceFallback(input: GuardInput, state: GuardState): GuardDecision {
  // #1651 / ADR-0075 —— 回落出口的子域身份不变式：父域 Cookie 让任何登录态在所有子域生效，
  // 回落决策若把用户送向「当前子域之外的工作区」，会被 subdomainBoundaryStep 升级成整页跳转
  // （tutor/admin 的 Cookie 开 training./recruit. 被弹去 mentor./manage.，即 #1614 的镜像）。
  // 出口①（身份落点异域）：目标落点子域 ≠ 当前子域，或 admin/tutor 的静态落点子域 ≠ 当前
  // 子域（他们在共号子域上本就是「别人」，落点再同域也是替别人选的路）→ 清登录态回本子域
  // 登录页并携带回跳，让用户就地换身份。
  if (!state.ipDirect && state.subdomain !== 'main') {
    const ws = workspaceOf(input)
    const landing = ws && ws !== 'auth' ? WORKSPACE_SUBDOMAIN[ws] : resolveTargetSubdomain(input)
    const homeSub = state.role === 'admin' ? 'admin' : state.role === 'tutor' ? 'tutor' : null
    if (landing !== state.subdomain || (homeSub && homeSub !== state.subdomain)) {
      return { action: 'redirect', to: { path: '/login', query: { redirect: input.fullPath } }, clearAuth: true }
    }
  }
  const decision: GuardDecision = (() => {
    if (state.role === 'admin') {
      const landing = adminLandingPage(capability =>
        holdsCapability(state.role, state.capabilities, capability)
      )
      return { action: 'redirect', to: { name: landing } }
    }
    if (state.role === 'tutor') {
      return { action: 'redirect', to: resolveWorkspaceForRole(state.role) }
    }
    // 其余 → 学员工作区
    return { action: 'redirect', to: '/training' }
  })()
  // 出口②（共号子域自绕）：recruiter/未知角色的回落落点就是 /training——若当前路径也是它，
  // 重跑守卫会再次进入本函数得到同一决策，vue-router 掐导航成空白页 → 同样清态回登录页。
  if (!state.ipDirect && state.subdomain !== 'main' && decision.to === input.path) {
    return { action: 'redirect', to: { path: '/login', query: { redirect: input.fullPath } }, clearAuth: true }
  }
  return decision
}

/**
 * 步骤 5b：能力校验（ADR-0047 §1/§2；meta.capability 由页面描述符注入）。
 *
 * 语义边界：能力是**角色资格**，只对「需要登录」的页面生效——公开页（requiresAuth 全 false）
 * 今天对任意角色开放，加能力位不得把 /ai-assistant 这类页面锁死。判据来自后端能力表的
 * codegen（config/authz.ts），前端不再各抄一份角色清单。
 */
export const capabilityStep: GuardStep = (input, state) => {
  const required = input.meta?.capability as AuthzCapability | undefined
  if (!required) return null
  const requiresAuth = input.matched.some(record => record.requiresAuth === true)
  if (!requiresAuth) return null
  // 两条路径收在 holdsCapability：动态角色（管理端）读运行时能力集，静态角色读生成的能力表。
  if (holdsCapability(state.role, state.capabilities, required)) return null
  return workspaceFallback(input, state)
}

/**
 * 步骤 6：无证件 onboarding 预筛（ADR-0020）——hrwai_user 在 training 工作区且未选证件时
 * 强制进 onboarding；已选证件访问 onboarding → 回 /training。IP 直连旁路。
 * 求值序前置：authRequiredStep 已对无需登录路由放行（预筛不适用于公开页）。
 * 证件未加载 → load-credential 决策交还 orchestrator；加载失败同样落 'none'
 * （orchestrator 的 state() 映射），走 onboarding 预筛——与既有行为一致。
 */
export const credentialStep: GuardStep = (input, state) => {
  if (state.role !== 'hrwai_user' || state.ipDirect) return null
  if (workspaceOf(input) !== 'training') return null
  if (state.credential === 'unloaded') return { action: 'load-credential' }
  const isOnboarding = input.name === CREDENTIAL_ONBOARDING
  if (state.credential === 'none' && !isOnboarding) {
    return { action: 'redirect', to: { name: CREDENTIAL_ONBOARDING } }
  }
  if (state.credential === 'present' && isOnboarding) {
    return { action: 'redirect', to: '/training' }
  }
  return null
}

/**
 * 决策管线：按既有守卫求值序执行各步骤，首个产出决策的步骤生效；全步骤放行 = allow。
 * 步骤顺序 = 行为承诺的一部分，不可随意调整（步骤 4 的插入决策见 ADR-0071）。
 */
export function resolveGuardDecision(input: GuardInput, state: GuardState): GuardDecision {
  const steps: GuardStep[] = [subdomainBoundaryStep, authPageStep, authRequiredStep, subdomainRoleStep, capabilityLoadStep, roleStep, capabilityStep, credentialStep]
  for (const step of steps) {
    const decision = step(input, state)
    if (decision) return decision
  }
  return { action: 'allow' }
}
