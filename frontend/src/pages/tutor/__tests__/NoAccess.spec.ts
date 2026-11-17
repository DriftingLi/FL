// 「无讲师权限」页的两态（#2600 / 规格 F5）：
// 「取不到权限」与「没有权限」必须说实话 —— 前者给重试，后者给授权指引。
// 用真 store（只 mock 网络层）：状态机与页面的接线（capabilityState → 文案/动作）正是被测的行为。
//
// 与 admin 侧同名测试同形但**不复用**：两页的求助指引指向不同的操作人（讲师角色在「讲师管理」下
// 由管理员分配，管理角色在「角色权限」下由超管分配），合并就得按角色分支。
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const h = vi.hoisted(() => ({ push: vi.fn() }))

vi.mock('vue-router', () => ({ useRouter: () => ({ push: h.push }) }))
vi.mock('@/api/auth', () => ({
  authApi: { getUserInfo: vi.fn(), logout: vi.fn() }
}))
vi.mock('@/api/tutor', () => ({
  tutorApi: { fetchMyCapabilities: vi.fn() }
}))

import { epLite } from '@/test/element-lite'
import { authApi } from '@/api/auth'
import { tutorApi } from '@/api/tutor'
import { useAuthStore } from '@/stores/auth'
import NoAccess from '../NoAccess.vue'

const fetchTutorCapabilities = vi.mocked(tutorApi.fetchMyCapabilities)

function mountPage() {
  return mount(NoAccess, { global: { plugins: [epLite()] } })
}

/** 登录为讲师并把能力集装载推到 settle（失败用例传 rejected mock）。 */
async function signInAsTutor() {
  const store = useAuthStore()
  store.setAuthData({ token: 't-tutor', role: 'tutor', account: 'tutor1', user_id: 1 } as never)
  await flushPromises()
  return store
}

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  window.history.replaceState({}, '', '/training/tutor/no-access')
  setActivePinia(createPinia())
  vi.mocked(authApi.getUserInfo).mockResolvedValue({ account: 'tutor1', role: 'tutor', user_id: 1 } as never)
  vi.mocked(authApi.logout).mockResolvedValue(null as never)
})

describe('无讲师权限页', () => {
  it('装载成功且集合为空 → 「还没有讲师权限」+ 授权指引，没有重试按钮', async () => {
    fetchTutorCapabilities.mockResolvedValue({ capabilities: [], granted: false })
    await signInAsTutor()

    const w = mountPage()
    expect(w.find('.no-access-title').text()).toBe('当前账号还没有讲师权限')
    // 指引必须指向讲师侧的求助对象（讲师角色由管理员在「讲师管理」分配）
    expect(w.text()).toContain('讲师角色')
    expect(w.text()).toContain('讲师管理')
    expect(w.text()).not.toContain('重试')
    // 不复用管理端话术：不该出现「角色权限」「超级管理员」这套管理端指引
    expect(w.text()).not.toContain('超级管理员')
  })

  it('装载失败 → 说实话（「加载失败」而不是「没有权限」）+ 重试入口', async () => {
    fetchTutorCapabilities.mockRejectedValue(new Error('boom'))
    await signInAsTutor()

    const w = mountPage()
    expect(w.find('.no-access-title').text()).toBe('讲师权限加载失败')
    expect(w.text()).toContain('不代表账号没有权限')
    const buttons = w.findAll('button')
    expect(buttons.some(b => b.text().includes('重试'))).toBe(true)
    expect(buttons.some(b => b.text().includes('退出登录'))).toBe(true)
  })

  it('失败后点重试：重新装载并直接进落点（不必用户自己找路）', async () => {
    fetchTutorCapabilities.mockRejectedValueOnce(new Error('boom'))
    await signInAsTutor()
    const w = mountPage()

    fetchTutorCapabilities.mockResolvedValueOnce({ capabilities: ['tutor.access'], granted: true })
    await w.findAll('button').find(b => b.text().includes('重试'))!.trigger('click')
    await flushPromises()

    expect(fetchTutorCapabilities).toHaveBeenCalledTimes(2)
    expect(h.push).toHaveBeenCalledWith({ name: 'TutorDashboard' })
  })

  it('重试后仍然空集 → 就地切回「没有权限」的说明（不跳转、不留在失败态）', async () => {
    fetchTutorCapabilities.mockRejectedValueOnce(new Error('boom'))
    await signInAsTutor()
    const w = mountPage()

    fetchTutorCapabilities.mockResolvedValueOnce({ capabilities: [], granted: false })
    await w.findAll('button').find(b => b.text().includes('重试'))!.trigger('click')
    await flushPromises()

    expect(w.find('.no-access-title').text()).toBe('当前账号还没有讲师权限')
    expect(h.push).not.toHaveBeenCalled()
  })

  it('退出登录走登出单点并回登录页', async () => {
    fetchTutorCapabilities.mockResolvedValue({ capabilities: [], granted: false })
    await signInAsTutor()
    const w = mountPage()

    await w.findAll('button').find(b => b.text().includes('退出登录'))!.trigger('click')
    await flushPromises()

    expect(authApi.logout).toHaveBeenCalled()
    expect(h.push).toHaveBeenCalledWith({ name: 'Login' })
  })
})
