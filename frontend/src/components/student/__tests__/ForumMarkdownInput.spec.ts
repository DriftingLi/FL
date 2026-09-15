// 论坛正文输入框（#1017 / ADR-0052）：守三件事——
// ① 纯文本档不出现 tab / 工具栏 / 提示行；② Markdown 档三样都在，且工具能改正文；
// ③ 预览走发布端同一个渲染单点（ForumContent），不是第二套渲染。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, ref, nextTick } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'
import ForumMarkdownInput from '../ForumMarkdownInput.vue'
import ForumContent from '../ForumContent.vue'
import ForumImageUploader from '../ForumImageUploader.vue'
import MarkdownToolbar from '@/components/markdown/MarkdownToolbar.vue'
import UiUnderlineTabs from '@/components/ui/UiUnderlineTabs.vue'
import { ElMessage } from 'element-plus'
import { FORUM_MARKDOWN_HINT } from '@/utils/forumDisplay'
import { MARKDOWN_TOOLBAR_ITEMS } from '@/utils/markdownToolbar'

vi.mock('markstream-vue/index.css', () => ({}))

const uploadMocks = vi.hoisted(() => ({
  /** 让用例能把「上传中」这一态喂给组件（B6） */
  uploading: false,
  handlePaste: vi.fn(),
  handleDragOver: vi.fn(),
  handleDragLeave: vi.fn(),
  handleDrop: vi.fn(),
  uploadFiles: vi.fn()
}))
vi.mock('@/composables/useForumImageUpload', () => ({
  useForumImageUpload: () => ({
    uploading: { value: uploadMocks.uploading },
    dragging: { value: false },
    uploadFiles: uploadMocks.uploadFiles,
    removeImage: vi.fn(),
    handlePaste: uploadMocks.handlePaste,
    handleDragOver: uploadMocks.handleDragOver,
    handleDragLeave: uploadMocks.handleDragLeave,
    handleDrop: uploadMocks.handleDrop
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

  it('窄屏不撑破卡片：tab 不许缩，工具栏允许缩到容器宽后横向滚动', () => {
    const w = mountInput({ format: 'markdown' })
    expect(w.findComponent(UiUnderlineTabs).classes()).toContain('shrink-0')
    const toolbar = w.findComponent(MarkdownToolbar)
    expect(toolbar.classes()).toContain('min-w-0')
    expect(toolbar.classes()).toContain('overflow-x-auto')
    expect(toolbar.classes()).not.toContain('shrink-0')
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

  it('按真实选区插入：选中 bc 点加粗 → a**bc**d，选区留在 bc 上', async () => {
    // 用 v-model 宿主挂载（贴近真实用法）：不绑 v-model 的话正文不会回写，
    // 选区端点会被旧值长度夹住，测不到真实行为。
    const Host = defineComponent({
      components: { ForumMarkdownInput },
      setup() {
        return { text: ref('abcd'), imgs: ref<string[]>([]) }
      },
      template: '<ForumMarkdownInput v-model="text" v-model:images="imgs" format="markdown" />'
    })
    const host = mount(Host, { global: { plugins: [epLite()] } })
    const child = host.findComponent(ForumMarkdownInput)
    const textarea = child.find('textarea').element as HTMLTextAreaElement
    // 这一步同时验证了 UiInput 暴露的 getTextarea() 真拿到了原生元素
    textarea.setSelectionRange(1, 3)
    await child.findComponent(MarkdownToolbar).findAll('button')[1].trigger('click')
    await flushPromises()
    await nextTick()
    expect(host.vm.text).toBe('a**bc**d')
    // 包上标记后仍选中原来的 bc（作者可以接着改）
    expect([textarea.selectionStart, textarea.selectionEnd]).toEqual([3, 5])
  })

  it('原生插入路径（execCommand）：保住选区，EP 在 nextTick 里的光标复位不会赢', async () => {
    // happy-dom 没有 execCommand 实现，这里注入一个「真的会插入并派发 input」的替身，
    // 从而跑通 insertNatively 的成功分支（回退分支由「按真实选区插入」那条用例覆盖）。
    const doc = document as unknown as { execCommand?: unknown }
    const original = doc.execCommand
    // VTU 默认把组件挂在**游离**容器里，document.querySelector 找不到那个 textarea，
    // 所以由测试显式把目标元素交给替身（等价于浏览器里「焦点所在的 textarea」）。
    let target: HTMLTextAreaElement | null = null
    doc.execCommand = (_command: string, _ui: boolean, text: string) => {
      const active = target
      if (!active) return false
      const start = active.selectionStart
      const end = active.selectionEnd
      active.value = active.value.slice(0, start) + text + active.value.slice(end)
      const caret = start + text.length
      active.setSelectionRange(caret, caret)
      active.dispatchEvent(new Event('input', { bubbles: true }))
      // 如实模拟 Element Plus 的 useCursor：它在 input 后的**自己的 nextTick** 里
      // 按「插入段末尾」复位光标（setSelectionRange(pos, pos) → 选区被塌陷）。
      // 组件若不在更晚的一次 tick 里补回选区，这里就会把选区吃掉（评审 A1）。
      void nextTick(() => {
        active.setSelectionRange(caret, caret)
      })
      return true
    }

    try {
      const Host = defineComponent({
        components: { ForumMarkdownInput },
        setup() {
          return { text: ref('abcd'), imgs: ref<string[]>([]) }
        },
        template: '<ForumMarkdownInput v-model="text" v-model:images="imgs" format="markdown" />'
      })
      const host = mount(Host, { global: { plugins: [epLite()] } })
      const child = host.findComponent(ForumMarkdownInput)
      const textarea = child.find('textarea').element as HTMLTextAreaElement
      target = textarea
      textarea.setSelectionRange(1, 3)
      await child.findComponent(MarkdownToolbar).findAll('button')[1].trigger('click')
      await flushPromises()
      await nextTick()
      await nextTick()
      expect(host.vm.text).toBe('a**bc**d')
      // EP 的 useCursor 会在自己的 nextTick 里把选区塌陷成 caret：组件补的那次必须赢
      expect([textarea.selectionStart, textarea.selectionEnd]).toEqual([3, 5])
    } finally {
      doc.execCommand = original
    }
  })

  it('B6 上传中：虚线区进入上传中态（点击不再弹选择器）', () => {
    uploadMocks.uploading = true
    try {
      const w = mountInput({ format: 'markdown' })
      const zone = w.findComponent(ForumImageUploader).find('button')
      expect(zone.classes()).toContain('cursor-wait')
      expect(w.text()).toContain('上传中…')
    } finally {
      uploadMocks.uploading = false
    }
  })

  it('B9 已在字数上限：工具栏插入被挡住并提示，不改正文', async () => {
    const warn = vi.spyOn(ElMessage, 'warning').mockReturnValue(undefined as never)
    try {
      const w = mountInput({ format: 'markdown', modelValue: 'abcd', maxlength: 4 })
      await w.findComponent(MarkdownToolbar).findAll('button')[1].trigger('click')
      await flushPromises()
      expect(w.emitted('update:modelValue')).toBeFalsy()
      expect(warn).toHaveBeenCalled()
      expect(w.find('.forum-md-input-count').text()).toBe('4/4')
    } finally {
      warn.mockRestore()
    }
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

  it('卡片上任意位置拖拽都转发给上传单点（拖到正文上也不许浏览器导航走）', async () => {
    const w = mountInput({ format: 'markdown' })
    const card = w.find('.forum-md-input')
    await card.trigger('dragover')
    await card.trigger('dragleave')
    await card.trigger('drop')
    expect(uploadMocks.handleDragOver).toHaveBeenCalledTimes(1)
    expect(uploadMocks.handleDragLeave).toHaveBeenCalledTimes(1)
    expect(uploadMocks.handleDrop).toHaveBeenCalledTimes(1)
    // 正文也在转发面内（这是评审 A2 的要点：拖到正文上不该让浏览器导航走）
    await w.find('textarea').trigger('dragover')
    expect(uploadMocks.handleDragOver).toHaveBeenCalledTimes(2)
  })

  it('textarea 的 keydown 透传给调用方（Ctrl/Cmd+Enter 提交由上层决定）', async () => {
    const w = mountInput({ format: 'markdown' })
    await w.find('textarea').trigger('keydown', { key: 'Enter', ctrlKey: true })
    expect(w.emitted('keydown')).toBeTruthy()
  })
})
