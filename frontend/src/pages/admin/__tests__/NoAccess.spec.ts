// 「无管理权限」页的两态（#1638 立页；#1645 分出「装载失败」）：
// 「取不到权限」与「没有权限」必须说实话 —— 前者给重试，后者给授权指引。
// 用真 store（只 mock 网络层）：状态机与页面的接线（capabilityState → 文案/动作）正是被测的行为。
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const h = vi.hoisted(() => ({ push: vi.fn() }))

vi.mock('vue-router', () => ({ useRouter: () => ({ push: h.push }) }))
vi.mock('@/api/auth', () => ({
  authApi: { getUserInfo: vi.fn(), logout: vi.fn() }
}))
vi.mock('@/api/admin', () => ({
  adminApi: { fetchMyCapabilities: vi.fn() }
}))

import { epLite } from '@/test/element-lite'
import { authApi } from '@/api/auth'
import { adminApi } from '@/api/admin'
import { useAuthStore } from '@/stores/auth'
import NoAccess from '../NoAccess.vue'

const fetchMyCapabilities = vi.mocked(adminApi.fetchMyCapabilities)

function mountPage() {
  return mount(NoAccess, { global: { plugins: [epLite()] } })
}

/** 登录为管理员并把能力集装载推到 settle（失败用例传 rejected mock）。 */
async function signInAsAdmin() {
  const store = useAuthStore()
  store.setAuthData({ token: 't-admin', role: 'admin', account: 'admin1', user_id: 1 } as never)
  await flushPromises()
  return store
}

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  window.history.replaceState({}, '', '/admin/no-access')
  setActivePinia(createPinia())
  vi.mocked(authApi.getUserInfo).mockResolvedValue({ account: 'admin1', role: 'admin', user_id: 1 } as never)
  vi.mocked(authApi.logout).mockResolvedValue(null as never)
})

describe('无管理权限页', () => {
  it('装载成功且集合为空 → 「一个能力位都没有」+ 授权指引，没有重试按钮', async () => {
    fetchMyCapabilities.mockResolvedValue({ capabilities: [], granted: true })
    await signInAsAdmin()

    const w = mountPage()
    expect(w.find('.no-access-title').text()).toBe('当前账号还没有管理权限')
    expect(w.text()).toContain('角色权限')
    expect(w.text()).not.toContain('重试')
  })

  it('装载失败 → 说实话（「加载失败」而不是「没有权限」）+ 重试入口', async () => {
    fetchMyCapabilities.mockRejectedValue(new Error('boom'))
    await signInAsAdmin()

    const w = mountPage()
    expect(w.find('.no-access-title').text()).toBe('管理权限加载失败')
    expect(w.text()).toContain('不代表账号没有权限')
    const buttons = w.findAll('button')
    expect(buttons.some(b => b.text().includes('重试'))).toBe(true)
    expect(buttons.some(b => b.text().includes('退出登录'))).toBe(true)
  })

  it('失败后点重试：重新装载并直接进落点（不必用户自己找路）', async () => {
    fetchMyCapabilities.mockRejectedValueOnce(new Error('boom'))
    await signInAsAdmin()
    const w = mountPage()

    fetchMyCapabilities.mockResolvedValueOnce({ capabilities: ['admin.access'], granted: true })
    await w.findAll('button').find(b => b.text().includes('重试'))!.trigger('click')
    await flushPromises()

    expect(fetchMyCapabilities).toHaveBeenCalledTimes(2)
    expect(h.push).toHaveBeenCalledWith({ name: 'AdminDashboard' })
  })

  it('重试后仍然空集 → 就地切回「没有能力」的说明（不跳转、不留在失败态）', async () => {
    fetchMyCapabilities.mockRejectedValueOnce(new Error('boom'))
    await signInAsAdmin()
    const w = mountPage()

    fetchMyCapabilities.mockResolvedValueOnce({ capabilities: [], granted: true })
    await w.findAll('button').find(b => b.text().includes('重试'))!.trigger('click')
    await flushPromises()

    expect(w.find('.no-access-title').text()).toBe('当前账号还没有管理权限')
    expect(h.push).not.toHaveBeenCalled()
  })

  it('退出登录走登出单点并回登录页', async () => {
    fetchMyCapabilities.mockResolvedValue({ capabilities: [], granted: true })
    await signInAsAdmin()
    const w = mountPage()

    await w.findAll('button').find(b => b.text().includes('退出登录'))!.trigger('click')
    await flushPromises()

    expect(authApi.logout).toHaveBeenCalled()
    expect(h.push).toHaveBeenCalledWith({ name: 'Login' })
  })
})
