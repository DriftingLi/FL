// inspection.ts 契约测试（ADR-0053 §7）：管理端巡检面 9 个调用点收回请求层后，
// 端点路径、筛选轴透传与静默通道语义都要钉住。
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn(), post: vi.fn() }
}))

import { unwrappedRequest } from '@/api/request'
import { inspectionApi } from '../inspection'

const mockGet = vi.mocked(unwrappedRequest.get)
const mockPost = vi.mocked(unwrappedRequest.post)

const SILENT = { headers: { 'X-Silent': '1' } }

beforeEach(() => {
  mockGet.mockClear()
  mockPost.mockClear()
})

describe('inspectionApi', () => {
  it('pointsLedger：GET /admin/points/ledger，筛选轴作为 query 且走静默通道', async () => {
    mockGet.mockResolvedValue({ items: [], total: 0 })
    await inspectionApi.pointsLedger({ page: 2, page_size: 20, ref_type: 'forum_topic' })
    expect(mockGet).toHaveBeenCalledWith('/admin/points/ledger', {
      params: { page: 2, page_size: 20, ref_type: 'forum_topic' },
      ...SILENT
    })
  })

  it('deletedAfterAccepted：GET /admin/inspection/deleted-after-accepted，静默', async () => {
    mockGet.mockResolvedValue({ count: 3 })
    await inspectionApi.deletedAfterAccepted()
    expect(mockGet).toHaveBeenCalledWith('/admin/inspection/deleted-after-accepted', SILENT)
  })

  it('resumeViews：GET /admin/recruit/views，分页参数 + 静默', async () => {
    mockGet.mockResolvedValue({ items: [], total: 0 })
    await inspectionApi.resumeViews({ page: 1, page_size: 20 })
    expect(mockGet).toHaveBeenCalledWith('/admin/recruit/views', {
      params: { page: 1, page_size: 20 },
      ...SILENT
    })
  })

  it('contactRequests：GET /admin/recruit/requests', async () => {
    mockGet.mockResolvedValue({ items: [], total: 0 })
    await inspectionApi.contactRequests({ page: 1, page_size: 20 })
    expect(mockGet).toHaveBeenCalledWith('/admin/recruit/requests', {
      params: { page: 1, page_size: 20 },
      ...SILENT
    })
  })

  it('jobs：GET /admin/jobs，recruiter_id 过滤透传', async () => {
    mockGet.mockResolvedValue({ items: [], total: 0 })
    await inspectionApi.jobs({ page: 1, page_size: 20, recruiter_id: '5' })
    expect(mockGet).toHaveBeenCalledWith('/admin/jobs', {
      params: { page: 1, page_size: 20, recruiter_id: '5' },
      ...SILENT
    })
  })

  it('jobReports：GET /admin/job-reports', async () => {
    mockGet.mockResolvedValue({ items: [], total: 0 })
    await inspectionApi.jobReports({ page: 3, page_size: 20 })
    expect(mockGet).toHaveBeenCalledWith('/admin/job-reports', {
      params: { page: 3, page_size: 20 },
      ...SILENT
    })
  })

  it('forceOfflineJob：POST /admin/jobs/:id/force-offline，body 为 reason', async () => {
    mockPost.mockResolvedValue(null)
    await inspectionApi.forceOfflineJob(12, '资质不符')
    expect(mockPost).toHaveBeenCalledWith('/admin/jobs/12/force-offline', { reason: '资质不符' })
  })

  it('handleJobReport：POST /admin/job-reports/:id/handle', async () => {
    mockPost.mockResolvedValue(null)
    await inspectionApi.handleJobReport(3)
    expect(mockPost).toHaveBeenCalledWith('/admin/job-reports/3/handle')
  })
})
