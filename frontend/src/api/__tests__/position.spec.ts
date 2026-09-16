// position.ts 契约测试（ADR-0053 §7）：页面直连的端点收回请求层后，URL / 方法 / 参数拼装
// 与「静默通道」语义都要钉住 —— 否则这次收口只搬了位置、没保住行为。
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() }
}))

import { unwrappedRequest } from '@/api/request'
import { positionApi } from '../position'

const mockGet = vi.mocked(unwrappedRequest.get)
const mockPost = vi.mocked(unwrappedRequest.post)
const mockPut = vi.mocked(unwrappedRequest.put)
const mockDelete = vi.mocked(unwrappedRequest.delete)

const SILENT = { headers: { 'X-Silent': '1' } }

beforeEach(() => {
  mockGet.mockClear()
  mockPost.mockClear()
  mockPut.mockClear()
  mockDelete.mockClear()
})

describe('positionApi', () => {
  it('listPublic：GET /positions；不传 silent 时无请求头选项', async () => {
    mockGet.mockResolvedValue({ positions: [] })
    await positionApi.listPublic()
    expect(mockGet).toHaveBeenCalledWith('/positions', undefined)
  })

  it('listPublic：silent=true 时带 X-Silent（选项字典加载失败不弹 toast）', async () => {
    mockGet.mockResolvedValue({ positions: [] })
    await positionApi.listPublic({ silent: true })
    expect(mockGet).toHaveBeenCalledWith('/positions', SILENT)
  })

  it('listAdmin：GET /admin/positions，silent 同上', async () => {
    mockGet.mockResolvedValue({ positions: [] })
    await positionApi.listAdmin({ silent: true })
    expect(mockGet).toHaveBeenCalledWith('/admin/positions', SILENT)
  })

  it('create：POST /admin/position，body 原样透传', async () => {
    mockPost.mockResolvedValue({ position_id: 1 })
    await positionApi.create({ code: 'driver', name: '叉车司机' })
    expect(mockPost).toHaveBeenCalledWith('/admin/position', { code: 'driver', name: '叉车司机' })
  })

  it('update：PUT /admin/position/:id', async () => {
    mockPut.mockResolvedValue({ position_id: 7 })
    await positionApi.update(7, { name: '维修工' })
    expect(mockPut).toHaveBeenCalledWith('/admin/position/7', { name: '维修工' })
  })

  it('remove：DELETE /admin/position/:id', async () => {
    mockDelete.mockResolvedValue(null)
    await positionApi.remove(7)
    expect(mockDelete).toHaveBeenCalledWith('/admin/position/7')
  })

  it('swap：PUT /admin/position/:id/sort，body 为 swap_with', async () => {
    mockPut.mockResolvedValue(null)
    await positionApi.swap(7, 9)
    expect(mockPut).toHaveBeenCalledWith('/admin/position/7/sort', { swap_with: 9 })
  })
})
