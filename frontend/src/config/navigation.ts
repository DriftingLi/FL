import type { Component } from 'vue'
import type { RouteName } from './pages'
import {
  navGroups,
  navPages,
  externalNavItems,
  type PageDescriptor,
  type Workspace
} from './pages'
// 注：MagicStick 仍用于管理员"内容生成"菜单项

export interface NavItem {
  key: string
  label: string
  /** 目标路由的 name（页面描述符表派生的 RouteName union，而非硬编码字符串）。 */
  routeName?: RouteName
  /**
   * 除 `routeName` 外，还应让本项高亮的路由 name。
   *
   * 用于「列表页 → 详情页」这类**详情页没有独立导航项**的场景：
   * ForumPage → ForumDetail、CourseList → ChapterView、
   * TutorCourses → TutorChapterManage / TutorChapterEdit、
   * TutorQuestionManage → TutorQuestionCreate。
   * 不配的话，进入详情后侧栏整条（含父分组）都不高亮。
   */
  activeRouteNames?: readonly RouteName[]
  /** 目标路由需要的动态参数（如章节页 ChapterView 需要 courseId/chapterId）。 */
  routeParams?: Record<string, string | number>
  icon?: Component
  children?: NavItem[]
  /** 外链地址（用于内容精选等跨子域跳转） */
  externalUrl?: string
  // exact=true 时仅精确匹配 route.name 才高亮。
  // name 匹配天然精确，因此该标记保留以兼容既有语义，但不再参与前缀判断。
  exact?: boolean
}

type RouteParamsLike = Record<string, string | string[] | undefined>

/**
 * 判断当前路由是否命中某个导航项。
 *
 * 抽成纯函数的原因：侧栏高亮出过两次线上问题（一次双高亮、一次详情页不高亮），
 * 而 `isRouteActive` 原先是 `AppSidebar.vue` 里的私有函数，无法单测覆盖。
 *
 * 匹配规则：
 * 1. `routeName` 精确相等，或命中 `activeRouteNames` 中任一名字；
 * 2. 若声明了 `routeParams`，需再逐个比对 params —— 避免同名路由（如不同课程的
 *    章节页）被一起点亮。
 */
export function isNavRouteActive(
  item: NavItem,
  routeName: unknown,
  routeParams?: RouteParamsLike
): boolean {
  if (!item.routeName) return false

  const names = [item.routeName, ...(item.activeRouteNames ?? [])].map(String)
  if (!names.includes(String(routeName ?? ''))) return false

  if (item.routeParams) {
    for (const [k, v] of Object.entries(item.routeParams)) {
      const actual = routeParams?.[k]
      const actualValue = Array.isArray(actual) ? actual[0] : actual
      if (String(actualValue ?? '') !== String(v)) return false
    }
  }
  return true
}

// ===== 导航树由页面描述符派生（ADR-0047 §2 / spec #930）=====
// 分组标签与图标来自 navGroups，叶子项来自 pages 的 nav 字段；分组工作区按组装配，
// 无分组工作区（导师/招聘）为扁平清单。侧栏不再各自维护菜单清单。

/** 描述符 → 导航项（key 用路由名，外链项见 externalNavItems）。 */
function toNavItem(page: PageDescriptor): NavItem {
  const nav = page.nav!
  return {
    key: page.name,
    label: nav.label,
    routeName: page.name,
    activeRouteNames: nav.activeRouteNames,
    routeParams: nav.routeParams,
    icon: nav.icon,
    exact: nav.exact
  }
}

/** 组内装配：先放顶层项，再把声明了 parent 的项挂到父项之下（父项缺失时降级为顶层，避免静默丢项）。 */
function buildGroupChildren(entries: PageDescriptor[], workspace: Workspace, group: string): NavItem[] {
  const out: NavItem[] = []
  const byName = new Map<RouteName, NavItem>()
  for (const page of entries.filter(e => !e.nav!.parent)) {
    const item = toNavItem(page)
    byName.set(page.name, item)
    out.push(item)
  }
  for (const page of entries.filter(e => !!e.nav!.parent)) {
    const parent = byName.get(page.nav!.parent!)
    if (!parent) {
      out.push(toNavItem(page))
      continue
    }
    parent.children = [...(parent.children ?? []), toNavItem(page)]
  }
  for (const ext of externalNavItems[workspace] ?? []) {
    if (ext.group !== group) continue
    out.push({ key: ext.key, label: ext.label, icon: ext.icon, externalUrl: ext.externalUrl })
  }
  return out
}

/** 派生某工作区的导航树（按 nav.order 排序；分组工作区按 navGroups 组装）。 */
export function buildNavigation(workspace: Workspace): NavItem[] {
  const entries = navPages(workspace)
  const groups = navGroups[workspace]
  if (!groups) {
    return entries
      .filter(e => !e.nav!.parent)
      .slice()
      .sort((a, b) => (a.nav!.order ?? 0) - (b.nav!.order ?? 0))
      .map(toNavItem)
  }
  return groups
    .map(group => {
      const inGroup = entries
        .filter(e => e.nav!.group === group.key)
        .slice()
        .sort((a, b) => (a.nav!.order ?? 0) - (b.nav!.order ?? 0))
      return {
        key: group.key,
        label: group.label,
        icon: group.icon,
        children: buildGroupChildren(inGroup, workspace, group.key)
      } as NavItem
    })
    .filter(group => (group.children?.length ?? 0) > 0)
}

export const studentNav: NavItem[] = buildNavigation("training")
export const tutorNav: NavItem[] = buildNavigation("tutor")
export const adminNav: NavItem[] = buildNavigation("manage")
export const recruiterNav: NavItem[] = buildNavigation("recruit")

export const roleNavigation: Record<string, NavItem[]> = {
  student: studentNav,
  admin: adminNav,
  tutor: tutorNav,
  recruiter: recruiterNav
}

// ===== 侧栏分组判定（纯函数；ADR-0047 §2 / spec #930 决策 6）=====
//
// 这三条判定原先长在 AppSidebar.vue 的编排里（当时零测试）。它们与 isNavRouteActive 是同一族：
// 侧栏出过的两次线上问题都发生在「判定与编排混在一起」的地方，故一并抽成纯函数。
// 判定的测试面在 config/__tests__/navigation.spec.ts（两次故障 + 两层深度上限与分组高亮的
// 结构性不对称，逐条钉住）；项级 markup 自第十三波 票9 起收在 components/layout/AppSidebarItem.vue。

/** 分组展开态：默认展开，只有显式为 false 才收起（与组件原语义逐字一致）。 */
export function isGroupExpanded(map: Record<string, boolean>, key: string): boolean {
  return map[key] !== false
}

/** 切换分组展开态：返回新对象（不原地修改，便于测试与响应式追踪）。 */
export function toggleGroupExpanded(
  map: Record<string, boolean>,
  key: string
): Record<string, boolean> {
  return { ...map, [key]: !isGroupExpanded(map, key) }
}

/** 收集分组下的全部叶子项（最多两层一级子级，与侧栏结构一致；无 routeName/externalUrl 的容器跳过）。 */
/**
 * 该导航项是否「有落点」（可渲染成一行），还是只是一个分组标题。
 *
 * 判据的唯一宿主：侧栏父组件此前在 v-if 里各写一遍 `externalUrl || routeName`，
 * 与 AppSidebarItem 的 kind 判别同源不同处（ADR-0060 票9：判定面只在 navigation.ts，
 * 渲染只在 AppSidebarItem）。
 */
export function isNavItemRenderable(item: NavItem): boolean {
  return !!(item.externalUrl || item.routeName)
}

export function flattenLeaves(item: NavItem): NavItem[] {
  const result: NavItem[] = []
  for (const child of item.children || []) {
    if (child.routeName || child.externalUrl) result.push(child)
    if (child.children?.length) {
      for (const sub of child.children) {
        if (sub.routeName || sub.externalUrl) result.push(sub)
      }
    }
  }
  return result
}

/** 分组是否激活：任一子项（含二层）命中当前路由即点亮整条分组。 */
export function isGroupActive(
  item: NavItem,
  routeName: unknown,
  routeParams?: RouteParamsLike
): boolean {
  if (!item.children?.length) return false
  for (const child of item.children) {
    if (child.children?.length) {
      if (isNavRouteActive(child, routeName, routeParams)) return true
      for (const sub of child.children) {
        if (isNavRouteActive(sub, routeName, routeParams)) return true
      }
    } else if (isNavRouteActive(child, routeName, routeParams)) {
      return true
    }
  }
  return false
}
