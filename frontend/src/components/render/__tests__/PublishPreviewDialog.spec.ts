// 发布端预览弹窗的两种正文形态（#2631）。
//
// 守一件事：管理端在「导入 Word」形态下点「发布端预览」时，必须看到那个 .docx，
// 而不是一片空白（此前的实现只挂了 Markdown 渲染，docx 档位下点开是空的）。
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

vi.mock('docx-preview', () => ({ renderAsync: vi.fn().mockResolvedValue(undefined) }))

import PublishPreviewDialog from '../PublishPreviewDialog.vue'
import DocxDocument from '../DocxDocument.vue'
import PublishMarkdown from '../PublishMarkdown.vue'

const DOCX_URL = '/static/uploads/featured/docx/plan_1700000000000.docx'

let fetchMock: ReturnType<typeof vi.fn>

beforeEach(() => {
  fetchMock = vi.fn().mockResolvedValue({ ok: true, blob: () => Promise.resolve(new Blob(['PK'])) })
  vi.stubGlobal('fetch', fetchMock)
})

afterEach(() => {
  vi.unstubAllGlobals()
})

function mountDialog(props: Record<string, unknown>) {
  return mount(PublishPreviewDialog, {
    props: { modelValue: true, ...props },
    global: { plugins: [epLite()] },
    attachTo: document.body
  })
}

describe('PublishPreviewDialog 的正文形态', () => {
  it('没有 docxUrl 时走 Markdown 渲染（原行为不变）', async () => {
    const wrapper = mountDialog({ content: '# 标题', subset: 'featured' })
    // el-dialog 走 teleport/transition，内容要等一帧才进 DOM
    await flushPromises()
    await new Promise((r) => setTimeout(r, 0))

    expect(wrapper.findComponent(PublishMarkdown).exists()).toBe(true)
    expect(wrapper.findComponent(DocxDocument).exists()).toBe(false)
    wrapper.unmount()
  })

  it('给了 docxUrl 时改渲染 Word，且不再挂 Markdown', async () => {
    const wrapper = mountDialog({ content: '', docxUrl: DOCX_URL })
    await flushPromises()
    await new Promise((r) => setTimeout(r, 0))

    expect(wrapper.findComponent(DocxDocument).exists()).toBe(true)
    expect(wrapper.findComponent(PublishMarkdown).exists()).toBe(false)
    // 真的去取了那个文件 —— 否则弹窗里仍是空白
    expect(fetchMock).toHaveBeenCalledWith(DOCX_URL)
    wrapper.unmount()
  })
})