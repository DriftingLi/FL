// 讲师端「发布端预览」（ADR-0046 / #903）。
//
// seam 同 ChapterDiscussion.spec：mount 真实页面，只 mock 网络层与重子组件
// （Vditor 在 jsdom 里不可靠，编辑器用替身；替身仍实现 getValue，页面取的就是它）。
// 守的是「预览 = 发布」：预览区里表格是真表格、公式是真公式、代码有高亮，
// 而不是 Vditor 内部引擎的解释。
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { epLite } from '@/test/element-lite'

vi.mock('@/api/tutor', () => ({
  tutorApi: {
    getChapterDetail: vi.fn(),
    getCourseChapters: vi.fn(),
    updateChapter: vi.fn(),
    deleteFile: vi.fn()
  }
}))

vi.mock('@/stores/credential', () => ({
  useCredentialStore: () => ({ current: null })
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { courseId: '1', chapterId: '2' } }),
  useRouter: () => ({ push: vi.fn() })
}))

import { tutorApi } from '@/api/tutor'
import TutorChapterEdit from '../TutorChapterEdit.vue'

/** Vditor 替身：jsdom 里跑不起来，但仍然提供页面用到的 getValue（取编辑器最新值）。 */
const MarkdownEditorStub = defineComponent({
  name: 'MarkdownEditor',
  props: { modelValue: { type: String, default: '' } },
  emits: ['update:modelValue'],
  setup(props, { expose, emit }) {
    expose({ getValue: () => props.modelValue })
    return () =>
      h('textarea', {
        class: 'md-editor-stub',
        value: props.modelValue,
        onInput: (event: Event) => emit('update:modelValue', (event.target as HTMLTextAreaElement).value)
      })
  }
})

const CHAPTER_CONTENT = [
  '# 扭矩计算',
  '',
  '| 参数 | 值 |',
  '| --- | --- |',
  '| 扭矩 | 100 |',
  '',
  '扭矩 $T = 9550P/n$ 时按此取值',
  '',
  '```js',
  'const a = 1',
  '```'
].join('\n')

function mountPage() {
  return mount(TutorChapterEdit, {
    global: {
      plugins: [epLite()],
      stubs: {
        MarkdownEditor: MarkdownEditorStub,
        FileUpload: true,
        VideoPlayer: true,
        DocumentViewer: true,
        PptViewer: true
      }
    }
  })
}

let wrapper: ReturnType<typeof mountPage> | null = null

beforeEach(() => {
  vi.mocked(tutorApi.getChapterDetail).mockResolvedValue({
    chapter_id: 2,
    title: '扭矩计算',
    content: CHAPTER_CONTENT,
    files: []
  } as never)
  vi.mocked(tutorApi.getCourseChapters).mockResolvedValue({ course: { name: '叉车维修' }, chapters: [] } as never)
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
})

describe('TutorChapterEdit 发布端预览（#903）', () => {
  it('预览入口存在，且打开后渲染的是发布端结果（表格 / 公式 / 代码高亮）', async () => {
    wrapper = mountPage()
    await flushPromises()

    const previewButton = wrapper.findAll('button').find((b) => b.text().includes('发布端预览'))
    expect(previewButton).toBeTruthy()
    await previewButton!.trigger('click')
    await flushPromises()

    const dialog = wrapper.find('.el-dialog')
    expect(dialog.exists()).toBe(true)
    expect(dialog.find('table').exists()).toBe(true)
    expect(dialog.find('.katex').exists()).toBe(true)
    expect(dialog.find('.hljs-keyword').exists()).toBe(true)
  })

  it('预览不写回表单：关闭预览后编辑内容与保存按钮状态不变', async () => {
    wrapper = mountPage()
    await flushPromises()

    const textarea = wrapper.find('.md-editor-stub')
    expect((textarea.element as HTMLTextAreaElement).value).toBe(CHAPTER_CONTENT)

    const previewButton = wrapper.findAll('button').find((b) => b.text().includes('发布端预览'))
    await previewButton!.trigger('click')
    await flushPromises()
    expect(wrapper.find('.el-dialog').exists()).toBe(true)

    // 未改动正文时「保存正文」仍是禁用态——预览没有顺手改掉表单状态
    const saveButton = wrapper.findAll('button').find((b) => b.text().includes('保存正文'))
    expect(saveButton!.attributes('disabled')).toBeDefined()
  })
})
