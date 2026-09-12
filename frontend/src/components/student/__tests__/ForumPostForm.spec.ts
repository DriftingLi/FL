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
import UiSegmentTabs from '@/components/ui/UiSegmentTabs.vue'
import UiInput from '@/components/ui/UiInput.vue'
import ForumPostForm from '../ForumPostForm.vue'

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn(), post: vi.fn(), delete: vi.fn(), put: vi.fn() }
}))

import { unwrappedRequest } from '@/api/request'

const mockPost = vi.mocked(unwrappedRequest.post)

type FormProps = InstanceType<typeof ForumPostForm>['$props']

function mountForm(props: FormProps) {
  return mount(ForumPostForm, { props })
}

const chipLabels = (chips: { findAll: (s: string) => Array<{ text: () => string }> }) =>
  chips.findAll('[role="tab"]').map((b) => b.text().trim())

// mockPost 是模块级共享 mock：逐例清调用记录，否则提交类用例互相污染计数
beforeEach(() => {
  vi.clearAllMocks()
})

describe('发帖表单类别 chips（#742 / ADR-0040）', () => {
  it('不传 categories：不渲染 chips（存量调用零 diff）', () => {
    const wrapper = mountForm({ category: 'discussion' })
    expect(wrapper.findComponent(UiSegmentTabs).exists()).toBe(false)
  })

  it('章节帖 chapter_id 通道：传了 categories 也不渲染 chips（两通道互斥）', () => {
    const wrapper = mountForm({
      category: 'discussion',
      chapterId: 77,
      categories: ['discussion', 'question']
    })
    expect(wrapper.findComponent(UiSegmentTabs).exists()).toBe(false)
  })

  it('传入 categories：渲染两分类 chips（讨论/问答），默认选中 = category prop', () => {
    const wrapper = mountForm({
      category: 'question',
      categories: ['discussion', 'question']
    })
    const chips = wrapper.findComponent(UiSegmentTabs)
    expect(chips.exists()).toBe(true)
    expect(chips.props('modelValue')).toBe('question')
    expect(chipLabels(chips)).toEqual(['讨论', '问答'])
  })

  it('历史 experience 不再可用：category prop 归一为 discussion，chips 选项里也不出现', () => {
    const wrapper = mountForm({
      category: 'experience',
      categories: ['discussion', 'question', 'experience']
    })
    const chips = wrapper.findComponent(UiSegmentTabs)
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
