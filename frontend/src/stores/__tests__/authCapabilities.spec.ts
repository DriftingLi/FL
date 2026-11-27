// 管理端运行时能力集的拉取语义（#1618 段1；装载状态机 #1645）：
// 非动态角色不发请求；动态角色登录/恢复登录时拉取；**失败落空集且不保留旧值**（fail closed）；
// 「装载中」与「没有能力」必须可分辨 —— 前者不得被当成后者（线上：全权限超管被判无权限）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises } from '@vue/test-utils'
import { authApi } from '@/api/auth'
import { adminApi } from '@/api/admin'
import { tutorApi } from '@/api/tutor'
import { useAuthStore } from '@/stores/auth'
import { createDynamicCapabilityLoad } from '@/stores/dynamicCapabilityLoad'
import * as authzRuntime from '@/utils/authzRuntime'
import { resolveGuardDecision, type GuardInput, type GuardState } from '@/router/guard'

vi.mock('@/api/auth', () => ({
  authApi: { getUserInfo: vi.fn(), logout: vi.fn() }
}))
vi.mock('@/api/admin', () => ({
  adminApi: { fetchMyCapabilities: vi.fn() }
}))
vi.mock('@/api/tutor', () => ({
  tutorApi: { fetchMyCapabilities: vi.fn() }
}))

const getUserInfo = vi.mocked(authApi.getUserInfo)
const fetchMyCapabilities = vi.mocked(adminApi.fetchMyCapabilities)
const fetchTutorCapabilities = vi.mocked(tutorApi.fetchMyCapabilities)

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  window.history.replaceState({}, '', '/app')
  setActivePinia(createPinia())
})

describe('管理端能力集拉取', () => {
  it('登录为管理角色：拉取能力集并落 store', async () => {
    getUserInfo.mockResolvedValue({ pending_fields: [], account: 'admin1', role: 'admin', user_id: 1, username: 'admin1' })
    fetchMyCapabilities.mockResolvedValue({ capabilities: ['admin.access', 'audit.read'], granted: true })

    const store = useAuthStore()
    store.setAuthData({ token: 't-admin', role: 'admin', account: 'admin1', user_id: 1 } as never)
    await flushPromises()

    expect(fetchMyCapabilities).toHaveBeenCalledTimes(1)
    expect(store.capabilities).toEqual(['admin.access', 'audit.read'])
  })

  it('非管理角色：不发请求，能力集保持为空（可达面由静态表回答）', async () => {
    getUserInfo.mockResolvedValue({ pending_fields: [], account: 'u1', role: 'hrwai_user', user_id: 1, username: 'u1' })

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
    getUserInfo.mockResolvedValue({ pending_fields: [], account: 'admin1', role: 'admin', user_id: 1, username: 'admin1' })
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
    getUserInfo.mockResolvedValue({ pending_fields: [], account: 'admin1', role: 'admin', user_id: 1, username: 'admin1' })
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
    getUserInfo.mockResolvedValue({ pending_fields: [], account: 'admin1', role: 'admin', user_id: 1, username: 'admin1' })
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
    getUserInfo.mockResolvedValue({ pending_fields: [], account: 'admin1', role: 'admin', user_id: 1, username: 'admin1' })
    fetchMyCapabilities.mockRejectedValue(new Error('boom'))

    const store = useAuthStore()
    store.setAuthData({ token: 't-admin', role: 'admin', account: 'admin1', user_id: 1 } as never)
    await flushPromises()

    expect(store.capabilityState).toBe('failed')
    expect(store.capabilitiesSettled).toBe(true)
    expect(store.capabilities).toEqual([])
  })

  it('并发调用复用同一次在途请求（守卫补拉与登录预热不会发两次）', async () => {
    getUserInfo.mockResolvedValue({ pending_fields: [], account: 'admin1', role: 'admin', user_id: 1, username: 'admin1' })
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
    getUserInfo.mockResolvedValue({ pending_fields: [], account: 'admin1', role: 'admin', user_id: 1, username: 'admin1' })
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
    getUserInfo.mockResolvedValue({ pending_fields: [], account: 'admin1', role: 'admin', user_id: 1, username: 'admin1' })
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

describe('讲师端能力集拉取（#2600 / 规格 F2）', () => {
  it('登录为讲师：走讲师端点，不走管理端端点', async () => {
    getUserInfo.mockResolvedValue({ pending_fields: [], account: 'tutor1', role: 'tutor', user_id: 9, username: 'tutor1' })
    fetchTutorCapabilities.mockResolvedValue({ capabilities: ['tutor.access', 'question.author'], granted: true })

    const store = useAuthStore()
    store.setAuthData({ token: 't-tutor', role: 'tutor', account: 'tutor1', user_id: 9 } as never)
    await flushPromises()

    expect(fetchTutorCapabilities).toHaveBeenCalledTimes(1)
    // 两个域的可见面不同，端点不得混用（混用会把管理端能力集当成讲师能力集）
    expect(fetchMyCapabilities).not.toHaveBeenCalled()
    expect(store.capabilities).toEqual(['tutor.access', 'question.author'])
    expect(store.capabilityState).toBe('ready')
  })

  it('未挂角色的讲师：落空集但仍是 ready（「真的没有权限」，不是装载失败）', async () => {
    getUserInfo.mockResolvedValue({ pending_fields: [], account: 'tutor2', role: 'tutor', user_id: 10, username: 'tutor2' })
    fetchTutorCapabilities.mockResolvedValue({ capabilities: [], granted: false })

    const store = useAuthStore()
    store.setAuthData({ token: 't-tutor2', role: 'tutor', account: 'tutor2', user_id: 10 } as never)
    await flushPromises()

    expect(store.capabilities).toEqual([])
    expect(store.capabilityState).toBe('ready')
  })

  it('讲师能力集拉取失败 → 空集 + failed（不保留旧份，前端据此画「加载失败」而非「没有权限」）', async () => {
    getUserInfo.mockResolvedValue({ pending_fields: [], account: 'tutor3', role: 'tutor', user_id: 11, username: 'tutor3' })
    fetchTutorCapabilities.mockRejectedValue(new Error('boom'))

    const store = useAuthStore()
    store.setAuthData({ token: 't-tutor3', role: 'tutor', account: 'tutor3', user_id: 11 } as never)
    await flushPromises()

    expect(store.capabilities).toEqual([])
    expect(store.capabilityState).toBe('failed')
    expect(store.capabilitiesSettled).toBe(true)
  })
})

describe('动态角色判定的单一入口', () => {
  it('能力装载器按 authzRuntime 的判定选择 adapter，不维护第二份角色判定', async () => {
    const isDynamicRole = vi.spyOn(authzRuntime, 'isDynamicRole').mockReturnValue(true)
    try {
      const admin = vi.fn().mockResolvedValue({ capabilities: ['admin.access'] })
      const tutor = vi.fn().mockResolvedValue({ capabilities: ['tutor.access'] })
      const loader = createDynamicCapabilityLoad({ admin, tutor })

      loader.setIdentity({ key: 'custom-role', role: 'custom_dynamic_role' })
      await loader.load()

      expect(isDynamicRole).toHaveBeenCalledWith('custom_dynamic_role')
      expect(admin).toHaveBeenCalledTimes(1)
      expect(tutor).not.toHaveBeenCalled()
      expect(loader.capabilities.value).toEqual(['admin.access'])
    } finally {
      isDynamicRole.mockRestore()
    }
  })
})

describe('动态能力装载：真实 auth store → router guard 组合（spec #2920 切片 4）', () => {
  const adminDashboard: GuardInput = {
    path: '/admin/dashboard',
    fullPath: '/admin/dashboard',
    name: 'AdminDashboard',
    meta: { requiresAuth: true, workspace: 'manage', role: 'admin', capability: 'admin.access' },
    matched: [{ requiresAuth: true, workspace: 'manage', role: 'admin', capability: 'admin.access' }]
  }

  function stateFromStore(store: ReturnType<typeof useAuthStore>, role: 'admin' | 'tutor'): GuardState {
    return {
      isLoggedIn: store.isLoggedIn,
      role: store.userInfo?.role ?? '',
      hasValidToken: !!(store.token && store.isLoggedIn && store.userInfo?.role),
      subdomain: role,
      ipDirect: true,
      capabilities: store.capabilities,
      capabilitiesSettled: store.capabilitiesSettled
    }
  }

  it('loading 不判权：守卫只消费 store 的落定标志并等待装载', async () => {
    getUserInfo.mockResolvedValue({ pending_fields: [], account: 'admin1', role: 'admin', user_id: 1, username: 'admin1' })
    fetchMyCapabilities.mockImplementationOnce(() => new Promise(() => {}) as never)

    const store = useAuthStore()
    store.setAuthData({ token: 't-admin', role: 'admin', account: 'admin1', user_id: 1 } as never)

    expect(store.capabilityState).toBe('loading')
    expect(resolveGuardDecision(adminDashboard, stateFromStore(store, 'admin'))).toEqual({
      action: 'load-capabilities'
    })
  })

  it('failed 显式重试成功后重新判定落点，而不是停留在无权限页', async () => {
    getUserInfo.mockResolvedValue({ pending_fields: [], account: 'admin1', role: 'admin', user_id: 1, username: 'admin1' })
    fetchMyCapabilities
      .mockRejectedValueOnce(new Error('boom'))
      .mockResolvedValueOnce({ capabilities: ['audit.read'], granted: true })

    const store = useAuthStore()
    store.setAuthData({ token: 't-admin', role: 'admin', account: 'admin1', user_id: 1 } as never)
    await flushPromises()

    expect(store.capabilityState).toBe('failed')
    expect(resolveGuardDecision(adminDashboard, stateFromStore(store, 'admin'))).toEqual({
      action: 'redirect',
      to: { name: 'AdminNoAccess' }
    })

    await store.loadCapabilities()

    expect(store.capabilityState).toBe('ready')
    expect(resolveGuardDecision(adminDashboard, stateFromStore(store, 'admin'))).toEqual({
      action: 'redirect',
      to: { name: 'AuditLogs' }
    })
  })

  it('并发进入受保护页面复用同一次在途装载', async () => {
    getUserInfo.mockResolvedValue({ pending_fields: [], account: 'admin1', role: 'admin', user_id: 1, username: 'admin1' })
    let resolveCaps: (value: unknown) => void = () => {}
    fetchMyCapabilities.mockImplementationOnce(
      () => new Promise(resolve => { resolveCaps = resolve }) as never
    )

    const store = useAuthStore()
    store.setAuthData({ token: 't-admin', role: 'admin', account: 'admin1', user_id: 1 } as never)
    const first = store.loadCapabilities()
    const second = store.loadCapabilities()
    resolveCaps({ capabilities: ['admin.access'], granted: true })
    await Promise.all([first, second])

    expect(fetchMyCapabilities).toHaveBeenCalledTimes(1)
    expect(resolveGuardDecision(adminDashboard, stateFromStore(store, 'admin'))).toEqual({ action: 'allow' })
  })

  it('讲师复用同一装载协议：loading 等待，落定后守卫按讲师能力放行', async () => {
    getUserInfo.mockResolvedValue({ pending_fields: [], account: 'tutor1', role: 'tutor', user_id: 9, username: 'tutor1' })
    let resolveCaps: (value: unknown) => void = () => {}
    fetchTutorCapabilities.mockImplementationOnce(
      () => new Promise(resolve => { resolveCaps = resolve }) as never
    )
    const tutorDashboard: GuardInput = {
      path: '/training/tutor',
      fullPath: '/training/tutor',
      name: 'TutorDashboard',
      meta: { requiresAuth: true, workspace: 'tutor', role: 'tutor', capability: 'tutor.access' },
      matched: [{ requiresAuth: true, workspace: 'tutor', role: 'tutor', capability: 'tutor.access' }]
    }

    const store = useAuthStore()
    store.setAuthData({ token: 't-tutor', role: 'tutor', account: 'tutor1', user_id: 9 } as never)

    expect(resolveGuardDecision(tutorDashboard, stateFromStore(store, 'tutor'))).toEqual({
      action: 'load-capabilities'
    })
    resolveCaps({ capabilities: ['tutor.access'], granted: true })
    await flushPromises()

    expect(fetchMyCapabilities).not.toHaveBeenCalled()
    expect(resolveGuardDecision(tutorDashboard, stateFromStore(store, 'tutor'))).toEqual({ action: 'allow' })
  })

  it('identity key 变化立即使旧在途请求失效，旧结果不得写入新账号', async () => {
    getUserInfo.mockResolvedValue({ pending_fields: [], account: 'admin2', role: 'admin', user_id: 2, username: 'admin2' })
    let resolveOldCaps: (value: unknown) => void = () => {}
    fetchMyCapabilities
      .mockImplementationOnce(() => new Promise(resolve => { resolveOldCaps = resolve }) as never)
      .mockResolvedValueOnce({ capabilities: ['admin.access'], granted: true })

    const store = useAuthStore()
    store.setAuthData({ token: 't-admin-1', role: 'admin', account: 'admin1', user_id: 1 } as never)
    await flushPromises()
    expect(store.userInfo.user_id).toBe(2)

    resolveOldCaps({ capabilities: ['audit.read'], granted: true })
    await flushPromises()

    expect(store.capabilities).toEqual([])
    expect(store.capabilityState).toBe('idle')

    await store.loadCapabilities()
    expect(fetchMyCapabilities).toHaveBeenCalledTimes(2)
    expect(store.capabilities).toEqual(['admin.access'])
  })
})
