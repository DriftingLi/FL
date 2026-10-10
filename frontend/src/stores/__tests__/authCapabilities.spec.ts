// 管理端运行时能力集的拉取语义（#1618 段1）：
// 非动态角色不发请求；动态角色登录/恢复登录时拉取；**失败落空集且不保留旧值**（fail closed）。
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
