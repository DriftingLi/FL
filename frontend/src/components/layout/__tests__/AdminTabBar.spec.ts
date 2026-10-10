// 标签栏组件（#1620）：路由变化开签、点击切签、关闭按钮、固定页无关闭按钮。
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const h = vi.hoisted(() => ({
  route: { name: 'AdminDashboard', fullPath: '/admin/dashboard', meta: { workspace: 'manage', title: '仪表盘' } },
  push: vi.fn()
}))

vi.mock('vue-router', () => ({
  useRoute: () => h.route,
  useRouter: () => ({ push: h.push })
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
  h.route = { name: 'AdminDashboard', fullPath: '/admin/dashboard', meta: { workspace: 'manage', title: '仪表盘' } }
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
    h.route = { name: 'AuditLogs', fullPath: '/admin/audit-logs', meta: { workspace: 'manage', title: '审计日志' } }
    const w = mountBar()
    expect(store.activeName).toBe('AuditLogs')
    await w.findAll('.admin-tab-close')[0].trigger('click')
    expect(store.tabs.map(t => t.name)).toEqual([PINNED_ADMIN_TAB])
    expect(h.push).toHaveBeenCalledWith({ name: PINNED_ADMIN_TAB })
  })
})
