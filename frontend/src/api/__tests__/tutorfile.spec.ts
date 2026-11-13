// tutorfile.ts 断点续传契约测试（#2495 / spec A8）：
// 客户端持久化「会话 id + 已完成分片清单」，中断后保留会话、恢复只补缺片（服务端清单为准）。
// 注意：分片 PUT 走 unwrappedRequest.put（不是 post），init/complete 走 post。
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn(), post: vi.fn(), delete: vi.fn(), put: vi.fn() }
}))

import { unwrappedRequest } from '@/api/request'
import { tutorFileApi, uploadFileInChunks } from '../tutorfile'

const mockGet = vi.mocked(unwrappedRequest.get)
const mockPost = vi.mocked(unwrappedRequest.post)
const mockPut = vi.mocked(unwrappedRequest.put)

function makeFile(size: number, name = 'a.mp4'): File {
  return new File([new Uint8Array(size)], name, { type: 'video/mp4' })
}

// localStorage 键随 File 实例的 lastModified（happy-dom 可能不认构造参数里的固定值，故按实例推导）
function stateKey(f: File): string {
  return `tutorfile-upload:${f.name}:${f.size}:${f.lastModified}` 
}

beforeEach(() => {
  mockGet.mockReset()
  mockPost.mockReset()
  mockPut.mockReset()
  localStorage.clear()
})

describe('uploadFileInChunks 断点续传（#2495）', () => {
  it('全新上传：init → 逐片 → complete，完成后清持久化', async () => {
    const file = makeFile(12)
    mockPost
      .mockResolvedValueOnce({ session_id: 's1', chunk_size: 5, total_chunks: 3 }) // init
      .mockResolvedValueOnce({ file_id: 9, file_url: '/u', file_name: 'a.mp4', content_type: 'video', file_size: 12, created_at: '' }) // complete
    mockPut.mockResolvedValue({ part_number: 1, size: 5 })

    const res = await uploadFileInChunks(file)
    expect(res.file_id).toBe(9)
    expect(mockPost).toHaveBeenCalledTimes(2) // init + complete
    expect(mockPut).toHaveBeenCalledTimes(3) // 三片
    expect(localStorage.getItem(stateKey(file))).toBeNull() // 完成即清
  })

  it('中断保留会话：分片失败不 abort、进度持久化（供下次续传）', async () => {
    const file = makeFile(12)
    mockPost.mockResolvedValueOnce({ session_id: 's1', chunk_size: 5, total_chunks: 3 })
    mockPut
      .mockResolvedValueOnce({ part_number: 1, size: 5 })
      .mockRejectedValueOnce(new Error('network'))

    await expect(uploadFileInChunks(file)).rejects.toThrow('network')
    // 没有调 abort（留会话）
    const abortCalls = mockPost.mock.calls.filter(([url]) => String(url).includes('/abort'))
    expect(abortCalls).toHaveLength(0)
    // 持久化里有 s1 与已完成分片 [1]
    const state = JSON.parse(localStorage.getItem(stateKey(file))!)
    expect(state.session_id).toBe('s1')
    expect(state.done_parts).toEqual([1])
  })

  it('恢复只补缺片：服务端清单 [1,3] → 只传第 2 片', async () => {
    const file = makeFile(12)
    localStorage.setItem(
      stateKey(file),
      JSON.stringify({ session_id: 's1', file_name: 'a.mp4', file_size: 12, done_parts: [1] })
    )
    mockGet.mockResolvedValueOnce({
      session_id: 's1',
      file_name: 'a.mp4',
      file_size: 12,
      chunk_size: 5,
      total_chunks: 3,
      status: 'active',
      received_parts: [1, 3] // 服务端为准：3 也已收
    })
    mockPost.mockResolvedValueOnce({ file_id: 9, file_url: '/u', file_name: 'a.mp4', content_type: 'video', file_size: 12, created_at: '' })
    mockPut.mockResolvedValue({ part_number: 2, size: 5 })

    const res = await uploadFileInChunks(file)
    expect(res.file_id).toBe(9)
    // 只补缺片：init 没再调（post 只有 complete 一次），chunk 只调了 part 2
    const initCalls = mockPost.mock.calls.filter(([url, body]) => String(url).endsWith('/upload') && body)
    expect(initCalls).toHaveLength(0)
    expect(mockPut).toHaveBeenCalledTimes(1)
    expect(String(mockPut.mock.calls[0][0])).toContain('/chunk/2')
    expect(localStorage.getItem(stateKey(file))).toBeNull()
  })

  it('会话已失效：清单查询失败 → 清旧档从头 init', async () => {
    const file = makeFile(12)
    localStorage.setItem(
      stateKey(file),
      JSON.stringify({ session_id: 'gone', file_name: 'a.mp4', file_size: 12, done_parts: [1] })
    )
    mockGet.mockRejectedValueOnce(new Error('404'))
    mockPost
      .mockResolvedValueOnce({ session_id: 's2', chunk_size: 5, total_chunks: 3 })
      .mockResolvedValueOnce({ file_id: 9, file_url: '/u', file_name: 'a.mp4', content_type: 'video', file_size: 12, created_at: '' })
    mockPut.mockResolvedValue({ part_number: 1, size: 5 })

    await uploadFileInChunks(file)
    // 新 init 发生（一次带 body 的 /upload post）
    const initCalls = mockPost.mock.calls.filter(([url, body]) => String(url).endsWith('/upload') && body)
    expect(initCalls).toHaveLength(1)
    // 三片全传（从头来）
    expect(mockPut).toHaveBeenCalledTimes(3)
  })
})

describe('tutorFileApi 端点形状（#2495）', () => {
  it('getSession：GET /tutor/files/upload/:session_id', async () => {
    mockGet.mockResolvedValue({ session_id: 's1' })
    await tutorFileApi.getSession('s1')
    expect(mockGet).toHaveBeenCalledWith('/tutor/files/upload/s1')
  })
})

describe('tutorFileApi 挂接与管理面（#2497）', () => {
  it('listFiles：筛选参数映射 query', async () => {
    mockGet.mockResolvedValue({ files: [] })
    await tutorFileApi.listFiles({
      content_type: 'video',
      attached: true,
      created_from: '2026-11-01',
      created_to: '2026-11-13'
    })
    expect(mockGet).toHaveBeenCalledWith('/tutor/files', {
      params: {
        content_type: 'video',
        attached: 'true',
        created_from: '2026-11-01',
        created_to: '2026-11-13'
      }
    })
  })

  it('attach/detach/rename/swap 端点形状', async () => {
    mockPost.mockResolvedValue(null)
    await tutorFileApi.attachFile(7, 3)
    expect(mockPost).toHaveBeenLastCalledWith('/tutor/files/7/attach', { chapter_id: 3 })
    await tutorFileApi.detachFile(7)
    expect(mockPost).toHaveBeenLastCalledWith('/tutor/files/7/detach')
    await tutorFileApi.renameFile(7, '新名字.mp4')
    expect(mockPost).toHaveBeenLastCalledWith('/tutor/files/7/rename', { file_name: '新名字.mp4' })
    await tutorFileApi.swapChapterFile(3, 7, 8)
    expect(mockPost).toHaveBeenLastCalledWith('/tutor/chapter/3/files/swap', {
      old_file_id: 7,
      new_file_id: 8
    })
  })
})
