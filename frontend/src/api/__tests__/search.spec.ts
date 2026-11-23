// search.ts 契约测试：全局搜索端点与参数（ADR-0018）。
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn() }
}))

import { unwrappedRequest } from '@/api/request'
import { searchApi } from '../search'

const mockGet = vi.mocked(unwrappedRequest.get)

beforeEach(() => {
  mockGet.mockClear()
})

describe('searchApi.search', () => {
  it('默认全分类，返回各分区聚合', async () => {
    mockGet.mockResolvedValue({ keyword: '液压', courses: { items: [], total: 0 } })
    await searchApi.search({ keyword: '液压' })
    expect(mockGet).toHaveBeenCalledWith('/search', { params: { keyword: '液压', category_id: 'all' } })
  })

  it('指定 type：type/page/page_size 透传（后端返回分页结果）', async () => {
    mockGet.mockResolvedValue({ keyword: '液压', type: 'course', items: [], total: 0, page: 1, pages: 0 })
    await searchApi.search({ keyword: '液压', type: 'course', page: 2, page_size: 10, category_id: 4 })
    expect(mockGet).toHaveBeenCalledWith('/search', {
      params: { keyword: '液压', type: 'course', page: 2, page_size: 10, category_id: 4 }
    })
  })
})
