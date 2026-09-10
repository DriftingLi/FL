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
    getMockExamHistory: vi.fn().mockResolvedValue({ exams: [] })
  }
}))

vi.mock('@/api/realExam', () => ({
  realExamApi: { startExam: vi.fn() }
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
    await btn!.trigger('click')
    await flushPromises()
    await new Promise((r) => setTimeout(r, 50))
    await flushPromises()
    expect(wrapper.findComponent(AnsweringSessionShell).exists()).toBe(true)
    expect(wrapper.text()).toContain('Q1')
  })
})
