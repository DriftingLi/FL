// utils/storage.ts 双令牌（access + refresh）单测：独立 key、get/set/remove、clearLocalAuth 双清。
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import {
  TOKEN_KEY,
  REFRESH_TOKEN_KEY,
  getToken,
  setToken,
  setRefreshToken,
  setUserInfo,
  clearLocalAuth
} from '../storage'

describe('本地存储：双令牌生命周期（ADR-0012）', () => {
  beforeEach(() => localStorage.clear())
  afterEach(() => localStorage.clear())

  it('access 与 refresh 各自独立 key 存取', () => {
    setToken('access-token')
    setRefreshToken('refresh-token')

    expect(localStorage.getItem(TOKEN_KEY)).toBe('access-token')
    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBe('refresh-token')
    expect(getToken()).toBe('access-token')
  })

  it('clearLocalAuth 双清并清 userInfo', () => {
    setToken('access-token')
    setRefreshToken('refresh-token')
    setUserInfo({ role: 'hrwai_user' })

    clearLocalAuth()

    expect(localStorage.getItem(TOKEN_KEY)).toBeNull()
    expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBeNull()
  })
})
