// ForumPostForm 类别 chips 契约（#742 批次四）。
//
// 守三个零 diff / 联动边界：
//  1. 不传 categories：不渲染 chips（存量调用零 diff——ChapterDiscussion 等共用方不受影响）；
//  2. 章节帖 chapter_id 通道：即便传了 categories 也不渲染（chapter_id 与 category 互斥）；
//  3. 联动只在「用户主动切换且偏离入口类别」时生效，切回入口类别即恢复壳定制 placeholder。
import { describe, it, expect, vi } from 'vitest'
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

describe('发帖表单类别 chips（#742）', () => {
  it('不传 categories：不渲染 chips（存量调用零 diff）', () => {
    const wrapper = mountForm({ category: 'discussion' })
    expect(wrapper.findComponent(UiSegmentTabs).exists()).toBe(false)
  })

  it('章节帖 chapter_id 通道：传了 categories 也不渲染 chips（两通道互斥）', () => {
    const wrapper = mountForm({
      category: 'discussion',
      chapterId: 77,
      categories: ['discussion', 'question', 'experience']
    })
    expect(wrapper.findComponent(UiSegmentTabs).exists()).toBe(false)
  })

  it('传入 categories：渲染三分类 chips，默认选中 = category prop', () => {
    const wrapper = mountForm({
      category: 'experience',
      categories: ['discussion', 'question', 'experience']
    })
    const chips = wrapper.findComponent(UiSegmentTabs)
    expect(chips.exists()).toBe(true)
    expect(chips.props('modelValue')).toBe('experience')
  })

  it('联动恢复：切走再切回入口类别，placeholder 恢复壳定制文案', async () => {
    const wrapper = mountForm({
      category: 'question',
      categories: ['discussion', 'question', 'experience'],
      contentPlaceholder: '详细描述你的问题、已尝试的方法（定制文案）'
    })
    const chips = wrapper.findComponent(UiSegmentTabs)
    const contentInput = () => wrapper.findAllComponents(UiInput)[1]

    // 切到经验帖：placeholder 联动为经验帖文案
    chips.vm.$emit('update:modelValue', 'experience')
    await flushPromises()
    expect(contentInput().props('placeholder')).toContain('考试批次/科目')

    // 切回入口类别（问答）：恢复壳定制文案
    chips.vm.$emit('update:modelValue', 'question')
    await flushPromises()
    expect(contentInput().props('placeholder')).toContain('定制文案')
  })

  it('提交携带当前选中类别（chips 切换后 createTopic 跟随）', async () => {
    mockPost.mockResolvedValue({} as never)
    const wrapper = mountForm({
      category: 'discussion',
      categories: ['discussion', 'question', 'experience']
    })
    const chips = wrapper.findComponent(UiSegmentTabs)
    await wrapper.findAllComponents(UiInput)[0].setValue('标题')
    await wrapper.findAllComponents(UiInput)[1].setValue('内容')
    chips.vm.$emit('update:modelValue', 'experience')
    await flushPromises()
    await (wrapper.vm as unknown as { submit: () => Promise<boolean> }).submit()
    await flushPromises()
    expect(mockPost).toHaveBeenCalledTimes(1)
    expect(mockPost.mock.calls[0][0]).toBe('/forum/topics')
    expect((mockPost.mock.calls[0][1] as { category: string }).category).toBe('experience')
  })
})
