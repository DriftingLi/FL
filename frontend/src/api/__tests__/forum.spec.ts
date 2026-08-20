// forum.ts 互动契约测试：点赞/举报/我的帖子/我的回复/管理端举报（ADR-0018）。
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn(), post: vi.fn(), delete: vi.fn(), put: vi.fn() }
}))

import { unwrappedRequest } from '@/api/request'
import { forumApi, adminForumApi } from '../forum'

const mockGet = vi.mocked(unwrappedRequest.get)
const mockPost = vi.mocked(unwrappedRequest.post)
const mockDelete = vi.mocked(unwrappedRequest.delete)
const mockPut = vi.mocked(unwrappedRequest.put)

beforeEach(() => {
  mockGet.mockClear()
  mockPost.mockClear()
  mockDelete.mockClear()
  mockPut.mockClear()
})

describe('forumApi 互动', () => {
  it('likeTopic：POST /forum/topics/:id/like', async () => {
    mockPost.mockResolvedValue({ likes_count: 1, liked: true })
    await forumApi.likeTopic(7)
    expect(mockPost).toHaveBeenCalledWith('/forum/topics/7/like')
  })

  it('unlikeTopic：DELETE /forum/topics/:id/like', async () => {
    mockDelete.mockResolvedValue({ likes_count: 0, liked: false })
    await forumApi.unlikeTopic(7)
    expect(mockDelete).toHaveBeenCalledWith('/forum/topics/7/like')
  })

  it('reportTopic：POST /forum/topics/:id/report，body 携带 reason', async () => {
    mockPost.mockResolvedValue(null)
    await forumApi.reportTopic(7, '垃圾广告')
    expect(mockPost).toHaveBeenCalledWith('/forum/topics/7/report', { reason: '垃圾广告' })
  })

  it('reportReply：POST /forum/replies/:id/report', async () => {
    mockPost.mockResolvedValue(null)
    await forumApi.reportReply(12, '人身攻击')
    expect(mockPost).toHaveBeenCalledWith('/forum/replies/12/report', { reason: '人身攻击' })
  })

  it('getMyTopics：路径 /forum/my-topics，分页参数透传', async () => {
    mockGet.mockResolvedValue({ topics: [], total: 0, page: 1, pages: 0 })
    await forumApi.getMyTopics({ page: 1, page_size: 10 })
    expect(mockGet).toHaveBeenCalledWith('/forum/my-topics', { params: { page: 1, page_size: 10 } })
  })

  it('getMyReplies：路径 /forum/my-replies', async () => {
    mockGet.mockResolvedValue({ replies: [], total: 0, page: 1, pages: 0 })
    await forumApi.getMyReplies({ page: 2 })
    expect(mockGet).toHaveBeenCalledWith('/forum/my-replies', { params: { page: 2 } })
  })
})

describe('adminForumApi 举报管理', () => {
  it('listReports：路径 /admin/forum/reports，status/page 透传', async () => {
    mockGet.mockResolvedValue({ reports: [], total: 0, page: 1, pages: 0 })
    await adminForumApi.listReports({ status: 0, page: 1, page_size: 20 })
    expect(mockGet).toHaveBeenCalledWith('/admin/forum/reports', {
      params: { status: 0, page: 1, page_size: 20 }
    })
  })

  it('handleReport：PUT /admin/forum/reports/:id，body 携带 status', async () => {
    mockPut.mockResolvedValue(null)
    await adminForumApi.handleReport(3, 1)
    expect(mockPut).toHaveBeenCalledWith('/admin/forum/reports/3', { status: 1 })
  })
})
