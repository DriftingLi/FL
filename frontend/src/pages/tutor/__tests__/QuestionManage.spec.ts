// #412 讲师端题库管理：证件列渲染真实归属（而非恒为占位符），请求带 sort=id_asc。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

vi.mock('@/api/questionBank', () => ({
  questionBankApi: {
    getQuestions: vi.fn(),
    getQuestion: vi.fn(),
    submitQuestion: vi.fn(),
    updateQuestion: vi.fn(),
  },
}))
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: vi.fn().mockResolvedValue(true) }),
}))
vi.mock('@/api/credential', () => ({
  credentialApi: { listCredentials: vi.fn() },
}))
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

import { questionBankApi } from '@/api/questionBank'
import { credentialApi } from '@/api/credential'
import QuestionManage from '../QuestionManage.vue'

function mountPage() {
  return mount(QuestionManage, { global: { plugins: [epLite()] } })
}

beforeEach(() => {
  vi.mocked(credentialApi.listCredentials).mockResolvedValue({
    credentials: [{ id: 3, code: 'forklift_n1', name: '叉车司机N1证', description: '', category: 'special_operation', level: null, sort_order: 1, status: 1, created_at: '', updated_at: '' }],
  })
  // 票 6（ADR-0060 决策 6）：列表函数出口给的是中立容器 Page<T>，fixture 不再写域键
  vi.mocked(questionBankApi.getQuestions).mockResolvedValue({
    total: 1,
    items: [
      {
        id: 42,
        type: 'single_choice',
        content: '题干',
        options: null,
        image_url: '',
        status: 'published',
        reject_reason: '',
        score: 3,
        created_by: null,
        created_by_type: 'tutor',
        credential_id: 3,
        created_at: '2026-08-31T10:00:00Z',
        updated_at: '2026-08-31T10:00:00Z',
      },
    ],
  })
})

describe('QuestionManage 证件列与排序（#412）', () => {
  it('证件列渲染真实归属而非占位符', async () => {
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('叉车司机N1证')
    expect(wrapper.text()).not.toContain('—')
  })

  it('列表请求显式携带 sort=id_asc', async () => {
    mountPage()
    await flushPromises()
    const params = vi.mocked(questionBankApi.getQuestions).mock.calls[0][0]
    expect(params && params.sort).toBe('id_asc')
  })
})

// 第十二波票 6 前端配套（#1168）：「提交审核」改接显式动作端点——
// 写面已不携带 status 通道，review 动作必须走 POST /questions/:id/submit，不得再借 updateQuestion 回写 status。
describe('提交审核改接显式端点（票 6）', () => {
  it('下拉 review 走 submitQuestion(42)，不经 updateQuestion 写 status', async () => {
    vi.mocked(questionBankApi.submitQuestion).mockResolvedValue({} as never)
    const wrapper = mountPage()
    await flushPromises()
    const dropdown = wrapper.findComponent({ name: 'ElDropdown' })
    expect(dropdown.exists()).toBe(true)
    dropdown.vm.$emit('command', 'review')
    await flushPromises()
    expect(questionBankApi.submitQuestion).toHaveBeenCalledWith(42)
    expect(questionBankApi.updateQuestion).not.toHaveBeenCalled()
  })
})
