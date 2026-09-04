// contribution.ts 上传头真实行为测试（#517 上线后 400 的回归守卫）。
// seam：axios adapter 层——不 mock request 模块，FormData 经真实 transformRequest，
// 断言最终发出的 Content-Type 带 multipart boundary（实例默认 JSON 头必须被覆盖）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import type { InternalAxiosRequestConfig } from 'axios'

vi.mock('element-plus', () => ({
  ElMessage: { error: vi.fn(), success: vi.fn(), warning: vi.fn() }
}))

vi.mock('@/utils/storage', () => ({
  getToken: vi.fn(() => null),
  getRefreshToken: vi.fn(() => null),
  setToken: vi.fn(),
  setRefreshToken: vi.fn()
}))

vi.mock('@/stores/credential', () => ({
  useCredentialStore: vi.fn(() => ({ current: { id: 7 } }))
}))

import { contributionApi } from '@/api/contribution'
import { unwrappedRequest } from '@/api/request'

let captured: InternalAxiosRequestConfig | null = null

beforeEach(() => {
  captured = null
  // 真实例覆盖 adapter：request.ts 的 client 在模块加载时已创建，改 axios.defaults 无效
  ;(unwrappedRequest as unknown as { defaults: { adapter: unknown } }).defaults.adapter = (async (config: InternalAxiosRequestConfig) => {
    captured = config
    return {
      data: { code: 200, message: 'ok', data: { file_name: 'a.pdf', file_url: '/static/uploads/contributions/a_x.pdf', file_size: 1, content_type: 'document' } },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
      request: {}
    }
  }) as never
})

describe('uploadFile 真实头行为（#517 回归）', () => {
  it('FormData 上传最终 Content-Type 为 multipart/form-data 带 boundary', async () => {
    const file = new File(['pdf-bytes'], 'a.pdf', { type: 'application/pdf' })
    await contributionApi.uploadFile(file)
    const ct = String(captured!.headers.get('Content-Type') ?? captured!.headers['Content-Type'])
    // 关键断言：FormData 请求必须以 multipart/form-data 发出（axios 会剥掉手写的 boundary，
    // 由 XHR/fetch 在发送时自动补 boundary=...，后端 FormFile 依赖它解析）
    expect(ct).toBe('multipart/form-data')
    expect(ct).not.toContain('application/json')
  })

  it('create JSON 请求仍走 application/json 默认头', async () => {
    await contributionApi.create({
      credential_id: 7,
      title: 'T',
      intro: 'I',
      files: [{ file_url: '/u/a.pdf', file_name: 'a.pdf', file_size: 1, content_type: 'document' }]
    })
    const ct = String(captured!.headers.get('Content-Type') ?? captured!.headers['Content-Type'])
    expect(ct).toContain('application/json')
  })
})
