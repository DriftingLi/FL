// QuestionBank 页面级最小断言（#616）：外围交互（收藏/知识点/作答计时）改经
// useQuestionPeripherals 声明接入后的行为等价证据——
// 进题查收藏状态、切换收藏、知识点只在出结果后查（'result' 触发时机）、
// 切题对新题重查收藏、结果卡拿到作答用时。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createPinia } from 'pinia'

vi.mock('@/api/questionBank', () => ({
  questionBankApi: { getStats: vi.fn().mockResolvedValue({ total: 10 }) }
}))

vi.mock('@/api/practiceMode', () => ({
  practiceModeApi: {
    startSequential: vi.fn(),
    startTagPractice: vi.fn(),
    getFreeQuestions: vi.fn(),
    getProgress: vi.fn().mockResolvedValue(null),
    getSequentialProgress: vi.fn().mockResolvedValue(null),
    getPracticeStats: vi.fn().mockResolvedValue({ today_count: 1, total_count: 10, total_days: 2 }),
    saveProgress: vi.fn().mockResolvedValue(undefined),
    submitAnswer: vi.fn()
  }
}))

vi.mock('@/api/training', () => ({
  trainingApi: { getTags: vi.fn().mockResolvedValue({ tags: [] }) }
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

import { practiceModeApi } from '@/api/practiceMode'
import { favoriteApi } from '@/api/favorite'
import { questionInteractionApi } from '@/api/questionInteraction'
import AnswerResultCard from '@/components/practice/AnswerResultCard.vue'
import QuestionBank from '../QuestionBank.vue'

const QUESTIONS = [
  { id: 101, type: 'single_choice', content: '题101', options: { A: '选项A', B: '选项B' } },
  { id: 102, type: 'single_choice', content: '题102', options: { A: '选项A', B: '选项B' } }
]

beforeEach(() => {
  vi.mocked(practiceModeApi.startSequential).mockResolvedValue({
    questions: QUESTIONS, current_index: 0
  } as never)
  vi.mocked(practiceModeApi.submitAnswer).mockResolvedValue({
    is_correct: true, correct_answer: 'A', explanation: '解析',
    question_id: 101, user_answer: 'A', accuracy_rate: 90, common_wrong: null
  } as never)
  vi.mocked(favoriteApi.check).mockResolvedValue({ favorited: false, favorite_id: 0 })
  vi.mocked(favoriteApi.add).mockResolvedValue({ favorite_id: 5 } as never)
  vi.mocked(questionInteractionApi.listKnowledge).mockResolvedValue([{ id: 1, name: '考点A' }])
})

async function mountAndStart() {
  // CommentCard/NoteCard（结果区）依赖 auth store，mount 需挂 Pinia
  const wrapper = mount(QuestionBank, { global: { plugins: [ElementPlus, createPinia()] } })
  await flushPromises()
  const startBtn = wrapper.findAll('button').find(b => b.text().includes('开始练习'))
  await startBtn!.trigger('click')
  await flushPromises()
  return wrapper
}

describe('QuestionBank 外围交互接入（#616）', () => {
  it('进题查收藏状态；点击收藏 chip 调 add 并切到已收藏', async () => {
    const wrapper = await mountAndStart()

    expect(favoriteApi.check).toHaveBeenCalledWith({ target_type: 'question', target_id: 101 })
    const chip = wrapper.findAll('button').find(b => b.text().includes('收藏'))
    expect(chip).toBeTruthy()

    await chip!.trigger('click')
    await flushPromises()

    expect(favoriteApi.add).toHaveBeenCalledWith({ target_type: 'question', target_id: 101 })
    expect(wrapper.findAll('button').find(b => b.text().includes('已收藏'))).toBeTruthy()
  })

  it("知识点为 'result' 触发：进题不查；提交后按结果题目查并渲染考点", async () => {
    const wrapper = await mountAndStart()
    expect(questionInteractionApi.listKnowledge).not.toHaveBeenCalled()

    await wrapper.find('.q-option').trigger('click')
    const submitBtn = wrapper.findAll('button').find(b => b.text().includes('提交答案'))
    await submitBtn!.trigger('click')
    await flushPromises()

    expect(questionInteractionApi.listKnowledge).toHaveBeenCalledWith(101)
    expect(wrapper.text()).toContain('考点A')
  })

  it('结果卡拿到作答用时（duration 外围回填）', async () => {
    const wrapper = await mountAndStart()

    await wrapper.find('.q-option').trigger('click')
    const submitBtn = wrapper.findAll('button').find(b => b.text().includes('提交答案'))
    await submitBtn!.trigger('click')
    await flushPromises()

    const resultCard = wrapper.findComponent(AnswerResultCard)
    expect(resultCard.exists()).toBe(true)
    expect(typeof resultCard.props('durationSeconds')).toBe('number')
  })

  it('切题后对新题重查收藏状态', async () => {
    const wrapper = await mountAndStart()

    const nextBtn = wrapper.findAll('button').find(b => b.text().includes('下一题'))
    await nextBtn!.trigger('click')
    await flushPromises()

    expect(favoriteApi.check).toHaveBeenCalledWith({ target_type: 'question', target_id: 102 })
  })
})
