import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'
import type { QuestionDTO } from '@/api/questionBank'

const routeState = vi.hoisted(() => ({
  query: {} as Record<string, string>,
  push: vi.fn()
}))

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
  useRouter: () => ({ push: routeState.push, back: vi.fn() })
}))

vi.mock('@/api/training', () => ({
  trainingApi: {
    getTags: vi.fn(),
    getCategories: vi.fn()
  }
}))

vi.mock('@/api/questionBank', () => ({
  questionBankApi: {
    getQuestion: vi.fn(),
    createQuestion: vi.fn(),
    updateQuestion: vi.fn(),
    uploadImage: vi.fn()
  }
}))

import { questionBankApi } from '@/api/questionBank'
import { trainingApi } from '@/api/training'
import QuestionCreate from '../QuestionCreate.vue'

function questionOf(overrides: Partial<QuestionDTO> = {}): QuestionDTO {
  return {
    id: 42,
    type: 'single_choice',
    content: '题干',
    options: { A: '甲', B: '乙', C: '丙', D: '丁' },
    answer: 'A',
    explanation: '',
    image_url: '',
    reference_answer: '',
    scoring_criteria: '',
    score: 3,
    status: 'draft',
    reject_reason: '',
    created_by: 7,
    created_by_type: 'tutor',
    credential_id: 3,
    created_at: '',
    updated_at: '',
    ...overrides
  }
}

function mountPage() {
  return mount(QuestionCreate, { global: { plugins: [epLite()] } })
}

beforeEach(() => {
  vi.clearAllMocks()
  routeState.query = {}
  vi.mocked(trainingApi.getTags).mockResolvedValue({ tags: [] })
  vi.mocked(trainingApi.getCategories).mockResolvedValue({
    categories: [
      { id: 11, name: '叉车安全', status: 1 },
      { id: 12, name: '已停用分类', status: 0 }
    ]
  })
})

describe('QuestionCreate 培训分类', () => {
  it('创建只列启用分类，并把所选 canonical 分类写入请求', async () => {
    vi.mocked(questionBankApi.createQuestion).mockResolvedValue(questionOf())
    const wrapper = mountPage()
    await flushPromises()

    const select = wrapper.findAllComponents({ name: 'UiSelect' })
      .find(item => item.props('placeholder') === '选择培训分类')!
    const options = select.props('options') as { label: string; value: number }[]
    expect(options).toEqual([{ label: '叉车安全', value: 11 }])

    await select.vm.$emit('update:modelValue', 11)
    const createButton = wrapper.findAll('button').find(button => button.text().includes('创建'))!
    await createButton.trigger('click')
    await flushPromises()

    expect(questionBankApi.createQuestion).toHaveBeenCalledTimes(1)
    const payload = vi.mocked(questionBankApi.createQuestion).mock.calls[0][0]
    expect(payload.category_id).toBe(11)
    expect(payload).not.toHaveProperty('public_state')
    expect(routeState.push).toHaveBeenCalledWith({ name: 'TutorQuestionManage' })
  })

  it('编辑按 canonical 分类回显，并不把只读分类与公开资格回流到更新请求', async () => {
    routeState.query = { id: '42' }
    vi.mocked(questionBankApi.getQuestion).mockResolvedValue(questionOf({
      category: { id: 99, name: '旧分类', status: 0 },
      public_state: {
        category: { id: 11, name: '叉车安全', status: 1 },
        public_visibility: true,
        eligibility: { eligible: false, reasons: ['not_published'] }
      },
      public_eligibility: { eligible: true, reasons: [] }
    }))
    vi.mocked(questionBankApi.updateQuestion).mockResolvedValue(questionOf())
    const wrapper = mountPage()
    await flushPromises()

    const select = wrapper.findAllComponents({ name: 'UiSelect' })
      .find(item => item.props('placeholder') === '选择培训分类')!
    expect(select.props('modelValue')).toBe(11)

    const updateButton = wrapper.findAll('button').find(button => button.text().includes('更新'))!
    await updateButton.trigger('click')
    await flushPromises()

    expect(questionBankApi.updateQuestion).toHaveBeenCalledTimes(1)
    const payload = vi.mocked(questionBankApi.updateQuestion).mock.calls[0][1]
    expect(payload.category_id).toBe(11)
    expect(payload).not.toHaveProperty('category')
    expect(payload).not.toHaveProperty('public_state')
  })
})
