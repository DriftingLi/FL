// stores/auth.ts 双令牌生命周期补充单测（ADR-0012）：
// setAuthData 持久化 refresh；clearAuthData 双清 access 与 refresh。
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const { getUserInfoMock } = vi.hoisted(() => ({ getUserInfoMock: vi.fn() }))

vi.mock('@/api/auth', () => ({
  authApi: { getUserInfo: getUserInfoMock }
}))

import { useAuthStore } from '../auth'
import { REFRESH_TOKEN_KEY, TOKEN_KEY } from '@/utils/storage'

beforeEach(() => {
  setActivePinia(createPinia())
  localStorage.clear()
  getUserInfoMock.mockReset()
})

describe('auth store 双令牌生命周期（ADR-0012）', () => {
  it('setAuthData 持久化 access 与 refresh token', async () => {
    getUserInfoMock.mockResolvedValue({ role: 'hrwai_user' })
    const store = useAuthStore()

    store.setAuthData({
      token: 'access-token',
      refresh_token: 'refresh-token',
      role: 'hrwai_user',
      account: 'u1'
    })
    await Promise.resolve()

    expect(localStorage.getItem(TOKEN_KEY)).toBe('access-token')
    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBe('refresh-token')
    expect(store.isLoggedIn).toBe(true)
  })

  it('clearAuthData 双清 access 与 refresh', async () => {
    localStorage.setItem(TOKEN_KEY, 'access-token')
    localStorage.setItem(REFRESH_TOKEN_KEY, 'refresh-token')
    const store = useAuthStore()

    store.clearAuthData()

    expect(localStorage.getItem(TOKEN_KEY)).toBeNull()
    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBeNull()
    expect(store.isLoggedIn).toBe(false)
  })
})