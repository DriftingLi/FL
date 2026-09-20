// 页面描述符一致性锁（ADR-0047 §2 / spec #930 决策 5）。
//
// 描述符是页面的事实源，路由表与导航树都从它派生——所以「三者互相说得通」必须是一条测试，
// 而不是靠人记得同步。这里锁四件事：
//   1. 每个描述符都真的出现在路由表里，且 path 逐字一致（漏接 = 页面静默消失）；
//   2. 路由表里带 name 的记录都来自描述符（手写孤儿路由 = 又一处事实源）；
//   3. 描述符 name / path 无重复（重复会被 vue-router 静默覆盖）；
//   4. 导航项的 routeName / activeRouteNames 都能在路由表里找到（导航断链不可能上线）。
//
// 注：navigation.spec.ts 里的 EXPECTED 映射表**保留**——它是本次改造的零漂移证据
// （证明派生结果与改造前手写菜单逐项一致）。这里新增的是「三者同源」的结构锁，两者互补。
import { describe, it, expect } from 'vitest'
import router from '@/router'
import { pages, layouts, navGroups, navPages, externalNavItems, type Workspace } from '../pages'
import { buildNavigation } from '../navigation'

const namedRoutes = router.getRoutes().filter(r => !!r.name)
const routeByName = new Map(namedRoutes.map(r => [String(r.name), r]))

describe('页面描述符 ↔ 路由表', () => {
  it('每个描述符都在路由表里，且 path 逐字一致', () => {
    for (const page of pages) {
      const record = routeByName.get(page.name)
      expect(record, '描述符 ' + page.name + ' 未出现在路由表中').toBeTruthy()
      expect(record!.path, '描述符 ' + page.name + ' 的 path 与路由表不一致').toBe(page.path)
    }
  })

  it('路由表里带 name 的记录都来自描述符（没有手写孤儿路由）', () => {
    const declared = new Set(pages.map(p => p.name as string))
    const orphans = namedRoutes.filter(r => !declared.has(String(r.name))).map(r => String(r.name))
    expect(orphans, '这些路由不在描述符表里：' + orphans.join(', ')).toEqual([])
  })

  it('描述符 name 与 path 都不重复', () => {
    const names = pages.map(p => p.name as string)
    const paths = pages.map(p => p.path)
    expect(names.length).toBe(new Set(names).size)
    expect(paths.length).toBe(new Set(paths).size)
  })

  it('布局记录的工作区与布局声明一致', () => {
    for (const layout of layouts) {
      // 同一 path 可能有多条记录（布局根 + 空路径重定向子记录）；布局记录是带 workspace meta 的那条
      const records = router.getRoutes().filter(r => r.path === layout.basePath && !r.name)
      const record = records.find(r => r.meta && r.meta.workspace !== undefined)
      expect(record, '布局 ' + layout.key + ' 未注册（或缺少 workspace meta）').toBeTruthy()
      expect(record!.meta.workspace).toBe(layout.workspace)
      if (layout.role) expect(record!.meta.role).toBe(layout.role)
    }
  })

  it('描述符声明的布局都存在', () => {
    const keys = new Set(layouts.map(l => l.key))
    for (const page of pages) {
      if (page.layout) expect(keys.has(page.layout), page.name + ' 指向未知布局 ' + page.layout).toBe(true)
    }
  })
})

describe('导航树 ↔ 路由表', () => {
  const workspaces: Workspace[] = ['training', 'tutor', 'manage', 'recruit']

  it('导航项的 routeName / activeRouteNames 都能在路由表里找到', () => {
    for (const ws of workspaces) {
      const walk = (items: ReturnType<typeof buildNavigation>) => {
        for (const item of items) {
          if (item.routeName) {
            expect(routeByName.has(String(item.routeName)), ws + ' 导航项 ' + item.key + ' 指向不存在的路由 ' + item.routeName).toBe(true)
          }
          for (const extra of item.activeRouteNames ?? []) {
            expect(routeByName.has(String(extra)), ws + ' 导航项 ' + item.key + ' 的详情页归属 ' + extra + ' 不存在').toBe(true)
          }
          const isGroupNode = !!item.children?.length
          if (!isGroupNode && !item.externalUrl) {
            expect(item.routeName, ws + ' 导航叶子 ' + item.key + ' 既无路由也不是外链').toBeTruthy()
          }
          if (item.children?.length) walk(item.children)
        }
      }
      walk(buildNavigation(ws))
    }
  })

  it('导航项 key 在同一工作区内唯一（DOM key 冲突会让侧栏错乱）', () => {
    for (const ws of workspaces) {
      const keys: string[] = []
      const walk = (items: ReturnType<typeof buildNavigation>) => {
        for (const item of items) {
          keys.push(item.key)
          if (item.children?.length) walk(item.children)
        }
      }
      walk(buildNavigation(ws))
      expect(keys.length, ws + ' 导航 key 有重复：' + keys.join(', ')).toBe(new Set(keys).size)
    }
  })

  it('分组工作区的每个分组都非空，且分组顺序与 navGroups 声明一致', () => {
    for (const ws of workspaces) {
      const groups = navGroups[ws]
      if (!groups) continue
      const derived = buildNavigation(ws).map(g => g.key)
      expect(derived).toEqual(groups.map(g => g.key))
    }
  })

  it('声明了 nav 的页面都能在派生导航里找到对应项', () => {
    for (const ws of workspaces) {
      const keys = new Set<string>()
      const walk = (items: ReturnType<typeof buildNavigation>) => {
        for (const item of items) {
          keys.add(item.key)
          if (item.children?.length) walk(item.children)
        }
      }
      walk(buildNavigation(ws))
      for (const page of navPages(ws)) {
        expect(keys.has(page.name), ws + ' 的页面 ' + page.name + ' 声明了 nav 但没进导航树').toBe(true)
      }
    }
  })

  it('外链导航项只在声明的工作区出现，且带 externalUrl', () => {
    for (const [ws, items] of Object.entries(externalNavItems)) {
      const keys = new Set<string>()
      const walk = (list: ReturnType<typeof buildNavigation>) => {
        for (const item of list) {
          keys.add(item.key)
          if (item.children?.length) walk(item.children)
        }
      }
      walk(buildNavigation(ws as Workspace))
      for (const ext of items) {
        expect(ext.externalUrl.startsWith('http'), '外链项 ' + ext.key + ' 缺 externalUrl').toBe(true)
        expect(keys.has(ext.key), '外链项 ' + ext.key + ' 未出现在导航树').toBe(true)
      }
    }
  })
})

// 公开页面清单锁（ADR-0060 票8a）。requiresAuth 改为必填后，「哪些页面匿名可达」第一次
// 成为一份可逐名核对的清单，而不是一句「缺省即需登录」加上谁都可能漏写的一行：
// 新增公开页必须同时改这张表（评审时看得见），忘写 requiresAuth 则根本编译不过。
const PUBLIC_PAGES = [
  'AIAssistant',
  'AIAssistantFeature',
  'ForgotPassword',
  'Login',
  'Register',
  'ValuationBatteryInput',
  'ValuationBatteryResult',
  'ValuationForgotPassword',
  'ValuationHome',
  'ValuationLogin',
  'ValuationRegister',
  'ValuationReport',
  'ValuationResult'
]

describe('公开页面清单与 requiresAuth 必填', () => {
  it('匿名可达页面 = 登记清单，逐名相等（多一个少一个都算漂移）', () => {
    const actual = pages
      .filter(p => !p.requiresAuth)
      .map(p => String(p.name))
      .sort()
    expect(actual, '匿名可达页面发生漂移').toEqual([...PUBLIC_PAGES].sort())
  })

  it('每条描述符都显式声明 requiresAuth，且路由 meta 与之一致', () => {
    for (const page of pages) {
      expect(typeof page.requiresAuth, page.name + ' 未显式声明 requiresAuth').toBe('boolean')
      expect(routeByName.get(page.name)!.meta.requiresAuth, page.name + ' 的路由 meta 与描述符不一致').toBe(page.requiresAuth)
    }
  })

  it('挂在公开布局下却要求登录的页面，只有显式写出来的那一条', () => {
    const publicLayouts = new Set(layouts.filter(l => !l.requiresAuth).map(l => l.key))
    const protectedUnderPublic = pages.filter(p => p.layout && publicLayouts.has(p.layout) && p.requiresAuth)
    expect(protectedUnderPublic.map(p => String(p.name))).toEqual(['ValuationHistory'])
  })
})
