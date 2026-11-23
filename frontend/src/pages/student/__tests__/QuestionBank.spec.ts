// QuestionBank 页面级最小断言（#616）：外围交互（收藏/知识点/作答计时）改经
// useQuestionPeripherals 声明接入后的行为等价证据——
// 进题查收藏状态、切换收藏、知识点只在出结果后查（'result' 触发时机）、
// 切题对新题重查收藏、结果卡拿到作答用时。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'
import { createPinia } from 'pinia'

vi.mock('@/api/practiceMode', () => ({
  practiceModeApi: {
    startCategoryPractice: vi.fn(),
    getPracticeStats: vi.fn().mockResolvedValue({ today_count: 1, total_count: 10, total_days: 2 }),
    submitCategoryAnswer: vi.fn()
  }
}))

vi.mock('@/api/training', () => ({
  trainingApi: {
    getTags: vi.fn().mockResolvedValue({ tags: [] }),
    getCategories: vi.fn().mockResolvedValue({ categories: [{ id: 9, name: '维修' }] })
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

import { practiceModeApi } from '@/api/practiceMode'
import { favoriteApi } from '@/api/favorite'
import { questionInteractionApi } from '@/api/questionInteraction'
import { trainingApi } from '@/api/training'
import AnswerResultCard from '@/components/practice/AnswerResultCard.vue'
import QuestionBank from '../QuestionBank.vue'

const QUESTIONS = [
  { id: 101, type: 'single_choice', content: '题101', options: { A: '选项A', B: '选项B' } },
  { id: 102, type: 'single_choice', content: '题102', options: { A: '选项A', B: '选项B' } }
]

beforeEach(() => {
  vi.mocked(practiceModeApi.startCategoryPractice).mockResolvedValue({
    questions: QUESTIONS, current_index: 0, attempt_key: 'practice-attempt', answers_state: {}, completed: 0
  } as never)
  vi.mocked(practiceModeApi.submitCategoryAnswer).mockResolvedValue({
    is_correct: true, correct_answer: 'A', explanation: '解析',
    question_id: 101, user_answer: 'A', accuracy_rate: 90, common_wrong: null
  } as never)
  vi.mocked(favoriteApi.check).mockResolvedValue({ favorited: false, favorite_id: 0 })
  vi.mocked(favoriteApi.add).mockResolvedValue({ favorite_id: 5 } as never)
  vi.mocked(questionInteractionApi.listKnowledge).mockResolvedValue([{ id: 1, name: '考点A' } as never])
})

async function mountAndStart() {
  // CommentCard/NoteCard（结果区）依赖 auth store，mount 需挂 Pinia
  const wrapper = mount(QuestionBank, { global: { plugins: [epLite(), createPinia()] } })
  await flushPromises()
  const category = wrapper.findAllComponents({ name: 'ElSelect' }).find(select => select.props('placeholder') === '选择培训分类')
  category!.vm.$emit('update:modelValue', 9)
  await flushPromises()
  const startBtn = wrapper.findAll('button').find(b => b.text().includes('开始或继续练习'))
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

describe('QuestionBank 分类启动', () => {
  it('未选分类时不能开始，选择分类后只发送本次分类和练习模式', async () => {
    vi.mocked(practiceModeApi.startCategoryPractice).mockClear()
    vi.mocked(trainingApi.getTags).mockClear()
    const wrapper = mount(QuestionBank, { global: { plugins: [epLite(), createPinia()] } })
    await flushPromises()
    expect(trainingApi.getTags).toHaveBeenCalledWith()
    const button = wrapper.findAll('button').find(b => b.text().includes('开始或继续练习'))!
    expect(button.attributes('disabled')).toBeDefined()
    const category = wrapper.findAllComponents({ name: 'ElSelect' }).find(select => select.props('placeholder') === '选择培训分类')
    category!.vm.$emit('update:modelValue', 9)
    await flushPromises()
    await button.trigger('click')
    await flushPromises()
    expect(practiceModeApi.startCategoryPractice).toHaveBeenCalledWith(expect.objectContaining({ category_id: 9, mode: 'sequential' }))
    expect(wrapper.text()).toContain('维修')
  })
})
