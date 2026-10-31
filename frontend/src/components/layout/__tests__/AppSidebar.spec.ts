// 侧栏分组头：只出文字，不再复用叶子图标（#2150 反馈）。
// 折叠态也不能换回图标 —— 分组头退化为分隔线，保留分组边界。
import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { h } from 'vue'
import type { NavItem } from '@/config/navigation'

vi.mock('vue-router', () => ({
  useRoute: () => ({ name: 'CourseList', params: {} })
}))

import AppSidebar from '../AppSidebar.vue'

const menuItems: NavItem[] = [
  {
    key: 'study',
    label: '学习中心',
    icon: () => h('span', { class: 'group-test-icon' }),
    children: [
      { key: 'dashboard', label: '仪表盘', routeName: 'CourseList', icon: () => h('span') }
    ]
  }
] as NavItem[]

function mountSidebar(collapsed = false) {
  return mount(AppSidebar, {
    props: { menuItems, collapsed },
    global: {
      stubs: { RouterLink: { template: '<a><slot /></a>' } }
    }
  })
}

describe('侧栏分组头（AppSidebar）', () => {
  it('展开态只渲染分组文字，不渲染分组图标', () => {
    const w = mountSidebar(false)
    const header = w.find('.sidebar-nav .nav-group-label')
    expect(header.exists()).toBe(true)
    expect(header.find('.nav-group-text').text()).toBe('学习中心')
    expect(header.find('.nav-group-icon').exists()).toBe(false)
    expect(header.find('.group-test-icon').exists()).toBe(false)
    w.unmount()
  })

  it('折叠态分组头仍在（退化成分隔线），也不复活图标', async () => {
    const w = mountSidebar(true)
    expect(w.find('.app-sidebar').classes()).toContain('collapsed')
    const header = w.find('.sidebar-nav .nav-group-label')
    expect(header.exists()).toBe(true)
    expect(header.find('.nav-group-icon').exists()).toBe(false)
    expect(header.find('.group-test-icon').exists()).toBe(false)
    w.unmount()
  })
})
