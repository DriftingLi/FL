// 本地存储单点（storage.ts）测试：token / userInfo key 唯一入口（决策 D8）。
// 断言 key 常量、读写 round-trip 与 clearLocalAuth 收敛行为。
import { describe, it, expect, beforeEach } from 'vitest'
import {
  TOKEN_KEY,
  USER_INFO_KEY,
  getToken,
  setToken,
  removeToken,
  getUserInfo,
  setUserInfo,
  removeUserInfo,
  clearLocalAuth
} from '../storage'

beforeEach(() => {
  localStorage.clear()
})

describe('storage token / userInfo 单点', () => {
  it('key 常量与历史字面量一致（前端契约不变）', () => {
    expect(TOKEN_KEY).toBe('token')
    expect(USER_INFO_KEY).toBe('userInfo')
  })

  it('token 写入/读取/删除 round-trip', () => {
    expect(getToken()).toBeNull()
    setToken('jwt-abc')
    expect(localStorage.getItem(TOKEN_KEY)).toBe('jwt-abc')
    expect(getToken()).toBe('jwt-abc')
    removeToken()
    expect(getToken()).toBeNull()
  })

  it('userInfo 序列化 round-trip', () => {
    setUserInfo({ id: 1, nickname: '小明' })
    expect(getUserInfo()).toEqual({ id: 1, nickname: '小明' })
    removeUserInfo()
    expect(getUserInfo()).toBeNull()
  })

  it('损坏的 userInfo JSON 返回 null 而不抛错', () => {
    localStorage.setItem(USER_INFO_KEY, '{broken')
    expect(getUserInfo()).toBeNull()
  })

  it('clearLocalAuth 一次性清除 token 与 userInfo（401 兜底与登出共用）', () => {
    setToken('jwt-abc')
    setUserInfo({ id: 1 })
    clearLocalAuth()
    expect(localStorage.getItem(TOKEN_KEY)).toBeNull()
    expect(localStorage.getItem(USER_INFO_KEY)).toBeNull()
  })
})
