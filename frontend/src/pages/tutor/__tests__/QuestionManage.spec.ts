// #412 讲师端题库管理：证件列渲染真实归属（而非恒为占位符），请求带 sort=id_asc。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus from 'element-plus'

vi.mock('@/api/questionBank', () => ({
  questionBankApi: { getQuestions: vi.fn(), getQuestion: vi.fn() },
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
  return mount(QuestionManage, { global: { plugins: [ElementPlus] } })
}

beforeEach(() => {
  vi.mocked(credentialApi.listCredentials).mockResolvedValue({
    credentials: [{ id: 3, code: 'forklift_n1', name: '叉车司机N1证', description: '', category: 'special_operation', level: null, sort_order: 1, status: 1, created_at: '', updated_at: '' }],
  })
  vi.mocked(questionBankApi.getQuestions).mockResolvedValue({
    total: 1,
    questions: [
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
