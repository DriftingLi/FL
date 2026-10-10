// 标签栏组件（#1620）：路由变化开签、点击切签、关闭按钮、固定页无关闭按钮、固定页随能力集换页（#1638）。
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const h = vi.hoisted(() => ({
  route: { name: 'AdminDashboard', fullPath: '/admin/dashboard', meta: { workspace: 'manage' } },
  push: vi.fn(),
  // 当前账号的运行时能力集（#1618 段1）：固定页落点按它算（#1638）
  caps: ['admin.access'] as string[]
}))

vi.mock('vue-router', () => ({
  useRoute: () => h.route,
  useRouter: () => ({ push: h.push })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ userInfo: { role: 'admin' }, capabilities: h.caps })
}))

import { epLite } from '@/test/element-lite'
import AdminTabBar from '../AdminTabBar.vue'
import { useAdminTabsStore, PINNED_ADMIN_TAB } from '@/stores/adminTabs'

function mountBar() {
  return mount(AdminTabBar, { global: { plugins: [epLite()] } })
}

beforeEach(() => {
  vi.clearAllMocks()
  setActivePinia(createPinia())
  h.caps = ['admin.access']
  h.route = { name: 'AdminDashboard', fullPath: '/admin/dashboard', meta: { workspace: 'manage' } }
})

describe('AdminTabBar', () => {
  it('挂载即按当前路由开签（固定页）', () => {
    const w = mountBar()
    const store = useAdminTabsStore()
    expect(store.tabs.map(t => t.name)).toEqual([PINNED_ADMIN_TAB])
    expect(w.findAll('.admin-tab')).toHaveLength(1)
    // 固定页没有关闭按钮
    expect(w.find('.admin-tab-close').exists()).toBe(false)
  })

  it('固定页 = 账号的落点：没有仪表盘能力位时换成第一页可达页（#1638）', () => {
    h.caps = ['audit.read']
    const w = mountBar()
    const store = useAdminTabsStore()
    expect(store.tabs[0].name).toBe('AuditLogs')
    expect(store.tabs[0].pinned).toBe(true)
    expect(w.find('.admin-tab.is-pinned .admin-tab-title').text()).toBe('审计日志')
    expect(w.find('.admin-tab.is-pinned .admin-tab-close').exists()).toBe(false)
  })

  it('能力集为空 → 固定页是「无管理权限」页（而不是看不见的仪表盘）', () => {
    h.caps = []
    const w = mountBar()
    const store = useAdminTabsStore()
    // 固定页（恒在首位、不可关）换成无权限页：点它不会再被弹回来
    expect(store.tabs[0].name).toBe('AdminNoAccess')
    expect(store.tabs[0].pinned).toBe(true)
    expect(w.find('.admin-tab.is-pinned .admin-tab-title').text()).toBe('无管理权限')
    expect(w.find('.admin-tab.is-pinned .admin-tab-close').exists()).toBe(false)
  })

  it('点击非当前标签 → 路由到该标签', async () => {
    const store = useAdminTabsStore()
    store.open({ name: 'AuditLogs' as never, title: '审计日志', pinned: false })
    const w = mountBar()
    const tabs = w.findAll('.admin-tab')
    expect(tabs).toHaveLength(2)
    await tabs[1].trigger('click')
    expect(h.push).toHaveBeenCalledWith({ name: 'AuditLogs' })
  })

  it('关闭按钮移除标签并落到固定页', async () => {
    const store = useAdminTabsStore()
    store.open({ name: 'AuditLogs' as never, title: '审计日志', pinned: false })
    // 让当前路由就是待关的标签：挂载时 watcher 会把它激活（这正是要测的接线）
    h.route = { name: 'AuditLogs', fullPath: '/admin/audit-logs', meta: { workspace: 'manage' } }
    const w = mountBar()
    expect(store.activeName).toBe('AuditLogs')
    await w.findAll('.admin-tab-close')[0].trigger('click')
    expect(store.tabs.map(t => t.name)).toEqual([PINNED_ADMIN_TAB])
    expect(h.push).toHaveBeenCalledWith({ name: PINNED_ADMIN_TAB })
  })
})
