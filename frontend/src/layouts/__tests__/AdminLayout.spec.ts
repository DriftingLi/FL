import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { adminApi } from '@/api/admin'
import { pages } from '@/config/pages'
import Dashboard from '@/pages/admin/Dashboard.vue'
import { useAuthStore } from '@/stores/auth'
import { epLite } from '@/test/element-lite'
import AdminLayout from '../AdminLayout.vue'

const statistics = {
  overview: { total_students: 12, active_today: 3, total_courses: 5, total_study_duration: 60 },
  course_stats: []
}

const managePages = pages.filter(page => page.workspace === 'manage' && page.layout === 'manage')

async function mountWorkspace(capabilities: string[], path = '/admin/dashboard') {
  const pinia = createPinia()
  const auth = useAuthStore(pinia)
  auth.userInfo = { role: 'admin' }
  auth.isLoggedIn = true
  auth.capabilities = capabilities
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{
      path: '/',
      component: AdminLayout,
      children: managePages.map(page => ({
        name: page.name,
        path: page.path,
        meta: { workspace: 'manage' },
        component: page.name === 'AdminDashboard' ? Dashboard : { template: '<div />' }
      }))
    }]
  })
  await router.push(path)
  const wrapper = mount({ template: '<RouterView />' }, {
    global: {
      plugins: [pinia, router, epLite()],
      stubs: { AppTopBar: { template: '<header><slot name="center" /></header>' }, SiteFooter: true }
    }
  })
  return { auth, router, wrapper }
}

afterEach(() => vi.restoreAllMocks())

describe('管理端工作台与导航', () => {
  it('审核账号只看到获准入口，返回工作台时收起分组，隐藏期间照常开签', async () => {
    const load = vi.spyOn(adminApi, 'getStatistics')
    const { auth, router, wrapper } = await mountWorkspace(['admin.access', 'question.review'])
    expect(wrapper.get('main h1').text()).toBe('工作台')
    expect(wrapper.findAll('main a').map(link => link.text())).toEqual(['题库审核'])
    expect(wrapper.findAll('.nav-group-label').map(group => group.text())).toEqual(['审核处理'])
    expect(wrapper.get('.nav-group-label').attributes('aria-expanded')).toBe('false')
    const header = wrapper.get<HTMLElement>('.content-header').element
    expect(header.style.display).toBe('none')
    expect(load).not.toHaveBeenCalled()

    await wrapper.get('main a').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.name).toBe('QuestionReview')
    expect(wrapper.get('.nav-group-label').attributes('aria-expanded')).toBe('true')
    expect(wrapper.get('.nav-group-label').classes()).toContain('is-active')

    router.back()
    await flushPromises()
    expect(wrapper.get('.nav-group-label').attributes('aria-expanded')).toBe('false')

    const toggle = wrapper.get('button[aria-controls="admin-page-tabs"]')
    await toggle.trigger('click')
    expect(toggle.attributes('aria-expanded')).toBe('true')
    expect(header.style.display).not.toBe('none')
    expect(wrapper.findAll('[role="tab"]').map(tab => tab.text())).toEqual(['工作台', '题库审核'])
    auth.isLoggedIn = false
    await flushPromises()
    expect(header.style.display).toBe('none')
    wrapper.unmount()
  })

  it('直接访问详情页时展开所属分组并归入列表页签', async () => {
    const { wrapper } = await mountWorkspace(['featured.manage'], '/admin/featured-content/edit/12')
    expect(wrapper.get('.nav-group-label.is-active').text()).toBe('内容运营')
    expect(wrapper.get('.nav-group-label').attributes('aria-expanded')).toBe('true')
    expect(wrapper.get('[role="tab"]').text()).toBe('内容精选')
    wrapper.unmount()
  })

  it('统计请求失败时仍可使用任务入口，并能在摘要区重试', async () => {
    const load = vi.spyOn(adminApi, 'getStatistics')
      .mockRejectedValueOnce(new Error('统计加载失败'))
      .mockResolvedValueOnce(statistics)
    const capabilities = managePages.flatMap(page => page.capability ? [page.capability] : [])
    const { wrapper } = await mountWorkspace(capabilities)
    await flushPromises()
    expect(wrapper.findAll('main li a')).toHaveLength(8)
    expect(wrapper.get('main section[aria-label="统计摘要"]').text()).toContain('加载失败')
    await wrapper.get('main section[aria-label="统计摘要"] button').trigger('click')
    await flushPromises()
    expect(load).toHaveBeenCalledTimes(2)
    expect(wrapper.get('main section[aria-label="统计摘要"]').text()).toContain('今日活跃学员')
    wrapper.unmount()
  })
})
