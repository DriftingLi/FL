// TrainingLayout 布局级搜索入口 + ⌘/Ctrl+K（#984）。
//
// 测试在**布局自己的 seam** 上：侧栏与凭据切换器是外部组件（stub 掉），
// 断言的是「入口存在且可达」与「快捷键触发同一动作」两件行为。
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

const h = vi.hoisted(() => ({
  route: { path: '/training/courses', params: {} as Record<string, unknown>, name: 'CourseList' },
  push: vi.fn()
}))

vi.mock('vue-router', () => ({
  useRoute: () => h.route,
  useRouter: () => ({ push: h.push })
}))
vi.mock('@/stores/course', () => ({
  useCourseStore: () => ({ loadCourse: vi.fn(), chapters: [] })
}))
vi.mock('@/config/navigation', () => ({ roleNavigation: { student: [] } }))

import TrainingLayout from '../TrainingLayout.vue'

function mountLayout() {
  return mount(TrainingLayout, {
    global: {
      stubs: {
        SidebarLayout: { template: '<div class="sidebar-stub"><slot name="top" :collapsed="false" /></div>' },
        CredentialSwitcher: true
      }
    }
  })
}

let wrapper: ReturnType<typeof mountLayout> | null = null

beforeEach(() => {
  h.route.path = '/training/courses'
  h.push.mockClear()
})

// 每个用例都挂载真实组件（含 window 监听）：用例间必须卸载，否则上一个用例的监听会漏到下一个
// （「卸载后不再响应快捷键」这条会因此假失败）。
afterEach(() => {
  wrapper?.unmount()
  wrapper = null
})

describe('TrainingLayout 搜索入口', () => {
  it('侧栏顶部渲染搜索入口，点击跳搜索页并带 focus=1', async () => {
    const w = (wrapper = mountLayout())
    const btn = w.findAll('button').find((b) => b.text().includes('搜索'))
    expect(btn).toBeTruthy()
    await btn!.trigger('click')
    expect(h.push).toHaveBeenCalledWith({ path: '/training/search', query: { focus: '1' } })
  })

  it('⌘/Ctrl+K 走同一动作', async () => {
    wrapper = mountLayout()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true }))
    expect(h.push).toHaveBeenCalledWith({ path: '/training/search', query: { focus: '1' } })
  })

  it('已在搜索页时只发聚焦事件，不重复导航', async () => {
    h.route.path = '/training/search'
    wrapper = mountLayout()
    const spy = vi.fn()
    window.addEventListener('focus-global-search', spy)
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', metaKey: true }))
    expect(spy).toHaveBeenCalled()
    expect(h.push).not.toHaveBeenCalled()
    window.removeEventListener('focus-global-search', spy)
  })

  it('卸载后摘掉快捷键监听（布局切换不留残留）', async () => {
    const w = (wrapper = mountLayout())
    w.unmount()
    wrapper = null
    h.push.mockClear()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true }))
    expect(h.push).not.toHaveBeenCalled()
  })
})
