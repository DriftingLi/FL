// ForumPostForm 类别 chips 契约（#742 批次四，ADR-0040 收窄为两意图）。
//
// 守四个边界：
//  1. 不传 categories：不渲染 chips（存量调用零 diff——ChapterDiscussion 等共用方不受影响）；
//  2. 章节帖 chapter_id 通道：即便传了 categories 也不渲染（chapter_id 与 category 互斥）；
//  3. 发布侧只认 discussion/question：历史 'experience' 一律归 discussion，既不渲染成选项
//     也不外泄到 createTopic（后端对该值直接 400）；
//  4. 联动只在「用户主动切换且偏离入口类别」时生效，切回入口类别即恢复壳定制 placeholder。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'
import UiSegmentTabs from '@/components/ui/UiSegmentTabs.vue'
import UiInput from '@/components/ui/UiInput.vue'
import ForumContent from '../ForumContent.vue'
import ForumPostForm from '../ForumPostForm.vue'

// markstream 的 CSS 导入在 vitest 下无意义，替身掉
vi.mock('markstream-vue/index.css', () => ({}))

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn(), post: vi.fn(), delete: vi.fn(), put: vi.fn() }
}))

import { unwrappedRequest } from '@/api/request'

const mockPost = vi.mocked(unwrappedRequest.post)

type FormProps = InstanceType<typeof ForumPostForm>['$props']

function mountForm(props: FormProps) {
  // 按仓库约定装 epLite()：不装的话 el-button 等 EP 组件不会被解析，
  // 渲染成同名自定义元素（断言 DOM 按钮就会扑空，且没有任何报错）。
  return mount(ForumPostForm, { props, global: { plugins: [epLite()] } })
}

const chipLabels = (chips: { findAll: (s: string) => Array<{ text: () => string }> }) =>
  chips.findAll('[role="tab"]').map((b) => b.text().trim())

// mockPost 是模块级共享 mock：逐例清调用记录，否则提交类用例互相污染计数
beforeEach(() => {
  vi.clearAllMocks()
})

describe('发帖表单类别 chips（#742 / ADR-0040）', () => {
  // 注意：本组件自 #878 起**常驻**一个「正文格式」分段控件（纯文本 | Markdown），
  // 所以断言必须收窄到「类别 chips」本身，不能再说「页面上没有 UiSegmentTabs」。
  /** 按 options 里含 discussion 定位类别 chips（避开正文格式控件） */
  function categoryTabs(w: ReturnType<typeof mountForm>) {
    return w
      .findAllComponents(UiSegmentTabs)
      .find((c) => (c.props('options') as Array<{ value: string }>).some((o) => o.value === 'discussion'))
  }

  it('不传 categories：不渲染类别 chips（存量调用零 diff）', () => {
    const wrapper = mountForm({ category: 'discussion' })
    expect(categoryTabs(wrapper)).toBeUndefined()
  })

  it('章节帖 chapter_id 通道：传了 categories 也不渲染类别 chips（两通道互斥）', () => {
    const wrapper = mountForm({
      category: 'discussion',
      chapterId: 77,
      categories: ['discussion', 'question']
    })
    expect(categoryTabs(wrapper)).toBeUndefined()
  })

  it('传入 categories：渲染两分类 chips（讨论/问答），默认选中 = category prop', () => {
    const wrapper = mountForm({
      category: 'question',
      categories: ['discussion', 'question']
    })
    const chips = categoryTabs(wrapper)!
    expect(chips.props('modelValue')).toBe('question')
    expect(chipLabels(chips)).toEqual(['讨论', '问答'])
  })

  it('历史 experience 不再可用：category prop 归一为 discussion，chips 选项里也不出现', () => {
    const wrapper = mountForm({
      category: 'experience',
      categories: ['discussion', 'question', 'experience']
    })
    const chips = categoryTabs(wrapper)!
    expect(chips.props('modelValue')).toBe('discussion')
    expect(chipLabels(chips)).toEqual(['讨论', '问答'])
    expect((chips.props('options') as Array<{ value: string }>).map((o) => o.value)).toEqual([
      'discussion',
      'question'
    ])
  })

  it('联动恢复：切走再切回入口类别，placeholder 恢复壳定制文案', async () => {
    const wrapper = mountForm({
      category: 'question',
      categories: ['discussion', 'question'],
      contentPlaceholder: '详细描述你的问题、已尝试的方法（定制文案）'
    })
    const chips = wrapper.findComponent(UiSegmentTabs)
    const contentInput = () => wrapper.findAllComponents(UiInput)[1]

    // 切到讨论：placeholder 联动为讨论通用文案
    chips.vm.$emit('update:modelValue', 'discussion')
    await flushPromises()
    expect(contentInput().props('placeholder')).toContain('请输入内容')

    // 切回入口类别（问答）：恢复壳定制文案
    chips.vm.$emit('update:modelValue', 'question')
    await flushPromises()
    expect(contentInput().props('placeholder')).toContain('定制文案')
  })

  it('提交携带当前选中类别（chips 切换后 createTopic 跟随）', async () => {
    mockPost.mockResolvedValue({} as never)
    const wrapper = mountForm({
      category: 'discussion',
      categories: ['discussion', 'question']
    })
    const chips = wrapper.findComponent(UiSegmentTabs)
    await wrapper.findAllComponents(UiInput)[0].setValue('标题')
    await wrapper.findAllComponents(UiInput)[1].setValue('内容')
    chips.vm.$emit('update:modelValue', 'question')
    await flushPromises()
    await (wrapper.vm as unknown as { submit: () => Promise<boolean> }).submit()
    await flushPromises()
    expect(mockPost).toHaveBeenCalledTimes(1)
    expect(mockPost.mock.calls[0][0]).toBe('/forum/topics')
    expect((mockPost.mock.calls[0][1] as { category: string }).category).toBe('question')
  })

// ===== 正文格式声明（#878 / ADR-0044）=====
//
// 守四件事：① 首次默认纯文本；② 切换改变提交载荷；③ 记住上次选择（本地存储）；
// ④ 预览复用**同一个** UGC 渲染单点（预览与发布同源，不能是第二套渲染）。
describe('发帖表单正文格式（#878 / ADR-0044）', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
  })

  /** 按 options 里含 markdown 定位格式分段控件（避开类别 chips） */
  function formatTabs(w: ReturnType<typeof mountForm>) {
    return w
      .findAllComponents(UiSegmentTabs)
      .find((c) => (c.props('options') as Array<{ value: string }>).some((o) => o.value === 'markdown'))
  }

  function previewButton(w: ReturnType<typeof mountForm>) {
    return w.findAll('button').find((b) => b.text().includes('预览'))
  }

  it('渲染格式切换，首次默认纯文本', () => {
    const w = mountForm({ category: 'discussion' })
    const tabs = formatTabs(w)
    expect(tabs).toBeTruthy()
    expect(tabs!.props('modelValue')).toBe('text')
    // 纯文本下不应出现预览开关（没有可预览的东西）
    expect(previewButton(w)).toBeUndefined()
  })

  it('默认提交携带 content_format=text', async () => {
    mockPost.mockResolvedValue({} as never)
    const w = mountForm({ category: 'discussion' })
    await w.findAllComponents(UiInput)[0].setValue('标题')
    await w.findAllComponents(UiInput)[1].setValue('内容')
    await (w.vm as unknown as { submit: () => Promise<boolean> }).submit()
    await flushPromises()
    expect((mockPost.mock.calls[0][1] as { content_format: string }).content_format).toBe('text')
  })

  it('切到 Markdown 后提交携带 content_format=markdown，且正文原样提交', async () => {
    mockPost.mockResolvedValue({} as never)
    const w = mountForm({ category: 'discussion' })
    await w.findAllComponents(UiInput)[0].setValue('标题')
    await w.findAllComponents(UiInput)[1].setValue('## 内容')
    formatTabs(w)!.vm.$emit('update:modelValue', 'markdown')
    await flushPromises()
    await (w.vm as unknown as { submit: () => Promise<boolean> }).submit()
    await flushPromises()
    expect((mockPost.mock.calls[0][1] as { content_format: string }).content_format).toBe('markdown')
    // 前端不做任何转义或改写——存源串，渲染在展示层
    expect((mockPost.mock.calls[0][1] as { content: string }).content).toBe('## 内容')
  })

  it('记住上次选择：切到 Markdown 后重新挂载仍是 Markdown', async () => {
    const w1 = mountForm({ category: 'discussion' })
    formatTabs(w1)!.vm.$emit('update:modelValue', 'markdown')
    await flushPromises()

    const w2 = mountForm({ category: 'discussion' })
    expect(formatTabs(w2)!.props('modelValue')).toBe('markdown')
  })

  it('切回纯文本同样被记住', async () => {
    const w1 = mountForm({ category: 'discussion' })
    formatTabs(w1)!.vm.$emit('update:modelValue', 'markdown')
    await flushPromises()
    formatTabs(w1)!.vm.$emit('update:modelValue', 'text')
    await flushPromises()

    const w2 = mountForm({ category: 'discussion' })
    expect(formatTabs(w2)!.props('modelValue')).toBe('text')
  })

  it('选 Markdown 才出现预览；预览复用 UGC 渲染单点且内容一致', async () => {
    const w = mountForm({ category: 'discussion' })
    await w.findAllComponents(UiInput)[1].setValue('## 排查步骤')
    formatTabs(w)!.vm.$emit('update:modelValue', 'markdown')
    await flushPromises()

    const btn = previewButton(w)
    expect(btn).toBeTruthy()
    // 预览前：渲染单点不出现（还是编辑态）
    expect(w.findComponent(ForumContent).exists()).toBe(false)

    await btn!.trigger('click')
    await flushPromises()
    const fc = w.findComponent(ForumContent)
    expect(fc.exists()).toBe(true)
    expect(fc.props('format')).toBe('markdown')
    expect(fc.props('content')).toContain('## 排查步骤')
  })
});


  it('历史 experience 入口提交：归一为 discussion，不会产出后端 400 的类别', async () => {
    mockPost.mockResolvedValue({} as never)
    const wrapper = mountForm({
      category: 'experience',
      categories: ['discussion', 'question', 'experience']
    })
    await wrapper.findAllComponents(UiInput)[0].setValue('标题')
    await wrapper.findAllComponents(UiInput)[1].setValue('内容')
    await (wrapper.vm as unknown as { submit: () => Promise<boolean> }).submit()
    await flushPromises()
    expect(mockPost).toHaveBeenCalledTimes(1)
    expect((mockPost.mock.calls[0][1] as { category: string }).category).toBe('discussion')
  })
})
