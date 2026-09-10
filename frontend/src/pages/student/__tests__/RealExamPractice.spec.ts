// RealExamPractice 页面级最小断言（#616）：外围交互（收藏/知识点/作答计时）改经
// useQuestionPeripherals 声明接入后的行为等价证据——
// 进题即查收藏、知识点为 'enter' 进题预取（未作答就查，与题库练习的 'result' 时机不同）、
// 收藏星切换成功后提示「已收藏」并点亮。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { paperId: '7' }, query: { title: '2024 真题卷' } }),
  useRouter: () => ({ push: vi.fn() })
}))

vi.mock('element-plus', async (importOriginal) => {
  const actual = await importOriginal<typeof import('element-plus')>()
  return {
    ...actual,
    ElMessage: { success: vi.fn(), warning: vi.fn(), error: vi.fn(), info: vi.fn() },
    ElMessageBox: { confirm: vi.fn() }
  }
})

// 退出练习确认已收口到 useConfirm（#735），mock 掉让其直接放行
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({
    confirm: vi.fn().mockResolvedValue('confirm'),
    confirmDanger: vi.fn().mockResolvedValue('confirm'),
    prompt: vi.fn().mockResolvedValue({ value: '' })
  })
}))

vi.mock('@/api/realExam', () => ({
  realExamApi: { startPractice: vi.fn() }
}))

vi.mock('@/api/practiceMode', () => ({
  practiceModeApi: {
    getProgress: vi.fn().mockResolvedValue(null),
    saveProgress: vi.fn().mockResolvedValue(undefined),
    submitAnswer: vi.fn()
  }
}))

vi.mock('@/api/favorite', () => ({
  favoriteApi: { check: vi.fn(), add: vi.fn(), remove: vi.fn(), list: vi.fn() }
}))

vi.mock('@/api/questionInteraction', () => ({
  questionInteractionApi: {
    listKnowledge: vi.fn().mockResolvedValue([]),
    listComments: vi.fn().mockResolvedValue({ items: [], total: 0 }),
    getNote: vi.fn().mockResolvedValue(null)
  }
}))

import { ElMessage } from 'element-plus'
import { epLite } from '@/test/element-lite'
import { createPinia } from 'pinia'
import { realExamApi } from '@/api/realExam'
import { favoriteApi } from '@/api/favorite'
import { questionInteractionApi } from '@/api/questionInteraction'
import RealExamPractice from '../RealExamPractice.vue'

const QUESTIONS = [
  { id: 201, type: 'single_choice', content: '题201', options: { A: '选项A', B: '选项B' } },
  { id: 202, type: 'single_choice', content: '题202', options: { A: '选项A', B: '选项B' } }
]

beforeEach(() => {
  vi.mocked(realExamApi.startPractice).mockResolvedValue({
    questions: QUESTIONS, current_index: 0
  } as never)
  vi.mocked(favoriteApi.check).mockResolvedValue({ favorited: false, favorite_id: 0 })
  vi.mocked(favoriteApi.add).mockResolvedValue({ favorite_id: 8 } as never)
  vi.mocked(questionInteractionApi.listKnowledge).mockResolvedValue([{ id: 1, name: '考点K' }])
})

describe('RealExamPractice 外围交互接入（#616）', () => {
  it("进题即查收藏并预取知识点（'enter' 触发，未作答就查）", async () => {
    const wrapper = mount(RealExamPractice, { global: { plugins: [epLite(), createPinia()] } })
    await flushPromises()

    expect(favoriteApi.check).toHaveBeenCalledWith({ target_type: 'question', target_id: 201 })
    expect(questionInteractionApi.listKnowledge).toHaveBeenCalledTimes(1)
    expect(questionInteractionApi.listKnowledge).toHaveBeenCalledWith(201)
    expect(wrapper.text()).toContain('2024 真题卷')
  })

  it('点击收藏星：add 成功后提示「已收藏」且星标点亮', async () => {
    const wrapper = mount(RealExamPractice, { global: { plugins: [epLite(), createPinia()] } })
    await flushPromises()

    await wrapper.find('.fav-star').trigger('click')
    await flushPromises()

    expect(favoriteApi.add).toHaveBeenCalledWith({ target_type: 'question', target_id: 201 })
    expect(vi.mocked(ElMessage.success)).toHaveBeenCalledWith('已收藏')
    expect(wrapper.find('.fav-star').classes()).toContain('text-warn')
  })
})
