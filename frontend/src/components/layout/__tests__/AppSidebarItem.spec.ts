// AppSidebarItem：侧栏「一行」的唯一渲染实现（ADR-0060 票9，9 份 markup 收成 1 份）。
// seam：组件 props → 渲染出的行。票9 的承诺是「零视觉变更」，而当时的比对是一次性 harness
// （跑完即删）——那条平价需要常驻，否则后续改侧栏的人拿不到「三层 × 外链/路由/占位」的回归网。
// 这里锁的是**判据归属**：kind 三分支、active 只可能落在路由行上、层级只改 class 不改标签。
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import AppSidebarItem from '../AppSidebarItem.vue'
import type { NavItem } from '@/config/navigation'

function item(over: Partial<NavItem> = {}): NavItem {
  return { key: 'k1', label: '课程中心', routeName: 'CourseList', ...over } as NavItem
}

function mk(props: Record<string, unknown>) {
  return mount(AppSidebarItem, {
    props: { item: item(), collapsed: false, active: false, level: 1, ...props },
    global: {
      mocks: { $router: { push: () => undefined }, $route: { name: 'CourseList', params: {}, path: '/x' } },
      stubs: { RouterLink: { props: ['to'], template: '<a><slot /></a>' }, ElIcon: true }
    }
  })
}

describe('侧栏导航行（AppSidebarItem）', () => {
  it('有 routeName → 渲染 router-link，标签文字与图标位都在', () => {
    const w = mk({})
    expect(w.find('a').exists()).toBe(true)
    expect(w.find('a').classes()).toContain('nav-item')
    expect(w.text()).toContain('课程中心')
    expect(w.find('.nav-item-icon').exists()).toBe(true)
  })

  it('externalUrl 优先于 routeName → 渲染普通 <a>，且带 noopener', () => {
    const w = mk({ item: item({ externalUrl: 'https://example.com', routeName: undefined }) })
    const a = w.find('a')
    expect(a.attributes('href')).toBe('https://example.com')
    expect(a.attributes('target')).toBe('_blank')
    expect(a.attributes('rel')).toBe('noopener')
  })

  it('外链行永远拿不到 active（高亮判据只属于路由行——双高亮故障的那一半）', () => {
    const w = mk({ item: item({ externalUrl: 'https://example.com' }), active: true })
    expect(w.find('a').classes()).not.toContain('active')
  })

  it('active=true 的路由行才带 active class', () => {
    expect(mk({ active: true }).find('a').classes()).toContain('active')
    expect(mk({ active: false }).find('a').classes()).not.toContain('active')
  })

  it('level 只换 class，不换标签（三层共用同一份 markup）', () => {
    for (const level of [1, 2, 3] as const) {
      const w = mk({ level })
      expect(w.find('a').exists()).toBe(true)
      if (level === 3) expect(w.find('a').classes()).toContain('nav-sub-item')
      else expect(w.find('a').classes()).not.toContain('nav-sub-item')
    }
  })

  it('既无 routeName 又无 externalUrl：fallback 决定是占位行还是 router-link 兜底', () => {
    const inert = mk({ item: item({ routeName: undefined }), fallback: 'inert' })
    expect(inert.find('a').attributes('href')).toBe('#')
    const link = mk({ item: item({ routeName: undefined }), fallback: 'link' })
    expect(link.find('a').exists()).toBe(true)
    expect(link.find('a').attributes('href')).not.toBe('#')
  })

  it('collapsed 时文字标签保留在 DOM、交给透明度/布局过渡收起（不再 v-if 瞬删）', () => {
    const w = mk({ collapsed: true })
    const label = w.find('.nav-item-label')
    // 保留 DOM 是「宽度过渡期间文字不会第一帧就消失」的前提；视觉收起由 .collapsed CSS 负责
    expect(label.exists()).toBe(true)
    expect(label.attributes('aria-hidden')).toBe('true')
    // 读屏名由行级 aria-label 接管（可见文字被 aria-hidden）
    expect(w.find('a').attributes('aria-label')).toBe('课程中心')
    expect(w.find('.nav-item-icon').exists()).toBe(true)
  })
})
