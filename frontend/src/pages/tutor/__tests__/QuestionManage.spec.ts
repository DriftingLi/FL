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
// #2598：页面新增「只看我创建的」开关，需读当前讲师 id（authStore.userInfo.user_id）。
// 页面测试不装 pinia，故按本仓其他页面 spec 的惯例把 store 也 mock 掉。
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ userInfo: { user_id: 7, role: 'tutor' } }),
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

// #2598 题库写隔离前端两件套：创建者列（让讲师一眼分清哪些题能动手）+「只看我创建的」筛选轴。
// 两件都不新增接口：前者吃 DTO 既有的 created_by，后者只是多传一个 created_by 查询参数。
describe('创建者列与「只看我创建的」（#2598）', () => {
  const questionRow = (over: Record<string, unknown>) => ({
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
    ...over,
  })

  function mockRows(rows: unknown[]) {
    vi.mocked(questionBankApi.getQuestions).mockResolvedValue({ total: rows.length, items: rows } as never)
  }

  it('自己创建的题标「我」、别人的标「其他讲师」、无创建者标「历史题」', async () => {
    mockRows([
      questionRow({ id: 1, created_by: 7 }), // authStore mock 的 user_id = 7
      questionRow({ id: 2, created_by: 9 }),
      questionRow({ id: 3, created_by: null }),
    ])
    const wrapper = mountPage()
    await flushPromises()
    const text = wrapper.text()
    expect(text).toContain('我')
    expect(text).toContain('其他讲师')
    // 空创建者是历史遗留（规格 C2：只有管理员能动），不能显示成「其他讲师」——
    // 那会让讲师以为去找那位同事商量就能改
    expect(text).toContain('历史题')
  })

  it('开关打开后请求带 created_by=当前讲师，并回到第 1 页', async () => {
    vi.mocked(questionBankApi.getQuestions).mockClear()
    const wrapper = mountPage()
    await flushPromises()
    // 初始不带筛选
    expect(vi.mocked(questionBankApi.getQuestions).mock.calls[0][0]?.created_by).toBeUndefined()

    wrapper.findComponent({ name: 'UiSwitch' }).vm.$emit('update:modelValue', true)
    wrapper.findComponent({ name: 'UiSwitch' }).vm.$emit('change', true)
    await flushPromises()

    const last = vi.mocked(questionBankApi.getQuestions).mock.calls.at(-1)?.[0]
    expect(last?.created_by).toBe(7)
    expect(last?.page).toBe(1)
  })

  it('开关关闭后请求不再带 created_by', async () => {
    const wrapper = mountPage()
    await flushPromises()
    const sw = wrapper.findComponent({ name: 'UiSwitch' })
    sw.vm.$emit('update:modelValue', true)
    sw.vm.$emit('change', true)
    await flushPromises()
    sw.vm.$emit('update:modelValue', false)
    sw.vm.$emit('change', false)
    await flushPromises()
    expect(vi.mocked(questionBankApi.getQuestions).mock.calls.at(-1)?.[0]?.created_by).toBeUndefined()
  })
})
