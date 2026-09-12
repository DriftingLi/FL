// 内容精选「三端交集子集」提示与发布端预览（ADR-0046 / #902 #903）。
//
// seam 同 ChapterDiscussion.spec：mount 真实页面，只 mock 网络层与编辑器（Vditor 替身）。
// 守的是：作者在编辑器里就被明确告知「门户与移动端不渲染表格/公式」，且提示**不阻断保存**；
// 预览里看到的是交集渲染结果，不是一张读者看不到的表。
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { epLite } from '@/test/element-lite'

vi.mock('@/api/featured', () => ({
  adminFeaturedApi: {
    getDetail: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    uploadImage: vi.fn()
  },
  featuredCategoryOptions: [{ label: '行业资讯', value: 'news' }]
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: {} }),
  useRouter: () => ({ push: vi.fn() })
}))

import FeaturedContentEdit from '../FeaturedContentEdit.vue'

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

const TABLE = ['| 参数 | 值 |', '| --- | --- |', '| 扭矩 | 100 |'].join('\n')
const PLAIN = ['# 标题', '', '- 列表项', '', '普通段落文字'].join('\n')

function mountPage() {
  return mount(FeaturedContentEdit, {
    global: { plugins: [epLite()], stubs: { MarkdownEditor: MarkdownEditorStub } }
  })
}

let wrapper: ReturnType<typeof mountPage> | null = null

beforeEach(() => {
  wrapper = null
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
})

describe('FeaturedContentEdit 三端交集子集（#902）', () => {
  it('正文含表格时给出可见提示（点名门户与移动端不渲染）', async () => {
    wrapper = mountPage()
    await flushPromises()
    expect(wrapper.find('.el-alert').exists()).toBe(false)

    await wrapper.find('.md-editor-stub').setValue(TABLE)
    await flushPromises()

    const alert = wrapper.find('.el-alert')
    expect(alert.exists()).toBe(true)
    expect(alert.text()).toContain('门户与移动端不渲染')
    expect(alert.text()).toContain('表格')
  })

  it('正文含公式时同样给出提示', async () => {
    wrapper = mountPage()
    await flushPromises()
    await wrapper.find('.md-editor-stub').setValue('扭矩 $T = 9550P/n$ 时按此取值')
    await flushPromises()
    expect(wrapper.find('.el-alert').text()).toContain('公式')
  })

  it('子集内语法不触发提示', async () => {
    wrapper = mountPage()
    await flushPromises()
    await wrapper.find('.md-editor-stub').setValue(PLAIN)
    await flushPromises()
    expect(wrapper.find('.el-alert').exists()).toBe(false)
  })

  it('提示不阻断保存：保存按钮仍可点击并发出请求', async () => {
    const { adminFeaturedApi } = await import('@/api/featured')
    vi.mocked(adminFeaturedApi.create).mockResolvedValue({} as never)
    wrapper = mountPage()
    await flushPromises()

    await wrapper.find('.md-editor-stub').setValue(TABLE)
    await wrapper.find('input').setValue('标题')
    await flushPromises()

    const saveButton = wrapper.findAll('button').find((b) => b.text().includes('保存草稿'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')
    await flushPromises()
    expect(adminFeaturedApi.create).toHaveBeenCalled()
  })
})

describe('FeaturedContentEdit 发布端预览（#903）', () => {
  it('预览按交集渲染：没有表格、公式保持源码，并说明越界语法', async () => {
    wrapper = mountPage()
    await flushPromises()
    await wrapper.find('.md-editor-stub').setValue(`${TABLE}\n\n扭矩 $T = 9550P/n$ 时按此取值`)
    await flushPromises()

    const previewButton = wrapper.findAll('button').find((b) => b.text().includes('发布端预览'))
    expect(previewButton).toBeTruthy()
    await previewButton!.trigger('click')
    await flushPromises()

    const dialog = wrapper.find('.el-dialog')
    expect(dialog.exists()).toBe(true)
    expect(dialog.find('table').exists()).toBe(false)
    expect(dialog.find('.katex').exists()).toBe(false)
    expect(dialog.text()).toContain('| 参数 | 值 |')
    expect(dialog.text()).toContain('$T = 9550P/n$')
    expect(dialog.text()).toContain('门户与移动端不渲染')
  })
})
