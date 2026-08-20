// favorite.ts 契约测试：多态收藏端点路径与参数（ADR-0018）。
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn(), post: vi.fn(), delete: vi.fn() }
}))

import { unwrappedRequest } from '@/api/request'
import { favoriteApi } from '../favorite'

const mockGet = vi.mocked(unwrappedRequest.get)
const mockPost = vi.mocked(unwrappedRequest.post)
const mockDelete = vi.mocked(unwrappedRequest.delete)

beforeEach(() => {
  mockGet.mockClear()
  mockPost.mockClear()
  mockDelete.mockClear()
})

describe('favoriteApi', () => {
  it('list：路径 /favorites，target_type/page 透传', async () => {
    mockGet.mockResolvedValue({ favorites: [], total: 0, page: 1, pages: 0 })
    await favoriteApi.list({ target_type: 'course', page: 2, page_size: 20 })
    expect(mockGet).toHaveBeenCalledWith('/favorites', {
      params: { target_type: 'course', page: 2, page_size: 20 }
    })
  })

  it('add：POST /favorites，body 携带 target_type/target_id', async () => {
    mockPost.mockResolvedValue({ favorite_id: 1, target_type: 'topic', target_id: 5 })
    await favoriteApi.add({ target_type: 'topic', target_id: 5 })
    expect(mockPost).toHaveBeenCalledWith('/favorites', { target_type: 'topic', target_id: 5 })
  })

  it('remove：DELETE /favorites/:id', async () => {
    mockDelete.mockResolvedValue(null)
    await favoriteApi.remove(9)
    expect(mockDelete).toHaveBeenCalledWith('/favorites/9')
  })

  it('check：路径 /favorites/check，target_type/target_id 作为 query', async () => {
    mockGet.mockResolvedValue({ favorited: true, favorite_id: 9 })
    await favoriteApi.check({ target_type: 'course', target_id: 1 })
    expect(mockGet).toHaveBeenCalledWith('/favorites/check', {
      params: { target_type: 'course', target_id: 1 }
    })
  })
})
