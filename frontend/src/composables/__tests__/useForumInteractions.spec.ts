// useForumImageUpload / useLike：论坛互动语义单点（#389）的接口级测试。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { nextTick, ref } from 'vue'
import { useForumImageUpload } from '../useForumImageUpload'
import { useLike } from '../useLike'

vi.mock('@/api/forum', () => ({
  forumApi: { uploadImage: vi.fn() }
}))
vi.mock('element-plus', () => ({
  ElMessage: { error: vi.fn(), success: vi.fn(), warning: vi.fn() }
}))

import { forumApi } from '@/api/forum'
import { ElMessage } from 'element-plus'

function fileOf(name: string, size: number, type = 'image/png'): File {
  const f = new File(['x'], name, { type })
  Object.defineProperty(f, 'size', { value: size })
  return f
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('useForumImageUpload（上传校验单点）', () => {
  it('上传成功追加 URL；顺序批量', async () => {
    vi.mocked(forumApi.uploadImage).mockResolvedValue({ url: 'http://x/1.png' })
    const { urls, uploading, uploadFiles } = useForumImageUpload(3)

    await uploadFiles([fileOf('a.png', 100), fileOf('b.png', 100)])
    expect(urls.value).toEqual(['http://x/1.png', 'http://x/1.png'])
    expect(uploading.value).toBe(false)
    expect(forumApi.uploadImage).toHaveBeenCalledTimes(2)
  })

  it('超过 20MB 跳过并提示；非图片文件过滤；达到上限即停', async () => {
    vi.mocked(forumApi.uploadImage).mockResolvedValue({ url: 'http://x/1.png' })
    const { urls, uploadFiles } = useForumImageUpload(2)

    await uploadFiles([fileOf('big.png', 21 * 1024 * 1024), fileOf('doc.pdf', 10, 'application/pdf')])
    expect(urls.value).toEqual([])
    expect(ElMessage.error).toHaveBeenCalledWith('"big.png" 超过 20MB，已跳过')

    await uploadFiles([fileOf('1.png', 10), fileOf('2.png', 10), fileOf('3.png', 10)])
    expect(urls.value.length).toBe(2)

    // 已达上限：再传只警告，不发起请求
    const calls = vi.mocked(forumApi.uploadImage).mock.calls.length
    await uploadFiles([fileOf('4.png', 10)])
    expect(ElMessage.warning).toHaveBeenCalledWith('最多上传 2 张图片')
    expect(vi.mocked(forumApi.uploadImage).mock.calls.length).toBe(calls)
  })

  it('handlePaste：剪贴板图片转入上传并阻止默认行为', async () => {
    vi.mocked(forumApi.uploadImage).mockResolvedValue({ url: 'http://x/p.png' })
    const { urls, handlePaste } = useForumImageUpload(3)

    const file = fileOf('p.png', 10)
    const items = [{ kind: 'file', type: 'image/png', getAsFile: () => file }] as unknown as DataTransferItemList
    const event = { clipboardData: { items }, preventDefault: vi.fn() } as unknown as ClipboardEvent

    handlePaste(event)
    await vi.waitFor(() => expect(urls.value).toEqual(['http://x/p.png']))
    expect(event.preventDefault).toHaveBeenCalled()
  })

  it('removeImage 按索引删除', () => {
    const external = ref(['a', 'b', 'c'])
    const { urls, removeImage } = useForumImageUpload(3, { urls: external })
    removeImage(1)
    expect(urls.value).toEqual(['a', 'c'])
    expect(external.value).toEqual(['a', 'c'])
  })
})

describe('useLike（点赞乐观更新 + 失败回滚）', () => {
  it('未赞→点赞：乐观 +1，成功后以服务端结果收敛', async () => {
    const target = { id: 1, liked_by_me: false, likes_count: 5 }
    const like = vi.fn().mockResolvedValue({ likes_count: 6, liked: true })
    const unlike = vi.fn()
    const { toggle } = useLike(like, unlike)

    await nextTick()
    const p = toggle(target)
    expect(target.liked_by_me).toBe(true)
    expect(target.likes_count).toBe(6)
    await p
    expect(like).toHaveBeenCalledWith(1)
    expect(target.liked_by_me).toBe(true)
    expect(target.likes_count).toBe(6)
  })

  it('取消点赞：乐观 -1 且不为负；失败回滚先前置', async () => {
    const target = { id: 2, liked_by_me: true, likes_count: 1 }
    const like = vi.fn()
    const unlike = vi.fn().mockRejectedValue(new Error('net'))
    const { toggle } = useLike(like, unlike)

    await toggle(target)
    expect(unlike).toHaveBeenCalledWith(2)
    expect(target.liked_by_me).toBe(true)
    expect(target.likes_count).toBe(1)
  })
})
