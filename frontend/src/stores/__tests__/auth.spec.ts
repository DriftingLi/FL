// auth store 行为锁定测试：URL auth_token 交接 / localStorage 持久化 / 401 清理。
// 用真实 Pinia 实例（createPinia + setActivePinia），authApi 以 vi.mock 隔离。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises } from '@vue/test-utils'
import { authApi } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'

vi.mock('@/api/auth', () => ({
  authApi: { getUserInfo: vi.fn() }
}))

const getUserInfo = vi.mocked(authApi.getUserInfo)

function setUrl(search: string) {
  window.history.replaceState({}, '', `/app${search}`)
}

/** 创建 store 并显式启动初始化（对应 main.ts 的 initialize() 调用） */
function createStore() {
  const store = useAuthStore()
  store.initialize()
  return store
}

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  setUrl('')
  setActivePinia(createPinia())
})

describe('URL auth_token 消费', () => {
  it('携带有效 auth_token：建立登录态、持久化并立即从地址栏移除', async () => {
    setUrl('?auth_token=carried-9&next=/training')
    getUserInfo.mockResolvedValue({ role: 'hrwai_user', user_id: 3, username: 'u3' })

    const store = createStore()
    await flushPromises()

    expect(store.token).toBe('carried-9')
    expect(store.isLoggedIn).toBe(true)
    expect(store.userInfo.role).toBe('hrwai_user')
    expect(localStorage.getItem('token')).toBe('carried-9')
    expect(window.location.search).not.toContain('auth_token')
    expect(getUserInfo).toHaveBeenCalledTimes(1)
  })

  it('auth_token 优先级高于本地缓存', async () => {
    localStorage.setItem('token', 'old-token')
    localStorage.setItem('userInfo', JSON.stringify({ token: 'old-token', role: 'hrwai_user' }))
    setUrl('?auth_token=carried-new')
    getUserInfo.mockResolvedValue({ role: 'hrwai_user' })

    const store = createStore()
    await flushPromises()

    expect(store.token).toBe('carried-new')
    expect(localStorage.getItem('token')).toBe('carried-new')
  })

  it('无 auth_token：不建立登录态（no-op）', async () => {
    setUrl('?foo=1')
    getUserInfo.mockResolvedValue(null as never)

    const store = createStore()
    await flushPromises()

    expect(store.token).toBe('')
    expect(store.isLoggedIn).toBe(false)
    expect(window.location.search).toBe('?foo=1')
  })

  it('空 auth_token：视为缺失，no-op', async () => {
    setUrl('?auth_token=')
    getUserInfo.mockResolvedValue(null as never)

    const store = createStore()
    await flushPromises()

    expect(store.token).toBe('')
    expect(store.isLoggedIn).toBe(false)
  })
})

describe('token 持久化 round-trip', () => {
  it('setAuthData 写入 localStorage，clearAuthData 清空', async () => {
    getUserInfo.mockResolvedValue({ role: 'hrwai_user' })
    const store = useAuthStore()

    store.setAuthData({ token: 't-1', role: 'hrwai_user', user_id: 7 })

    expect(localStorage.getItem('token')).toBe('t-1')
    expect(JSON.parse(localStorage.getItem('userInfo') || '{}')).toMatchObject({ token: 't-1', role: 'hrwai_user', user_id: 7 })
    expect(store.isLoggedIn).toBe(true)

    store.clearAuthData()
    expect(localStorage.getItem('token')).toBeNull()
    expect(localStorage.getItem('userInfo')).toBeNull()
    expect(store.token).toBe('')
    expect(store.isLoggedIn).toBe(false)
    expect(store.userInfo).toEqual({})
    await flushPromises()
  })

  it('新 Pinia 实例从 localStorage 恢复登录态', async () => {
    localStorage.setItem('token', 't-restore')
    localStorage.setItem('userInfo', JSON.stringify({ token: 't-restore', role: 'hrwai_user', username: 'u9' }))
    getUserInfo.mockResolvedValue({ role: 'hrwai_user' })

    setActivePinia(createPinia())
    const store = createStore()
    await flushPromises()

    expect(store.token).toBe('t-restore')
    expect(store.isLoggedIn).toBe(true)
    expect(store.userInfo.username).toBe('u9')
  })

  it('storage 只有 token 没有 userInfo：不恢复并清空', async () => {
    localStorage.setItem('token', 'orphan')
    getUserInfo.mockResolvedValue(null as never)

    const store = createStore()
    await flushPromises()

    expect(store.token).toBe('')
    expect(store.isLoggedIn).toBe(false)
    expect(localStorage.getItem('token')).toBeNull()
  })

  it('/auth/me 返回资料合并进 userInfo 并持久化', async () => {
    localStorage.setItem('token', 't-merge')
    localStorage.setItem('userInfo', JSON.stringify({ token: 't-merge', role: 'hrwai_user', nickname: '老张' }))
    getUserInfo.mockResolvedValue({ role: 'hrwai_user', avatar_url: 'a.png' })

    const store = createStore()
    await flushPromises()

    expect(store.userInfo.nickname).toBe('老张')
    expect(store.userInfo.avatar_url).toBe('a.png')
    expect(JSON.parse(localStorage.getItem('userInfo') || '{}')).toMatchObject({ nickname: '老张', avatar_url: 'a.png' })
  })

  it('initialize 幂等：多次调用只执行一次校验', async () => {
    getUserInfo.mockResolvedValue({ role: 'hrwai_user' })

    const store = createStore()
    await store.initialize()
    await store.initialize()

    expect(getUserInfo).toHaveBeenCalledTimes(1)
  })
})

describe('401 / 校验失败触发清理', () => {
  it('validateToken 请求失败：清空登录态与 localStorage', async () => {
    localStorage.setItem('token', 'expired')
    localStorage.setItem('userInfo', JSON.stringify({ token: 'expired', role: 'hrwai_user' }))
    getUserInfo.mockRejectedValue(new Error('401'))

    const store = createStore()
    await flushPromises()

    expect(store.token).toBe('')
    expect(store.isLoggedIn).toBe(false)
    expect(localStorage.getItem('token')).toBeNull()
    expect(localStorage.getItem('userInfo')).toBeNull()
  })

  it('/auth/me 返回空：视为未登录并清空', async () => {
    localStorage.setItem('token', 'stale')
    localStorage.setItem('userInfo', JSON.stringify({ token: 'stale', role: 'hrwai_user' }))
    getUserInfo.mockResolvedValue(null as never)

    const store = createStore()
    await flushPromises()

    expect(store.isLoggedIn).toBe(false)
    expect(store.token).toBe('')
    expect(localStorage.getItem('token')).toBeNull()
  })
})
