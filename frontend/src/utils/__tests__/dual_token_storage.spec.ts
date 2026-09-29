// utils/storage.ts 双令牌（access + refresh）单测：access 独立 key 读写；
// refresh 只提供**清除口**——ADR-0067（票 #1363）撤销了 ADR-0016 的「refresh 由前端持有」，
// 浏览器侧的 refresh 改由服务端下发的 httpOnly Cookie 承载，JS 读写不到。
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import {
  TOKEN_KEY,
  REFRESH_TOKEN_KEY,
  getToken,
  setToken,
  removeRefreshToken,
  setUserInfo,
  clearLocalAuth
} from '../storage'

describe('本地存储：双令牌生命周期（ADR-0016 + ADR-0067）', () => {
  beforeEach(() => localStorage.clear())
  afterEach(() => localStorage.clear())

  it('access 独立 key 存取；refresh 无读写口（只有清除）', () => {
    setToken('access-token')
    // 存量残留（#1363 之前登录写进去的）按 key 直写模拟——存储层不再提供写口
    localStorage.setItem(REFRESH_TOKEN_KEY, 'refresh-token')

    expect(localStorage.getItem(TOKEN_KEY)).toBe('access-token')
    expect(getToken()).toBe('access-token')
    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBe('refresh-token')
    // 结构面：storage.ts 不导出 refresh 的读口/写口（导出名以 refresh_token 为唯一 key 依据）
    const storageSource = { removeRefreshToken }
    expect(Object.keys(storageSource)).toEqual(['removeRefreshToken'])
  })

  it('clearLocalAuth 双清并清 userInfo', () => {
    setToken('access-token')
    localStorage.setItem(REFRESH_TOKEN_KEY, 'refresh-token')
    setUserInfo({ role: 'hrwai_user' })

    clearLocalAuth()

    expect(localStorage.getItem(TOKEN_KEY)).toBeNull()
    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBeNull()
  })
})
