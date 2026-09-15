// 论坛图片上传单点（#389 / #1014）：拖拽与粘贴两个入口的漏斗口径。
// 这里只测状态机——「虚线区长什么样」由组件层负责。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import { forumApi } from '@/api/forum'
import { useForumImageUpload } from '../useForumImageUpload'

vi.mock('element-plus', () => ({
  ElMessage: { warning: vi.fn(), error: vi.fn() }
}))
vi.mock('@/api/forum', () => ({
  forumApi: { uploadImage: vi.fn() }
}))

const mockUpload = vi.mocked(forumApi.uploadImage)

function imageFile(name = 'a.png', size = 1024) {
  const file = new File(['x'], name, { type: 'image/png' })
  Object.defineProperty(file, 'size', { value: size })
  return file
}

/** 最小拖拽事件替身：只带 dataTransfer.files 与 preventDefault 记录 */
function dragEvent(files: File[]) {
  const state = { prevented: false, dropEffect: '' }
  const event = {
    dataTransfer: {
      files,
      get dropEffect() {
        return state.dropEffect
      },
      set dropEffect(v: string) {
        state.dropEffect = v
      }
    },
    defaultPrevented: () => state.prevented,
    preventDefault() {
      state.prevented = true
    }
  }
  return event as unknown as DragEvent & { defaultPrevented: () => boolean; dataTransfer: { dropEffect: string } }
}

function pasteEvent(files: Array<File | null>) {
  const state = { prevented: false }
  const items = files.map(file => ({
    kind: 'file',
    type: file ? file.type : 'text/plain',
    getAsFile: () => file
  }))
  const event = {
    clipboardData: { items },
    defaultPrevented: () => state.prevented,
    preventDefault() {
      state.prevented = true
    }
  }
  return event as unknown as ClipboardEvent & { defaultPrevented: () => boolean }
}

describe('useForumImageUpload 拖拽入口', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUpload.mockResolvedValue({ url: '/static/uploads/forum/a.png' } as never)
  })

  it('放下图片：接管默认行为（否则浏览器会导航去打开它）并上传', async () => {
    const upload = useForumImageUpload(3)
    const event = dragEvent([imageFile()])
    upload.handleDrop(event)
    await nextTick()
    expect(event.defaultPrevented()).toBe(true)
    expect(mockUpload).toHaveBeenCalledTimes(1)
    expect(upload.urls.value).toEqual(['/static/uploads/forum/a.png'])
  })

  it('拖入非图片：仍然接管默认行为，但不发上传请求', async () => {
    const upload = useForumImageUpload(3)
    const pdf = new File(['x'], 'a.pdf', { type: 'application/pdf' })
    const event = dragEvent([pdf])
    upload.handleDrop(event)
    await nextTick()
    expect(event.defaultPrevented()).toBe(true)
    expect(mockUpload).not.toHaveBeenCalled()
    expect(upload.urls.value).toEqual([])
  })

  it('已达上限：不高亮、也不上传', async () => {
    const upload = useForumImageUpload(1, { urls: undefined })
    upload.urls.value = ['/a.png']
    const over = dragEvent([imageFile('b.png')])
    upload.handleDragOver(over)
    expect(upload.dragging.value).toBe(false)
    upload.handleDrop(over)
    await nextTick()
    expect(mockUpload).not.toHaveBeenCalled()
  })

  it('dragenter/leave 维护高亮，放下后复位', () => {
    const upload = useForumImageUpload(3)
    upload.handleDragOver(dragEvent([imageFile()]))
    expect(upload.dragging.value).toBe(true)
    upload.handleDragLeave()
    expect(upload.dragging.value).toBe(false)
    upload.handleDragOver(dragEvent([imageFile()]))
    upload.handleDrop(dragEvent([imageFile()]))
    expect(upload.dragging.value).toBe(false)
  })

  it('超 20MB 的图片跳过并提示，其余继续', async () => {
    const upload = useForumImageUpload(3)
    const big = imageFile('big.png', 21 * 1024 * 1024)
    upload.handleDrop(dragEvent([big]))
    await nextTick()
    expect(mockUpload).not.toHaveBeenCalled()
    expect(vi.mocked(ElMessage.error)).toHaveBeenCalled()
  })
})

describe('useForumImageUpload 粘贴入口（行为不变）', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUpload.mockResolvedValue({ url: '/static/uploads/forum/a.png' } as never)
  })

  it('剪贴板含图片：拦截默认粘贴并上传', async () => {
    const upload = useForumImageUpload(3)
    const event = pasteEvent([imageFile()])
    upload.handlePaste(event)
    await nextTick()
    expect(event.defaultPrevented()).toBe(true)
    expect(mockUpload).toHaveBeenCalledTimes(1)
  })

  it('剪贴板无图片：不拦截，留原生粘贴', () => {
    const upload = useForumImageUpload(3)
    const event = pasteEvent([null])
    upload.handlePaste(event)
    expect(event.defaultPrevented()).toBe(false)
    expect(mockUpload).not.toHaveBeenCalled()
  })
})
