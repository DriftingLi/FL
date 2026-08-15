// export.ts downloadExport 契约测试（#230）：
// 后端随 Content-Disposition 下发文件名，前端优先读取响应头、拿不到回退 exportFallbackNames。
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn() }
}))

vi.mock('@/composables/useReportDownload', () => ({
  downloadBlob: vi.fn(),
  dispositionHeaderValue: (v: unknown) => (typeof v === 'string' && v.trim() !== '' ? v : null)
}))

import { unwrappedRequest } from '@/api/request'
import { downloadBlob } from '@/composables/useReportDownload'
import { downloadExport } from '../export'

const mockGet = vi.mocked(unwrappedRequest.get)
const mockDownloadBlob = vi.mocked(downloadBlob)

beforeEach(() => {
  mockGet.mockClear()
  mockDownloadBlob.mockClear()
})

describe('downloadExport', () => {
  it('有 Content-Disposition 响应头时优先使用响应头文件名（后端为唯一真值）', async () => {
    mockGet.mockResolvedValue({
      data: new Blob(['x']),
      headers: {
        'content-disposition': "attachment; filename=\"export.csv\"; filename*=UTF-8''%E5%AD%A6%E5%91%98%E5%90%8D%E5%8D%95.csv"
      }
    })
    await downloadExport('students')
    expect(mockGet).toHaveBeenCalledWith('/admin/export/students', expect.objectContaining({ responseType: 'blob', raw: true }))
    expect(mockDownloadBlob).toHaveBeenCalledTimes(1)
    const [blob, fallback, opts] = mockDownloadBlob.mock.calls[0]
    expect(blob).toBeInstanceOf(Blob)
    expect(fallback).toBe('学员名单.csv')
    expect(opts?.disposition).toContain('%E5%AD%A6')
  })

  it('拿不到 Content-Disposition 时回退显式文件名', async () => {
    mockGet.mockResolvedValue({ data: new Blob(['x']), headers: {} })
    await downloadExport('evaluations')
    const [, fallback, opts] = mockDownloadBlob.mock.calls[0]
    expect(fallback).toBe('评估记录.csv')
    expect(opts?.disposition).toBeNull()
  })
})