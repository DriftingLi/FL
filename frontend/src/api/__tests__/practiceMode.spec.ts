// practiceMode.ts 契约测试：练习进度保存的证件分桶（#505）。
// 背景：顺序练习进度按「当前证件」分桶（#414），读路径 GET sequential/progress 由主 client
// 拦截器自动注入 query credential_id，但 POST /progress 的凭证只从 JSON body 解析——拦截器
// 不触碰 body，因此保存方必须显式携带。若保存漏带，游标落进 credential_id IS NULL 孤儿行，
// 证件桶游标冻结（断点回跳 bug 根因）。
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn(), post: vi.fn() }
}))

vi.mock('@/stores/credential', () => ({
  useCredentialStore: vi.fn()
}))

import { unwrappedRequest } from '@/api/request'
import { practiceModeApi } from '../practiceMode'
import { useCredentialStore } from '@/stores/credential'

const mockPost = vi.mocked(unwrappedRequest.post)
const mockGet = vi.mocked(unwrappedRequest.get)
const mockUseCredentialStore = vi.mocked(useCredentialStore)

beforeEach(() => {
  mockPost.mockClear()
  mockGet.mockClear()
  mockUseCredentialStore.mockClear()
  mockUseCredentialStore.mockReturnValue({ current: { id: 7, name: '叉车司机N1证' } } as never)
})

describe('practiceModeApi.saveProgress（#505 读写同桶）', () => {
  it('顺序练习（sequential）：保存 body 携带当前证件 credential_id', async () => {
    mockPost.mockResolvedValue(null)
    await practiceModeApi.saveProgress(4, 'sequential', 20, {})
    expect(mockPost).toHaveBeenCalledWith('/practice-mode/progress', {
      index: 4,
      practice_mode: 'sequential',
      total: 20,
      answers_state: {},
      credential_id: 7
    })
  })

  it('无当前证件（未预筛选学员）：不携带 credential_id（NULL 桶兼容）', async () => {
    mockUseCredentialStore.mockReturnValue({ current: null } as never)
    mockPost.mockResolvedValue(null)
    await practiceModeApi.saveProgress(2, 'sequential', 10, {})
    expect(mockPost).toHaveBeenCalledWith('/practice-mode/progress', {
      index: 2,
      practice_mode: 'sequential',
      total: 10,
      answers_state: {},
      credential_id: undefined
    })
  })

  it('标签/按卷练习（非 sequential）：不携带 credential_id（分桶仅顺序练习）', async () => {
    mockPost.mockResolvedValue(null)
    await practiceModeApi.saveProgress(1, 'tag:7', 5, {})
    expect(mockPost).toHaveBeenCalledWith('/practice-mode/progress', {
      index: 1,
      practice_mode: 'tag:7',
      total: 5,
      answers_state: {},
      credential_id: undefined
    })
  })
})

describe('practiceModeApi 既有端点回归', () => {
  it('startSequential：传空 params 走拦截器注入 credential_id', async () => {
    mockGet.mockResolvedValue({ questions: [], progress: {} })
    await practiceModeApi.startSequential()
    expect(mockGet).toHaveBeenCalledWith('/practice-mode/sequential', { params: {} })
  })

  it('getProgress：mode 透传 query', async () => {
    mockGet.mockResolvedValue({ current_index: 0, total: 0, completed: 0 })
    await practiceModeApi.getProgress('sequential')
    expect(mockGet).toHaveBeenCalledWith('/practice-mode/progress', { params: { mode: 'sequential' } })
  })
})
