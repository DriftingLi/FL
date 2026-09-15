// 论坛正文输入框（#1014 / ADR-0052）：守三件事——
// ① 纯文本档不出现 tab / 工具栏 / 提示行；② Markdown 档三样都在，且工具能改正文；
// ③ 预览走发布端同一个渲染单点（ForumContent），不是第二套渲染。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'
import ForumMarkdownInput from '../ForumMarkdownInput.vue'
import ForumContent from '../ForumContent.vue'
import ForumImageUploader from '../ForumImageUploader.vue'
import MarkdownToolbar from '@/components/markdown/MarkdownToolbar.vue'
import UiUnderlineTabs from '@/components/ui/UiUnderlineTabs.vue'
import { FORUM_MARKDOWN_HINT } from '@/utils/forumDisplay'
import { MARKDOWN_TOOLBAR_ITEMS } from '@/utils/markdownToolbar'

vi.mock('markstream-vue/index.css', () => ({}))

const uploadMocks = vi.hoisted(() => ({ handlePaste: vi.fn(), uploadFiles: vi.fn() }))
vi.mock('@/composables/useForumImageUpload', () => ({
  useForumImageUpload: () => ({
    uploading: false,
    dragging: { value: false },
    uploadFiles: uploadMocks.uploadFiles,
    removeImage: vi.fn(),
    handlePaste: uploadMocks.handlePaste,
    handleDragOver: vi.fn(),
    handleDragLeave: vi.fn(),
    handleDrop: vi.fn()
  })
}))

function mountInput(props: Record<string, unknown> = {}) {
  return mount(ForumMarkdownInput, {
    props: { modelValue: '正文', format: 'text', images: [], ...props },
    global: { plugins: [epLite()] }
  })
}

describe('纯文本档（ADR-0044：Markdown 是 opt-in）', () => {
  it('不出现 编写/预览 tab、工具栏与提示行', () => {
    const w = mountInput({ format: 'text' })
    expect(w.findAllComponents(UiUnderlineTabs)).toHaveLength(0)
    expect(w.findAllComponents(MarkdownToolbar)).toHaveLength(0)
    expect(w.find('.forum-markdown-hint').exists()).toBe(false)
  })

  it('粘贴区与字数计数仍在（图片与格式无关）', () => {
    const w = mountInput({ format: 'text', maxlength: 5000, modelValue: '' })
    expect(w.findComponent(ForumImageUploader).exists()).toBe(true)
    expect(w.find('.forum-md-input-count').text()).toBe('0/5000')
  })
})

describe('Markdown 档', () => {
  it('出现 编写/预览 tab 与整排工具栏', () => {
    const w = mountInput({ format: 'markdown' })
    const tabs = w.findComponent(UiUnderlineTabs)
    expect(tabs.exists()).toBe(true)
    expect(tabs.findAll('button').map((b) => b.text())).toEqual(['编写', '预览'])
    const toolbar = w.findComponent(MarkdownToolbar)
    expect(toolbar.findAll('button')).toHaveLength(MARKDOWN_TOOLBAR_ITEMS.length)
  })

  it('提示行给的是 forumDisplay 的单点文案（表格不渲染 / 图片走粘贴区）', () => {
    const w = mountInput({ format: 'markdown' })
    const hint = w.find('.forum-markdown-hint')
    expect(hint.exists()).toBe(true)
    expect(hint.text()).toBe(FORUM_MARKDOWN_HINT)
    expect(hint.text()).toContain('表格不渲染')
  })

  it('点工具栏按钮改写正文（回退路径：happy-dom 无 execCommand）', async () => {
    const w = mountInput({ format: 'markdown', modelValue: 'abcd' })
    const bold = w.findComponent(MarkdownToolbar).findAll('button')[1]
    await bold.trigger('click')
    await flushPromises()
    const emitted = w.emitted('update:modelValue')
    expect(emitted).toBeTruthy()
    // 无选区时插一对标记（选区信息来自真实 textarea，happy-dom 下光标在 0）
    expect(String(emitted![0]?.[0])).toContain('**')
  })

  it('切到预览：渲染 ForumContent（发布端单点）且工具栏置灰', async () => {
    const w = mountInput({ format: 'markdown', modelValue: '## 标题' })
    expect(w.findComponent(ForumContent).exists()).toBe(false)
    const previewTab = w.findComponent(UiUnderlineTabs).findAll('button')[1]
    await previewTab.trigger('click')
    await flushPromises()
    const fc = w.findComponent(ForumContent)
    expect(fc.exists()).toBe(true)
    expect(fc.props('format')).toBe('markdown')
    expect(fc.props('content')).toBe('## 标题')
    // 预览档下按钮置灰但不消失：仍可悬停看提示（aria-disabled 而非原生 disabled）
    const toolbarButtons = w.findComponent(MarkdownToolbar).findAll('button')
    expect(toolbarButtons).toHaveLength(MARKDOWN_TOOLBAR_ITEMS.length)
    expect(toolbarButtons[0].attributes('aria-disabled')).toBe('true')
  })

  it('置灰态点按钮不改正文（预览档不该动源串）', async () => {
    const w = mountInput({ format: 'markdown', modelValue: 'abcd' })
    await w.findComponent(UiUnderlineTabs).findAll('button')[1].trigger('click')
    await w.findComponent(MarkdownToolbar).findAll('button')[1].trigger('click')
    await flushPromises()
    expect(w.emitted('update:modelValue')).toBeFalsy()
  })

  it('resetPreview()：提交后回到编写态', async () => {
    const w = mountInput({ format: 'markdown' })
    await w.findComponent(UiUnderlineTabs).findAll('button')[1].trigger('click')
    await flushPromises()
    expect(w.findComponent(ForumContent).exists()).toBe(true)
    ;(w.vm as unknown as { resetPreview: () => void }).resetPreview()
    await flushPromises()
    expect(w.findComponent(ForumContent).exists()).toBe(false)
  })

  it('format 切走再切回，不停在预览态', async () => {
    const w = mountInput({ format: 'markdown' })
    await w.findComponent(UiUnderlineTabs).findAll('button')[1].trigger('click')
    await flushPromises()
    await w.setProps({ format: 'text' })
    await w.setProps({ format: 'markdown' })
    expect(w.findComponent(ForumContent).exists()).toBe(false)
    expect(w.findComponent(UiUnderlineTabs).findAll('button')[0].attributes('aria-selected')).toBe('true')
  })
})

describe('粘贴与键盘', () => {
  beforeEach(() => vi.clearAllMocks())

  it('卡片上的粘贴转发给上传单点（不再挂 document）', async () => {
    const w = mountInput({ format: 'markdown' })
    await w.find('.forum-md-input').trigger('paste')
    expect(uploadMocks.handlePaste).toHaveBeenCalledTimes(1)
  })

  it('textarea 的 keydown 透传给调用方（Ctrl/Cmd+Enter 提交由上层决定）', async () => {
    const w = mountInput({ format: 'markdown' })
    await w.find('textarea').trigger('keydown', { key: 'Enter', ctrlKey: true })
    expect(w.emitted('keydown')).toBeTruthy()
  })
})
