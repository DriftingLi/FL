// 题库页按 URL 预选分类的用例（#2760 US-13/US-14）。
//
// 两条判据分开测，因为它们对应**两种不同的失败**：
//  1. 有效 category_id ⇒ 页面加载完分类后自动选中它，并允许直接开始练习
//     （失败表现：公开网校点「去练习」过来，学员还得再手选一次分类）
//  2. 无效 / 不在可选集内 ⇒ **不选中**（保持手动选择），也不报错
//     （失败表现：一个残留参数让 categoryId 落上一个越权/失效的值，
//      接着 beginPractice 打到后端 400，学员看到的是一次莫名其妙的失败）
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'
import { createPinia } from 'pinia'

vi.mock('@/api/practiceMode', () => ({
  practiceModeApi: {
    startCategoryPractice: vi.fn(),
    getPracticeStats: vi.fn().mockResolvedValue({ today_count: 0, total_count: 0, total_days: 0 }),
    submitCategoryAnswer: vi.fn()
  }
}))

// 可选分类只有 id=9（维修）与 id=10（安全操作）两个
vi.mock('@/api/training', () => ({
  trainingApi: {
    getTags: vi.fn().mockResolvedValue({ tags: [] }),
    getCategories: vi.fn().mockResolvedValue({
      categories: [{ id: 9, name: '维修' }, { id: 10, name: '安全操作' }]
    })
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

// useRoute 由本用例注入：预选逻辑只读 route.query.category_id。
const routeQuery = vi.hoisted(() => ({ current: {} as Record<string, unknown> }))
vi.mock('vue-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-router')>()
  return {
    ...actual,
    useRoute: () => ({ query: routeQuery.current })
  }
})

import { practiceModeApi } from '@/api/practiceMode'
import QuestionBank from '../QuestionBank.vue'

beforeEach(() => {
  routeQuery.current = {}
  vi.mocked(practiceModeApi.startCategoryPractice).mockReset()
  vi.mocked(practiceModeApi.startCategoryPractice).mockResolvedValue({
    questions: [{ id: 101, type: 'single_choice', content: '题101', options: { A: '选项A' } }],
    current_index: 0, attempt_key: 'k', answers_state: {}, completed: 0
  } as never)
})

// mountPage 挂载并等分类加载完。
async function mountPage() {
  const wrapper = mount(QuestionBank, { global: { plugins: [epLite(), createPinia()] } })
  await flushPromises()
  return wrapper
}

// selectedCategoryId 读「选择培训分类」下拉绑的 modelValue。
function selectedCategoryId(wrapper: Awaited<ReturnType<typeof mountPage>>) {
  const select = wrapper.findAllComponents({ name: 'ElSelect' })
    .find(item => item.props('placeholder') === '选择培训分类')
  return select?.props('modelValue')
}

describe('QuestionBank 按 URL 预选分类（#2760）', () => {
  it('URL 带有效的 category_id 时自动选中该分类', async () => {
    routeQuery.current = { category_id: '10' }
    const wrapper = await mountPage()
    expect(selectedCategoryId(wrapper)).toBe(10)
  })

  it('预选后可以直接开始练习，发出去的 category_id 就是 URL 里那个', async () => {
    routeQuery.current = { category_id: '10' }
    const wrapper = await mountPage()

    const startBtn = wrapper.findAll('button').find(b => b.text().includes('开始或继续练习'))!
    // 预选成功 ⇒ 按钮不再是 disabled（没有预选时它必须 disabled，见下一条）
    expect(startBtn.attributes('disabled')).toBeUndefined()
    await startBtn.trigger('click')
    await flushPromises()

    expect(practiceModeApi.startCategoryPractice).toHaveBeenCalledWith(
      expect.objectContaining({ category_id: 10 })
    )
  })

  it('category_id 不在可选分类里时保持未选中，按钮仍不可用', async () => {
    // 999 是个合法正整数，但不在 getCategories 的返回里（模拟「分类被停用/删掉」）
    routeQuery.current = { category_id: '999' }
    const wrapper = await mountPage()

    expect(selectedCategoryId(wrapper)).toBeNull()
    const startBtn = wrapper.findAll('button').find(b => b.text().includes('开始或继续练习'))!
    expect(startBtn.attributes('disabled')).toBeDefined()
  })

  it('category_id 形态非法（0 / 负数 / 非数字 / 数组）时一律忽略', async () => {
    for (const bad of ['0', '-3', 'abc', '9.5', ['9'], '']) {
      routeQuery.current = { category_id: bad }
      const wrapper = await mountPage()
      expect(selectedCategoryId(wrapper), `category_id=${JSON.stringify(bad)} 不该被选中`).toBeNull()
      wrapper.unmount()
    }
  })

  it('没有 category_id 时按原行为保持未选中（手动选择流程不变）', async () => {
    routeQuery.current = {}
    const wrapper = await mountPage()
    expect(selectedCategoryId(wrapper)).toBeNull()
    // 页面上不因为缺参数而报错
    expect(wrapper.text()).not.toContain('无法')
  })
})
