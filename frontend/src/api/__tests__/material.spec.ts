// material.ts 契约测试：学习资料聚合端点与参数（ADR-0018）。
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn() }
}))

import { unwrappedRequest } from '@/api/request'
import { materialApi } from '../material'

const mockGet = vi.mocked(unwrappedRequest.get)

beforeEach(() => {
  mockGet.mockClear()
})

describe('materialApi.list', () => {
  it('路径 /materials，course_id/page/page_size 透传', async () => {
    mockGet.mockResolvedValue({ materials: [], total: 0, page: 1, pages: 0 })
    await materialApi.list({ course_id: 3, page: 1, page_size: 20 })
    expect(mockGet).toHaveBeenCalledWith('/materials', {
      params: { course_id: 3, page: 1, page_size: 20 }
    })
  })

  it('course_id 可省略（不过滤课程）', async () => {
    mockGet.mockResolvedValue({ materials: [], total: 0, page: 1, pages: 0 })
    await materialApi.list({ page: 1 })
    expect(mockGet).toHaveBeenCalledWith('/materials', { params: { page: 1 } })
  })
})
