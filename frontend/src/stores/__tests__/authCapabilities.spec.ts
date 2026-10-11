// 管理端运行时能力集的拉取语义（#1618 段1；装载状态机 #1645）：
// 非动态角色不发请求；动态角色登录/恢复登录时拉取；**失败落空集且不保留旧值**（fail closed）；
// 「装载中」与「没有能力」必须可分辨 —— 前者不得被当成后者（线上：全权限超管被判无权限）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises } from '@vue/test-utils'
import { authApi } from '@/api/auth'
import { adminApi } from '@/api/admin'
import { useAuthStore } from '@/stores/auth'

vi.mock('@/api/auth', () => ({
  authApi: { getUserInfo: vi.fn(), logout: vi.fn() }
}))
vi.mock('@/api/admin', () => ({
  adminApi: { fetchMyCapabilities: vi.fn() }
}))

const getUserInfo = vi.mocked(authApi.getUserInfo)
const fetchMyCapabilities = vi.mocked(adminApi.fetchMyCapabilities)

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  window.history.replaceState({}, '', '/app')
  setActivePinia(createPinia())
})

describe('管理端能力集拉取', () => {
  it('登录为管理角色：拉取能力集并落 store', async () => {
    getUserInfo.mockResolvedValue({ account: 'admin1', role: 'admin', user_id: 1, username: 'admin1' })
    fetchMyCapabilities.mockResolvedValue({ capabilities: ['admin.access', 'audit.read'], granted: true })

    const store = useAuthStore()
    store.setAuthData({ token: 't-admin', role: 'admin', account: 'admin1', user_id: 1 } as never)
    await flushPromises()

    expect(fetchMyCapabilities).toHaveBeenCalledTimes(1)
    expect(store.capabilities).toEqual(['admin.access', 'audit.read'])
  })

  it('非管理角色：不发请求，能力集保持为空（可达面由静态表回答）', async () => {
    getUserInfo.mockResolvedValue({ account: 'u1', role: 'hrwai_user', user_id: 1, username: 'u1' })

    const store = useAuthStore()
    store.setAuthData({ token: 't-stu', role: 'hrwai_user', account: 'u1', user_id: 1 } as never)
    await flushPromises()

    expect(fetchMyCapabilities).not.toHaveBeenCalled()
    expect(store.capabilities).toEqual([])
    // 静态角色永远不需要这份集合：状态停在 idle（守卫的装载前置因此对它们无感）
    expect(store.capabilityState).toBe('idle')
    expect(store.capabilitiesSettled).toBe(false)
  })

  it('拉取失败 → 空集且**不保留旧值**（刚被收回权限的人不得继续看到菜单）', async () => {
    getUserInfo.mockResolvedValue({ account: 'admin1', role: 'admin', user_id: 1, username: 'admin1' })
    fetchMyCapabilities.mockResolvedValueOnce({ capabilities: ['admin.access', 'audit.read'], granted: true })
    fetchMyCapabilities.mockRejectedValueOnce(new Error('boom'))

    const store = useAuthStore()
    store.setAuthData({ token: 't-admin', role: 'admin', account: 'admin1', user_id: 1 } as never)
    await flushPromises()
    expect(store.capabilities).toEqual(['admin.access', 'audit.read'])

    await store.loadCapabilities()
    expect(store.capabilities).toEqual([])
  })

  it('清除登录态同时清空能力集', async () => {
    getUserInfo.mockResolvedValue({ account: 'admin1', role: 'admin', user_id: 1, username: 'admin1' })
    fetchMyCapabilities.mockResolvedValue({ capabilities: ['admin.access'], granted: true })

    const store = useAuthStore()
    store.setAuthData({ token: 't-admin', role: 'admin', account: 'admin1', user_id: 1 } as never)
    await flushPromises()
    expect(store.capabilities.length).toBe(1)

    store.clearAuthData()
    expect(store.capabilities).toEqual([])
  })
})

describe('能力集装载状态机（#1645）', () => {
  it('成功 → ready，失败 → failed：两条路都落定，不停在 loading', async () => {
    getUserInfo.mockResolvedValue({ account: 'admin1', role: 'admin', user_id: 1, username: 'admin1' })
    fetchMyCapabilities.mockResolvedValue({ capabilities: ['admin.access'], granted: true })

    const store = useAuthStore()
    store.setAuthData({ token: 't-admin', role: 'admin', account: 'admin1', user_id: 1 } as never)
    // 「装载中」是真实存在的一态，且**不等于**「没有能力」
    expect(store.capabilityState).toBe('loading')
    expect(store.capabilitiesSettled).toBe(false)
    await flushPromises()
    expect(store.capabilityState).toBe('ready')
    expect(store.capabilitiesSettled).toBe(true)
  })

  it('装载失败也落定（failed）：集合按空处理，但状态与「真的没有能力」可分辨', async () => {
    getUserInfo.mockResolvedValue({ account: 'admin1', role: 'admin', user_id: 1, username: 'admin1' })
    fetchMyCapabilities.mockRejectedValue(new Error('boom'))

    const store = useAuthStore()
    store.setAuthData({ token: 't-admin', role: 'admin', account: 'admin1', user_id: 1 } as never)
    await flushPromises()

    expect(store.capabilityState).toBe('failed')
    expect(store.capabilitiesSettled).toBe(true)
    expect(store.capabilities).toEqual([])
  })

  it('并发调用复用同一次在途请求（守卫补拉与登录预热不会发两次）', async () => {
    getUserInfo.mockResolvedValue({ account: 'admin1', role: 'admin', user_id: 1, username: 'admin1' })
    fetchMyCapabilities.mockResolvedValue({ capabilities: ['audit.read'], granted: true })

    const store = useAuthStore()
    store.setAuthData({ token: 't-admin', role: 'admin', account: 'admin1', user_id: 1 } as never)
    // 守卫在登录跳转时补拉：此刻登录预热的那一次还在途
    const [a, b] = await Promise.all([store.loadCapabilities(), store.loadCapabilities()])
    expect(a).toBeUndefined()
    expect(b).toBeUndefined()
    expect(fetchMyCapabilities).toHaveBeenCalledTimes(1)
    expect(store.capabilityState).toBe('ready')
  })

  it('登出作废在途装载：旧账号的能力集不落到新的登录态上', async () => {
    getUserInfo.mockResolvedValue({ account: 'admin1', role: 'admin', user_id: 1, username: 'admin1' })
    let resolveCaps: (v: unknown) => void = () => {}
    fetchMyCapabilities.mockImplementationOnce(
      () => new Promise(resolve => { resolveCaps = resolve }) as never
    )

    const store = useAuthStore()
    store.setAuthData({ token: 't-admin', role: 'admin', account: 'admin1', user_id: 1 } as never)
    expect(store.capabilityState).toBe('loading')

    store.clearAuthData()
    resolveCaps({ capabilities: ['admin.access'], granted: true })
    await flushPromises()

    expect(store.capabilities).toEqual([])
    expect(store.capabilityState).toBe('idle')
  })

  it('重新装载会再发一次请求（重试入口依赖它）', async () => {
    getUserInfo.mockResolvedValue({ account: 'admin1', role: 'admin', user_id: 1, username: 'admin1' })
    fetchMyCapabilities.mockRejectedValueOnce(new Error('boom'))
    fetchMyCapabilities.mockResolvedValueOnce({ capabilities: ['audit.read'], granted: true })

    const store = useAuthStore()
    store.setAuthData({ token: 't-admin', role: 'admin', account: 'admin1', user_id: 1 } as never)
    await flushPromises()
    expect(store.capabilityState).toBe('failed')

    await store.loadCapabilities()
    expect(fetchMyCapabilities).toHaveBeenCalledTimes(2)
    expect(store.capabilityState).toBe('ready')
    expect(store.capabilities).toEqual(['audit.read'])
  })
})
