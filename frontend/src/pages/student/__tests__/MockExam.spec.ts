// MockExam 开始考试回归（空白页根因：模板用了 AnsweringSessionShell 但缺 import，
// Vue 把它当未知元素原样渲染为空壳）：点开始考试后卷面壳必须真实挂载。
// seam：页面层 mock @/api/mockExam（不碰真实后端），断言组件挂载与首题渲染。
import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'
import AnsweringSessionShell from '@/components/student/AnsweringSessionShell.vue'

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn(), post: vi.fn(), delete: vi.fn(), put: vi.fn() }
}))

vi.mock('@/api/mockExam', () => ({
  mockExamApi: {
    startMockExam: vi.fn(),
    saveProgress: vi.fn(),
    submitMockExam: vi.fn(),
    resumeMockExam: vi.fn(),
    getInProgress: vi.fn().mockResolvedValue({ exams: [] }),
    getMockExamHistory: vi.fn().mockResolvedValue({ exams: [], total: 0 })
  }
}))

vi.mock('@/api/realExam', () => ({
  realExamApi: { startExam: vi.fn() }
}))

vi.mock('@/api/training', () => ({
  trainingApi: {
    getCategories: vi.fn().mockResolvedValue({ categories: [{ id: 8, name: '维修' }] }),
    getHistoricalCategories: vi.fn().mockResolvedValue({ categories: [{ id: 8, name: '维修' }, { id: 9, name: '停用分类' }] })
  }
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ query: {}, params: {} })
}))

vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({}) }))

import { mockExamApi } from '@/api/mockExam'
import MockExam from '../MockExam.vue'

describe('MockExam 开始考试', () => {
  it('点开始考试后卷面壳挂载并渲染首题', async () => {
    vi.mocked(mockExamApi.startMockExam).mockResolvedValue({
      mock_exam_id: 1,
      questions: [{ id: 1, type: 'single_choice', content: 'Q1', options: { A: 'a', B: 'b' } }],
      remaining_time: 5400
    } as never)
    const wrapper = mount(MockExam, { global: { plugins: [epLite()] } })
    await flushPromises()
    const btn = wrapper.findAll('button').find((b) => b.text().includes('开始考试'))
    expect(btn?.exists()).toBe(true)
    expect(btn?.attributes('disabled')).toBeDefined()
    const category = wrapper.findAllComponents({ name: 'ElSelect' }).find(select => select.props('placeholder') === '选择培训分类')
    category!.vm.$emit('update:modelValue', 8)
    await flushPromises()
    await btn!.trigger('click')
    await flushPromises()
    await new Promise((r) => setTimeout(r, 50))
    await flushPromises()
    expect(wrapper.findComponent(AnsweringSessionShell).exists()).toBe(true)
    expect(wrapper.text()).toContain('Q1')
    expect(mockExamApi.startMockExam).toHaveBeenCalledWith(expect.objectContaining({ category_id: 8 }))
    wrapper.unmount()
  })

  it('历史未分类的在途考试直接恢复答案和剩余时间', async () => {
    vi.mocked(mockExamApi.getInProgress).mockResolvedValueOnce({ exams: [{ id: 21 }] } as never)
    vi.mocked(mockExamApi.resumeMockExam).mockResolvedValueOnce({
      mock_exam_id: 21,
      questions: [
        { id: 2, type: 'single_choice', content: '已答第一题' },
        { id: 1, type: 'single_choice', content: '继续第二题' }
      ],
      answers: { 2: 'A' },
      remaining_time: 321
    } as never)
    const wrapper = mount(MockExam, { global: { plugins: [epLite()] } })
    await flushPromises()
    await wrapper.findAll('button').find((b) => b.text() === '继续考试')!.trigger('click')
    await flushPromises()
    expect(mockExamApi.resumeMockExam).toHaveBeenCalledWith(21)
    const shell = wrapper.getComponent(AnsweringSessionShell)
    expect(shell.props('answers')).toEqual({ 2: 'A' })
    expect(shell.props('remainingTime')).toBe(321)
    expect(wrapper.text()).toContain('继续第二题')
    wrapper.unmount()
  })
})
