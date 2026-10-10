// 管理端标签页 store 行为锁定（#1620）：固定页、会话内记忆、关闭落点、详情页归属。
import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAdminTabsStore, PINNED_ADMIN_TAB } from '@/stores/adminTabs'
import { pageTitleOf, tabOwnerOf } from '@/config/navigation'
import { pages } from '@/config/pages'

function tab(name: string, title = name, pinned = false) {
  return { name: name as never, title, pinned }
}

beforeEach(() => {
  setActivePinia(createPinia())
})

describe('打开与激活', () => {
  it('固定页恒在首位，重复打开不新增', () => {
    const s = useAdminTabsStore()
    s.open(tab('AuditLogs'))
    s.open(tab(PINNED_ADMIN_TAB, '仪表盘', true))
    s.open(tab(PINNED_ADMIN_TAB, '仪表盘', true))
    expect(s.tabs.map(t => t.name)).toEqual([PINNED_ADMIN_TAB, 'AuditLogs'])
    expect(s.activeName).toBe(PINNED_ADMIN_TAB)
  })

  it('同路由再打开只刷新标题（详情页带出的名字）', () => {
    const s = useAdminTabsStore()
    s.open(tab('AdminFeaturedContentEdit', '新建精选'))
    s.open(tab('AdminFeaturedContentEdit', '精选：某标题'))
    expect(s.tabs).toHaveLength(1)
    expect(s.tabs[0].title).toBe('精选：某标题')
  })
})

describe('关闭与落点', () => {
  it('关闭当前标签 → 落到右邻，再落到左邻', () => {
    const s = useAdminTabsStore()
    s.open(tab(PINNED_ADMIN_TAB, '仪表盘', true))
    s.open(tab('AuditLogs'))
    s.open(tab('AISettings'))
    s.activeName = 'AuditLogs'
    expect(s.close('AuditLogs')).toBe('AISettings') // 右邻
    s.activeName = 'AISettings'
    expect(s.close('AISettings')).toBe(PINNED_ADMIN_TAB) // 左邻 = 固定页
  })

  it('固定页不可关（返回 null，列表不变）', () => {
    const s = useAdminTabsStore()
    s.open(tab(PINNED_ADMIN_TAB, '仪表盘', true))
    expect(s.close(PINNED_ADMIN_TAB)).toBeNull()
    expect(s.tabs).toHaveLength(1)
  })

  it('关闭非当前标签不动当前落点', () => {
    const s = useAdminTabsStore()
    s.open(tab('AuditLogs'))
    s.open(tab('AISettings'))
    s.activeName = 'AISettings'
    expect(s.close('AuditLogs')).toBeNull()
    expect(s.activeName).toBe('AISettings')
  })

  it('关闭其他保留固定页与当前页；关闭全部只留固定页', () => {
    const s = useAdminTabsStore()
    s.open(tab(PINNED_ADMIN_TAB, '仪表盘', true))
    s.open(tab('AuditLogs'))
    s.open(tab('AISettings'))
    s.closeOthers('AuditLogs')
    expect(s.tabs.map(t => t.name)).toEqual([PINNED_ADMIN_TAB, 'AuditLogs'])
    expect(s.closeAll()).toBe(PINNED_ADMIN_TAB)
    expect(s.tabs.map(t => t.name)).toEqual([PINNED_ADMIN_TAB])
  })

  it('reset 清空（登出即清，标签是登录态的工作集）', () => {
    const s = useAdminTabsStore()
    s.open(tab('AuditLogs'))
    s.reset()
    expect(s.tabs).toEqual([])
    expect(s.activeName).toBe('')
  })
})

describe('路由 → 标签（详情页并入所属标签）', () => {
  // fixture 与生产同形：router/index.ts 的 routeMeta **不注入 title**。
  // （此前 fixture 自己塞了 meta.title，于是「标签印英文路由名」这条线上缺陷在测试里看不见 ——
  //   #1630 的根因之一就是用例替生产补了一个生产不存在的输入。）
  const routeOf = (name: string, workspace = 'manage') =>
    ({ name, fullPath: '/x', meta: { workspace } }) as never

  it('普通管理页自成标签，标题取描述符的中文名', () => {
    const s = useAdminTabsStore()
    const t = s.resolveTab(routeOf('AuditLogs'), tabOwnerOf, pageTitleOf)
    expect(t).toMatchObject({ name: 'AuditLogs', title: '审计日志', pinned: false })
  })

  it('详情页并入其列表页（归属来自描述符的 nav.activeRouteNames）', () => {
    const owner = tabOwnerOf('AdminFeaturedContentEdit')
    expect(owner).toBe('AdminFeaturedContentList')
    const s = useAdminTabsStore()
    expect(s.resolveTab(routeOf('AdminFeaturedContentEdit'), tabOwnerOf, pageTitleOf)?.name).toBe(
      'AdminFeaturedContentList'
    )
  })

  it('仪表盘解析为固定页；非管理端工作区不产生标签', () => {
    const s = useAdminTabsStore()
    expect(s.resolveTab(routeOf(PINNED_ADMIN_TAB), tabOwnerOf, pageTitleOf)?.pinned).toBe(true)
    expect(s.resolveTab(routeOf('CourseList', 'training'), tabOwnerOf, pageTitleOf)).toBeNull()
  })
})

describe('标签标题一律中文（#1630）', () => {
  const CJK = /[\u4e00-\u9fa5]/

  it('管理端每个「自成标签」的页面都有中文标题，且不是英文路由名', () => {
    const standalone = pages.filter(p => p.workspace === 'manage' && !tabOwnerOf(p.name))
    expect(standalone.length).toBeGreaterThan(10)
    for (const page of standalone) {
      const title = pageTitleOf(page.name)
      expect(title, `${page.name} 的标签标题`).toMatch(CJK)
      expect(title, `${page.name} 的标签标题`).not.toBe(page.name)
    }
  })
})
