// realExam.ts 契约测试：真题套卷端点路径与参数（ADR-0022）。
// credential_id 注入已下沉主 client 拦截器（#387），api 层只保证 params 占位存在。
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn(), post: vi.fn(), delete: vi.fn() }
}))

import { unwrappedRequest } from '@/api/request'
import { realExamApi } from '../realExam'

const mockGet = vi.mocked(unwrappedRequest.get)
const mockPost = vi.mocked(unwrappedRequest.post)

beforeEach(() => {
  mockGet.mockClear()
  mockPost.mockClear()
})

describe('realExamApi', () => {
  it('listPapers：GET /real-exam/papers，params 占位由拦截器注入 credential_id', async () => {
    mockGet.mockResolvedValue([])
    await realExamApi.listPapers()
    expect(mockGet).toHaveBeenCalledWith('/real-exam/papers', { params: {} })
  })

  it('startPractice：GET /real-exam/papers/:id/practice', async () => {
    mockGet.mockResolvedValue({ questions: [], current_index: 0, total: 0, completed: 0 })
    await realExamApi.startPractice(42)
    expect(mockGet).toHaveBeenCalledWith('/real-exam/papers/42/practice')
  })

  it('startExam：POST /real-exam/papers/:id/exam', async () => {
    mockPost.mockResolvedValue({ mock_exam_id: 1 })
    await realExamApi.startExam(42)
    expect(mockPost).toHaveBeenCalledWith('/real-exam/papers/42/exam')
  })

  it('redeemPaper：POST /real-exam/papers/:id/redeem', async () => {
    mockPost.mockResolvedValue({ balance: 0, total_earned: 0, sku: 'real_paper:42', ref_id: '42' })
    await realExamApi.redeemPaper(42)
    expect(mockPost).toHaveBeenCalledWith('/real-exam/papers/42/redeem')
  })
})
