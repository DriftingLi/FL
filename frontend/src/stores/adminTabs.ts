import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { RouteLocationNormalizedLoaded } from 'vue-router'
import type { RouteName } from '@/config/pages'

/**
 * 管理端标签页（#1620）。
 *
 * 形态与边界（ADR-0073 的决策）：
 * - **固定页**：不可关的保底返回点。默认是仪表盘；**没有仪表盘能力位的账号**它的固定页是
 *   「自己看得见的第一页」（#1638，取值由调用方注入，见 resolveTab 的 pinnedName）；
 * - **会话内记忆**：只活在 pinia 里，**不写 localStorage** —— 标签是会话工作集，刷新后
 *   上下文（滚动、筛选）本来也不保，持久化只会让「刷新后一堆标签指向旧状态」；
 * - **详情页并入所属标签**：页面描述符已有 `nav.activeRouteNames`（列表页声明「哪些详情页
 *   属于我」），标签据此反查归属，不重复开签；
 * - 关闭当前标签后落到**右邻 → 左邻 → 固定页**，与浏览器标签页的直觉一致。
 */
export interface AdminTab {
  /** 标签身份 = 路由名（同一路由的不同参数视为同一标签，详情页并入所属列表） */
  name: RouteName
  /** 标题（页面描述符的 nav.label；不读 route.meta.title，#1630） */
  title: string
  /** 是否为固定页（不可关闭） */
  pinned: boolean
}

/**
 * 固定页的**默认**取值：仪表盘。
 *
 * #1638 起固定页不再恒定是这一页：没有仪表盘能力位的账号，其固定页 = 「自己看得见的第一页」
 * （由 config/navigation 的 adminLandingPage 算，与守卫的能力回落**同源**）。
 * 本常量因此只作为默认值与测试夹具，真实取值由调用方（AdminTabBar）注入。
 */
export const PINNED_ADMIN_TAB: RouteName = 'AdminDashboard'

export const useAdminTabsStore = defineStore('adminTabs', () => {
  const tabs = ref<AdminTab[]>([])
  const activeName = ref<RouteName | ''>('')

  const closable = computed(() => tabs.value.filter(t => !t.pinned))
  const canCloseOthers = computed(() => closable.value.length > 1)

  /** 打开（或激活）一个标签：固定页恒在列表首位。 */
  function open(tab: AdminTab): void {
    activeName.value = tab.name
    const existing = tabs.value.find(t => t.name === tab.name)
    if (existing) {
      // 标题可能随页面数据变化（如详情页带出课程名），以最新一次为准
      if (tab.title && existing.title !== tab.title) existing.title = tab.title
      return
    }
    if (tab.pinned) {
      // 固定页恒在首位：按**标志**摘掉旧的固定页，而不是按某个写死的名字 ——
      // 固定页的取值依账号能力集而变（#1638），名字比较会漏掉「固定页换了一页」的情形。
      tabs.value = [tab, ...tabs.value.filter(t => !t.pinned && t.name !== tab.name)]
      return
    }
    tabs.value = [...tabs.value, tab]
  }

  /** 关闭一个标签；固定页不可关。返回关闭后的落点（未关闭任何标签时为 null）。 */
  function close(name: RouteName): RouteName | null {
    const idx = tabs.value.findIndex(t => t.name === name)
    if (idx < 0 || tabs.value[idx].pinned) return null
    tabs.value = tabs.value.filter(t => t.name !== name)
    if (activeName.value !== name) return null
    const fallback = tabs.value[idx] ?? tabs.value[idx - 1] ?? tabs.value.find(t => t.pinned)
    activeName.value = fallback ? fallback.name : ''
    return activeName.value || null
  }

  /** 关闭其它（保留固定页与当前页）。 */
  function closeOthers(keep: RouteName): void {
    tabs.value = tabs.value.filter(t => t.pinned || t.name === keep)
    activeName.value = keep
  }

  /** 关闭全部可关标签（固定页留下）。 */
  function closeAll(): RouteName | null {
    tabs.value = tabs.value.filter(t => t.pinned)
    const fallback = tabs.value[0]
    activeName.value = fallback ? fallback.name : ''
    return activeName.value || null
  }

  /** 登出/切换身份时清空（标签是登录态的工作集）。 */
  function reset(): void {
    tabs.value = []
    activeName.value = ''
  }

  /**
   * 由路由解析出标签身份与标题（详情页并入所属标签）。
   *
   * 归属判据取描述符的 `nav.activeRouteNames`（列表页自己声明哪些详情页属于它），
   * 不在这里另写一张「详情页 → 列表页」的表 —— 那是第二份真源。
   *
   * 标题也**不读 `route.meta.title`**（#1630）：路由记录从来没注入过 meta.title
   * （看 router/index.ts 的 routeMeta），于是标签上印的是英文路由名 —— 线上形态是
   * 「仪表盘 | AuditLogs | AdminAccountManage」。标题的唯一来源改为注入的 `titleOf`
   * （= config/navigation 的 pageTitleOf → 描述符的 nav.label），与固定页同源。
   *
   * `pinnedName`（#1638）：哪一页算固定页由调用方给（按账号能力集算的落点），
   * 缺省才是仪表盘 —— 一个能力位都没有的账号，固定页就是「无管理权限」页。
   */
  function resolveTab(
    route: RouteLocationNormalizedLoaded,
    ownerOf: (name: string) => RouteName | null,
    titleOf: (name: RouteName) => string,
    pinnedName: RouteName = PINNED_ADMIN_TAB
  ): AdminTab | null {
    if (route.meta?.workspace !== 'manage' || typeof route.name !== 'string') return null
    const owner = ownerOf(route.name)
    const name = (owner ?? route.name) as RouteName
    return { name, title: titleOf(name), pinned: name === pinnedName }
  }

  return {
    tabs,
    activeName,
    closable,
    canCloseOthers,
    open,
    close,
    closeOthers,
    closeAll,
    reset,
    resolveTab
  }
})
