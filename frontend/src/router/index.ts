import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useCredentialStore } from '@/stores/credential'
import {
  getSubdomain,
  buildCrossDomainAuthUrl,
  getDefaultWorkspaceBySubdomain,
  isIpDirectMode
} from '@/utils/subdomain'
import { resolveGuardDecision, type GuardInput, type GuardState } from './guard'
import { resolveWorkspaceForRole } from '@/utils/authRedirect'
import { layouts, pages, href, type LayoutKey, type PageDescriptor } from '@/config/pages'

// workspace（工作区）声明约定（#618）：「这条路由属于哪个工作区」单点写在各布局/页面路由的
// meta.workspace（子路由经 vue-router meta 合并继承，无需逐条重复）；守卫与登录回跳读声明，
// 子域前缀表（authRedirect.PATH_AUTH_ENTRIES）只兜底未声明路径（404）。工作区语义而非子域名字面，
// 派生关系（workspace → 子域名）单点在 guard.WORKSPACE_SUBDOMAIN。
// redirect 型记录（/ /dashboard /tutor 等旧路径兼容）在路由解析期即被改写为目标路由，
// 不会成为守卫的 to，因此无需 workspace 声明。

// ===== 路由表由页面描述符派生（ADR-0047 §2 / spec #930）=====
// 顶层记录（无布局外壳）与各布局的子记录都由 config/pages.ts 的描述符生成；
// 本文件只保留「兼容旧路径」的重定向记录与守卫装配，不再逐个手写页面路由。

/** 子路由 path：绝对路径去掉布局前缀；布局根（path === basePath）为空串。 */
function childPath(page: PageDescriptor, basePath: string): string {
  if (page.path === basePath) return ''
  return page.path.startsWith(basePath + '/') ? page.path.slice(basePath.length + 1) : page.path
}

/** 描述符 → meta：布局子记录只写与父记录不同的部分（vue-router 会与父记录 meta 合并）。 */
function routeMeta(page: PageDescriptor): Record<string, unknown> {
  const meta: Record<string, unknown> = { workspace: page.workspace }
  // requiresAuth 由描述符**逐条必填**（ADR-0060 票8a）：不再依赖「缺省 + 父记录 meta 继承」，
  // 于是新增页面忘写声明从「静默匿名可访问」变成编译期报错。子记录写的值与它此前继承到的
  // 布局值一致，故 meta 内容零变化。
  meta.requiresAuth = page.requiresAuth
  if (page.authPage) meta.authPage = true
  if (page.isValuationAuthPage) meta.isValuationAuthPage = true
  if (page.roles) meta.roles = page.roles
  if (page.capability) meta.capability = page.capability
  return meta
}

/** 布局根的子路由重定向：没有对应页面描述符，但必须保留（URL 兼容）。 */
const layoutRedirects: Partial<Record<LayoutKey, RouteRecordRaw[]>> = {
  // 管理端根路径进仪表盘
  manage: [{ path: '', redirect: href('AdminDashboard') }],
  // 设计稿把估值表单提升为首页：/valuation/input 等同于 /valuation
  valuation: [{ path: 'input', redirect: href('ValuationHome') }]
}

/** 描述符 → 布局子记录。 */
function toChildRecord(page: PageDescriptor, basePath: string): RouteRecordRaw {
  return { path: childPath(page, basePath), name: page.name, component: page.component, meta: routeMeta(page) }
}

/** 布局记录 + 顶层记录（顺序：布局按 layouts 声明序，其后是顶层页）。 */
function buildRoutes(): RouteRecordRaw[] {
  const records: RouteRecordRaw[] = []
  for (const layout of layouts) {
    const meta: Record<string, unknown> = { requiresAuth: layout.requiresAuth, workspace: layout.workspace }
    if (layout.role) meta.role = layout.role
    const children: RouteRecordRaw[] = [...(layoutRedirects[layout.key] ?? [])]
    for (const page of pages.filter(p => p.layout === layout.key)) {
      children.push(toChildRecord(page, layout.basePath))
    }
    records.push({ path: layout.basePath, component: layout.component, meta, children })
  }
  for (const page of pages.filter(p => !p.layout)) {
    records.push({ path: page.path, name: page.name, component: page.component, meta: routeMeta(page) })
  }
  return records
}

/** 兼容旧路径的重定向记录（/、/dashboard/*、/tutor/*）。 */
const legacyRedirects: RouteRecordRaw[] = [
  // ========== 根路径兜底（IP 直连模式） ==========
  // 官网已重构为独立 Nuxt 仓库（ADR-0001），Vue SPA 不再承载 '/'；
  // IP 直连模式下根路径按角色进入默认工作区
  {
    path: '/',
    redirect: () => {
      // valuation 子域根路径 → 估值首页（公开，无需登录）；
      // 原逻辑会跳 /login，守卫再把 valuation 子域的登录页转成 /valuation/login
      if (getSubdomain() === 'valuation') return '/valuation'
      if (getSubdomain() === 'recruit') return '/recruit'
      const authStore = useAuthStore()
      const workspace = resolveWorkspaceForRole(authStore.userInfo?.role)
      // 未知角色 resolveWorkspaceForRole 返回 '/'，根路径按原逻辑回登录页
      return workspace === '/' ? '/login' : workspace
    }
  },

  // ========== 兼容旧路由 /dashboard/* ==========
  {
    path: '/dashboard',
    redirect: () => {
      const authStore = useAuthStore()
      return resolveWorkspaceForRole(authStore.userInfo?.role)
    }
  },
  {
    path: '/dashboard/:pathMatch(.*)*',
    redirect: to => {
      const authStore = useAuthStore()
      const subPath = (to.params.pathMatch as string[])?.[0] || ''

      // 特殊路径映射
      if (subPath === 'valuation' || subPath.startsWith('valuation/')) {
        return '/' + subPath
      }
      if (subPath === 'ai-generate') {
        return '/ai-assistant'
      }
      // 默认按角色跳转（单点函数 resolveWorkspaceForRole）
      return resolveWorkspaceForRole(authStore.userInfo?.role)
    }
  },

  // ========== 兼容旧路由 /tutor/* ==========
  {
    path: '/tutor',
    redirect: '/training/tutor'
  },
  {
    path: '/tutor/:pathMatch(.*)*',
    redirect: to => {
      const subPath = (to.params.pathMatch as string[])?.[0] || ''
      return subPath ? `/training/tutor/${subPath}` : '/training/tutor'
    }
  }
]

const routes: RouteRecordRaw[] = [...buildRoutes(), ...legacyRedirects]

const router = createRouter({
  history: createWebHistory(),
  routes
})

/**
 * 全局守卫 orchestrator（#618）：决策逻辑全部在 guard.ts 的纯函数决策管线
 * （(目标路由， 认证/环境状态) ⇒ 决策，全分支单测覆盖），这里只做三件事——
 * 1. 注入环境事实：等待认证初始化、投影 to 为 GuardInput、读取 auth/credential/子域名状态；
 * 2. 执行决策：next / 清登录态重定向 / 跨子域名整页跳转 / 当前子域名默认工作区；
 * 3. load-credential 决策时补拉一次证件数据后重跑管线（管线纯函数，重跑无副作用；
 *    loadCurrent 内部吞错，失败记为无证件，与既有行为一致）。
 */
router.beforeEach(async (to, _from, next) => {
  const authStore = useAuthStore()

  // 等待认证初始化完成（main.ts 显式启动，幂等；同一 Promise 等待不重复执行）
  await authStore.initialize()

  const credStore = useCredentialStore()
  let credentialLoadAttempted = false

  const input = (): GuardInput => ({
    path: to.path,
    fullPath: to.fullPath,
    name: to.name,
    meta: to.meta as Record<string, unknown>,
    // 逐条匹配记录的 meta（保持既有 some() 语义：requiresAuth/authPage 按逐条判断）
    matched: to.matched.map(record => record.meta as Record<string, unknown>)
  })

  const state = (): GuardState => ({
    isLoggedIn: authStore.isLoggedIn,
    role: authStore.userInfo?.role ?? '',
    hasValidToken: !!(authStore.token && authStore.isLoggedIn && authStore.userInfo && authStore.userInfo.role),
    subdomain: getSubdomain(),
    ipDirect: isIpDirectMode(),
    credential: !credStore.initialized ? 'unloaded' : credStore.current === null ? 'none' : 'present'
  })

  for (;;) {
    const decision = resolveGuardDecision(input(), state())
    switch (decision.action) {
      case 'allow':
        next()
        return
      case 'redirect':
        // clearAuth = token 失效口径：先清登录态再回认证页
        if (decision.clearAuth) authStore.clearAuthData()
        next(decision.to)
        return
      case 'external':
        // 跨子域名整页跳转（不同 origin，token 不共享；经 auth_token 参数交接登录态）
        window.location.href = buildCrossDomainAuthUrl(decision.target, decision.path)
        return
      case 'workspace-home':
        // 留在当前子域名的默认工作区（valuation/recruit 子域名有独立入口）
        next(getDefaultWorkspaceBySubdomain())
        return
      case 'load-credential':
        // 证件数据未加载：补拉一次后重跑管线；credentialLoadAttempted 防御性兜底死循环
        if (credentialLoadAttempted) {
          next()
          return
        }
        credentialLoadAttempted = true
        await credStore.loadCurrent()
        break
    }
  }
})

export default router
