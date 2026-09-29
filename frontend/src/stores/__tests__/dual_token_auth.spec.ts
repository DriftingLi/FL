// stores/auth.ts 双令牌生命周期补充单测（ADR-0012 起，ADR-0067 修订 refresh 的存放面）：
// setAuthData 只持久化 access；refresh 不落 JS 可达存储（含 userInfo 这条侧漏路径）；
// clearAuthData 双清 access 与存量残留的 refresh。
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const { getUserInfoMock } = vi.hoisted(() => ({ getUserInfoMock: vi.fn() }))

vi.mock('@/api/auth', () => ({
  authApi: { getUserInfo: getUserInfoMock }
}))

import { useAuthStore } from '../auth'
import { REFRESH_TOKEN_KEY, TOKEN_KEY, USER_INFO_KEY } from '@/utils/storage'

beforeEach(() => {
  setActivePinia(createPinia())
  localStorage.clear()
  getUserInfoMock.mockReset()
})

describe('auth store 双令牌生命周期（ADR-0012）', () => {
  it('setAuthData 持久化 access，但不持久化 refresh（ADR-0067）', async () => {
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
    // ADR-0067：浏览器侧的 refresh 只在 httpOnly Cookie 里；登录响应那一份是给
    // 移动端/非浏览器客户端的，Web 端一处都不落。
    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBeNull()
    // 侧漏路径：userInfo 会被整体序列化进 localStorage，入库前必须剥掉 refresh_token
    expect(localStorage.getItem(USER_INFO_KEY)).not.toContain('refresh-token')
    expect(localStorage.getItem(USER_INFO_KEY)).toContain('"account":"u1"')
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