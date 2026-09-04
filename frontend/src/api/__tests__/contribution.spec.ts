// contribution.ts 契约测试（#517）：上传头、参数透传与端点形状。
// 防回归重点：uploadFile 必须显式覆盖 multipart 头——共享 client 默认 Content-Type 为
// application/json，不覆盖则 FormData 不带 boundary，后端 FormFile 解析不到文件（上线后 400）。
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn(), post: vi.fn(), delete: vi.fn(), put: vi.fn() }
}))

import { unwrappedRequest } from '@/api/request'
import { contributionApi, adminContributionApi } from '../contribution'

const mockGet = vi.mocked(unwrappedRequest.get)
const mockPost = vi.mocked(unwrappedRequest.post)
const mockDelete = vi.mocked(unwrappedRequest.delete)

beforeEach(() => {
  mockGet.mockClear()
  mockPost.mockClear()
  mockDelete.mockClear()
})

describe('contributionApi 学员端（#517）', () => {
  it('uploadFile：FormData 携带 file 字段 + 显式 multipart 头 + 放宽超时', async () => {
    mockPost.mockResolvedValue({ file_name: 'a.pdf', file_url: '/static/uploads/contributions/a.pdf', file_size: 10, content_type: 'document' })
    const file = new File(['x'], 'a.pdf', { type: 'application/pdf' })
    await contributionApi.uploadFile(file)

    expect(mockPost).toHaveBeenCalledTimes(1)
    const [url, body, config] = mockPost.mock.calls[0]
    expect(url).toBe('/contributions/upload-file')
    expect(body).toBeInstanceOf(FormData)
    expect((body as FormData).get('file')).toBeInstanceOf(File)
    // 关键：multipart 头必须显式覆盖（回归守卫）
    expect((config as any).headers['Content-Type']).toBe('multipart/form-data')
    expect((config as any).timeout).toBe(120000)
  })

  it('create：POST /contributions，body 透传证件/匿名位/文件数组', async () => {
    mockPost.mockResolvedValue({ id: 1, status: 'pending' })
    const payload = {
      credential_id: 3,
      title: 'T',
      intro: 'I',
      is_anonymous: true,
      files: [{ file_url: '/u/a.pdf', file_name: 'a.pdf', file_size: 1, content_type: 'document' }]
    }
    await contributionApi.create(payload)
    expect(mockPost).toHaveBeenCalledWith('/contributions', payload)
  })

  it('listPublic / listMine / download / withdraw / report 端点形状', async () => {
    mockGet.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
    await contributionApi.listPublic({ credential_id: 3, sort: 'hot', page: 2, page_size: 20 })
    expect(mockGet).toHaveBeenCalledWith('/contributions', {
      params: { credential_id: 3, sort: 'hot', page: 2, page_size: 20 }
    })

    await contributionApi.listMine({ page: 1, page_size: 20 })
    expect(mockGet).toHaveBeenLastCalledWith('/contributions/mine', { params: { page: 1, page_size: 20 } })

    mockPost.mockResolvedValue({ is_new: true, tier_awarded: 0 })
    await contributionApi.download(9)
    expect(mockPost).toHaveBeenLastCalledWith('/contributions/9/download')

    await contributionApi.report(9, 'piracy')
    expect(mockPost).toHaveBeenLastCalledWith('/contributions/9/report', { reason: 'piracy' })

    mockDelete.mockResolvedValue(null)
    await contributionApi.withdraw(9)
    expect(mockDelete).toHaveBeenLastCalledWith('/contributions/9')
  })
})

describe('adminContributionApi 管理端（#517）', () => {
  it('审核三端点 + 举报队列/处置形状', async () => {
    mockGet.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
    mockPost.mockResolvedValue({})

    await adminContributionApi.listPending({ page: 1, page_size: 20 })
    expect(mockGet).toHaveBeenLastCalledWith('/admin/contributions/pending', { params: { page: 1, page_size: 20 } })

    await adminContributionApi.approve(4)
    expect(mockPost).toHaveBeenLastCalledWith('/admin/contributions/4/approve')

    await adminContributionApi.reject(4, '不合规')
    expect(mockPost).toHaveBeenLastCalledWith('/admin/contributions/4/reject', { reason: '不合规' })

    await adminContributionApi.archive(4, '盗版')
    expect(mockPost).toHaveBeenLastCalledWith('/admin/contributions/4/archive', { reason: '盗版' })

    await adminContributionApi.listReports({ status: 0 })
    expect(mockGet).toHaveBeenLastCalledWith('/admin/contributions/reports', { params: { status: 0 } })

    await adminContributionApi.handleReport(2, 'archive')
    expect(mockPost).toHaveBeenLastCalledWith('/admin/contributions/reports/2/handle', { action: 'archive' })
  })
})
