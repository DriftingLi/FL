// HTTP 客户端工厂（client.ts）单测：信封解包 / 业务失败抛错 + toast / 401 分发 / 证件作用域语义。
// seam：axios adapter 层（mock adapter 模拟后端响应），不经过真实网络。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import axios, { AxiosError } from 'axios'

vi.mock('element-plus', () => ({
  ElMessage: { error: vi.fn(), success: vi.fn(), warning: vi.fn() }
}))

// 隔离 localStorage 环境差异（getToken 直读 localStorage）：证件注入相关用例只关心 params 改写，
// 无 token 也能走到注入分支
vi.mock('@/utils/storage', () => ({
  getToken: vi.fn(() => null),
  getRefreshToken: vi.fn(() => null),
  setToken: vi.fn(),
  setRefreshToken: vi.fn()
}))

import { ElMessage } from 'element-plus'
import { createHttpClient, createDefaultUnauthorizedPolicy } from '../client'

type Respond = (config: { url?: string; headers?: Record<string, unknown> }) => { status: number; body: unknown }

let respond: Respond

beforeEach(() => {
  vi.clearAllMocks()
  axios.defaults.adapter = (async (config: any) => {
    const { status, body } = respond(config)
    const response = {
      data: body,
      status,
      statusText: status >= 400 ? 'Error' : 'OK',
      headers: { 'content-type': 'application/json' },
      config,
      request: {}
    }
    // 模拟真实 axios 行为：非 2xx 以带 response 的 AxiosError 拒绝（触发拦截器错误分支）
    if (status >= 400) {
      throw new AxiosError(
        `Request failed with status code ${status}`,
        'ERR_BAD_REQUEST',
        config,
        null,
        response as never
      )
    }
    return response
  }) as never
})

function makeClient() {
  return createHttpClient({
    baseURL: '/api',
    onUnauthorized: vi.fn()
  })
}

describe('createHttpClient 信封解包', () => {
  it('成功信封（code 200）直接返回业务负载 data', async () => {
    respond = () => ({ status: 200, body: { code: 200, message: 'ok', data: { id: 1, name: '测试' } } })
    const client = makeClient()

    const res = await client.get<{ id: number; name: string }>('/courses')

    expect(res).toEqual({ id: 1, name: '测试' })
  })

  it('成功信封（code 201）同样解包返回 data', async () => {
    respond = () => ({ status: 201, body: { code: 201, message: 'created', data: { id: 9 } } })
    const client = makeClient()

    const res = await client.post<{ id: number }>('/courses', {})

    expect(res).toEqual({ id: 9 })
  })

  it('成功信封无 data 字段时解包返回 undefined', async () => {
    respond = () => ({ status: 200, body: { code: 200, message: 'ok' } })
    const client = makeClient()

    const res = await client.get<null>('/noop')

    expect(res).toBeUndefined()
  })

  it('业务失败（信封 code 非成功）抛错并统一 toast', async () => {
    respond = () => ({ status: 200, body: { code: 400, message: '业务失败', data: null } })
    const client = makeClient()

    await expect(client.get('/x')).rejects.toThrow('业务失败')
    expect(ElMessage.error).toHaveBeenCalledWith('业务失败')
  })

  it('X-Silent 静默请求业务失败抛错但不弹 toast', async () => {
    respond = () => ({ status: 200, body: { code: 400, message: '静默失败', data: null } })
    const client = makeClient()

    await expect(client.get('/x', { headers: { 'X-Silent': '1' } })).rejects.toThrow('静默失败')
    expect(ElMessage.error).not.toHaveBeenCalled()
  })

  it('HTTP 401 触发 onUnauthorized 并弹登录过期 toast', async () => {
    const onUnauthorized = vi.fn()
    respond = () => ({ status: 401, body: { code: 401, message: '登录已过期' } })
    const client = createHttpClient({ baseURL: '/api', onUnauthorized })

    await expect(client.get('/x')).rejects.toBeTruthy()
    expect(onUnauthorized).toHaveBeenCalledTimes(1)
    expect(ElMessage.error).toHaveBeenCalledWith('登录已过期，请重新登录')
  })

  it('blob 响应直接放行返回二进制数据', async () => {
    const blob = new Blob(['pdf'])
    respond = () => ({ status: 200, body: blob })
    const client = makeClient()

    const res = await client.get<Blob>('/report', { responseType: 'blob' })

    expect(res).toBeInstanceOf(Blob)
    expect(res).toBe(blob)
  })
})

describe('createDefaultUnauthorizedPolicy 统一 401 策略', () => {
  it('redirect=false（AI 助手）：仅清登录态，不跳转', async () => {
    const clearAuth = vi.fn()
    const policy = createDefaultUnauthorizedPolicy({ clearAuth, redirect: false })

    policy()

    expect(clearAuth).toHaveBeenCalledTimes(1)
  })

  it('默认 redirect：清登录态后按 resolveLoginPath 解析跳转目标', async () => {
    const clearAuth = vi.fn()
    const resolveLoginPath = vi.fn().mockReturnValue('/valuation/login')
    const policy = createDefaultUnauthorizedPolicy({ clearAuth, resolveLoginPath })

    policy()

    expect(clearAuth).toHaveBeenCalledTimes(1)
    // router 为延迟动态引入（异步），此处只验证策略已触发清态
  })
})

// ===== 证件作用域（ADR-0047 §4 / spec #931）=====
//
// 客户端**不再**维护「哪些端点要注入 credential_id」的豁免表——事实源在服务端：
// 受作用域端点在注册时声明作用域，缺省读用户当前证件。这里锁两件事：
//   1. 请求拦截器不改写任何请求参数（旧豁免表与注入实现已删除）；
//   2. 显式传入的 credential_id 原样透传（**公开路由**仍需调用方显式传，服务端无从兜底）。
describe('请求拦截器：不做证件注入（事实源在服务端）', () => {
  const captureParams = (): { value: unknown } => {
    const box = { value: undefined as unknown }
    respond = config => {
      box.value = (config as { params?: unknown }).params
      return { status: 200, body: { code: 200, message: 'ok', data: { total: 0 } } }
    }
    return box
  }

  it('params 原样透传，拦截器不添加 credential_id', async () => {
    const seen = captureParams()
    const client = createHttpClient({ baseURL: '/api', onUnauthorized: () => {} })
    await client.get('/question-bank/stats', { params: { page: 1 } })
    expect(seen.value).toEqual({ page: 1 })
  })

  it('显式传入的 credential_id 原样透传', async () => {
    const seen = captureParams()
    const client = createHttpClient({ baseURL: '/api', onUnauthorized: () => {} })
    await client.get('/question-bank/stats', { params: { credential_id: 7 } })
    expect(seen.value).toEqual({ credential_id: 7 })
  })
})
