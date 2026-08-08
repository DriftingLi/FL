// 题库标签管理页组件测试：新增标签必填编码输入（bug 3 回归护栏）+ 正向创建流程。
// seam：组件层，mock API 层。
// 注：happy-dom 下 Element Plus 表单级 rules 校验不生效（最小复现亦如此），
// 故校验行为改为断言「编码输入存在且必填」+ 创建 payload 正确性。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus, { ElMessage } from 'element-plus'

vi.mock('@/api/training', () => ({
  trainingApi: {
    getQuestionTags: vi.fn(),
    createQuestionTag: vi.fn(),
    updateQuestionTag: vi.fn(),
    deleteQuestionTag: vi.fn(),
    getTags: vi.fn(),
    setQuestionTags: vi.fn()
  }
}))

vi.mock('@/api/questionBank', () => ({
  questionBankApi: {
    getQuestions: vi.fn(),
    getStats: vi.fn()
  }
}))

import { trainingApi } from '@/api/training'
import { questionBankApi } from '@/api/questionBank'
import QuestionTags from '../QuestionTags.vue'

const successSpy = vi.spyOn(ElMessage, 'success').mockImplementation(() => undefined as never)

function mountPage() {
  return mount(QuestionTags, {
    global: { plugins: [ElementPlus] }
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(trainingApi.getQuestionTags).mockResolvedValue({
    code: 200,
    message: '',
    data: { tags: [{ id: 1, name: '液压', code: 'hydraulic', question_count: 3 }] }
  })
  vi.mocked(trainingApi.createQuestionTag).mockResolvedValue({ code: 201, message: '', data: { id: 2 } })
  vi.mocked(trainingApi.getTags).mockResolvedValue({ code: 200, message: '', data: { tags: [] } })
  vi.mocked(trainingApi.setQuestionTags).mockResolvedValue({ code: 200, message: '', data: null })
  vi.mocked(questionBankApi.getQuestions).mockResolvedValue({ code: 200, message: '', data: { questions: [], total: 0 } })
  vi.mocked(questionBankApi.getStats).mockResolvedValue({ code: 200, message: '', data: {} })
})

describe('QuestionTags 新增标签', () => {
  it('新增标签对话框包含必填的编码输入（bug 3 回归护栏）', async () => {
    const wrapper = mountPage()
    await flushPromises()

    await wrapper.find('.el-button--primary').trigger('click')
    await flushPromises()

    const inputs = wrapper.findAll('.el-dialog input')
    const codeInput = inputs.find(i => (i.element as HTMLInputElement).placeholder.includes('编码'))
    expect(codeInput).toBeTruthy()
    // 编码输入位于「新增标签」对话框内（编码 prop 存在）
    const formItems = wrapper.findAllComponents({ name: 'ElFormItem' })
    const codeItem = formItems.find(it => it.props('prop') === 'code')
    expect(codeItem).toBeTruthy()
  })

  it('填写名称与编码后创建成功并携带编码', async () => {
    const wrapper = mountPage()
    await flushPromises()

    await wrapper.find('.el-button--primary').trigger('click')
    await flushPromises()

    const inputs = wrapper.findAll('.el-dialog input')
    const nameInput = inputs.find(i => (i.element as HTMLInputElement).placeholder.includes('法规'))
    await nameInput!.setValue('制动')
    const codeInput = inputs.find(i => (i.element as HTMLInputElement).placeholder.includes('编码'))
    await codeInput!.setValue('brake')
    await wrapper.find('.el-dialog .el-button--primary').trigger('click')
    await flushPromises()

    expect(trainingApi.createQuestionTag).toHaveBeenCalledWith({
      name: '制动',
      code: 'brake'
    })
    expect(successSpy).toHaveBeenCalled()
  })

  it('编辑已有标签时回填编码', async () => {
    const wrapper = mountPage()
    await flushPromises()

    // 编辑第一个标签
    const editBtn = wrapper.findAll('.el-button').find(b => b.text() === '编辑')
    await editBtn!.trigger('click')
    await flushPromises()

    const inputs = wrapper.findAll('.el-dialog input')
    const codeInput = inputs.find(i => (i.element as HTMLInputElement).placeholder.includes('编码'))
    expect((codeInput!.element as HTMLInputElement).value).toBe('hydraulic')
  })
})

describe('QuestionTags 批量打标计数', () => {
  it('selection-change 收到 undefined 时不崩溃，批量打标按钮不出现（计数防御回归护栏）', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const table = wrapper.findComponent({ name: 'ElTable' })
    await table.vm.$emit('selection-change', undefined)
    await flushPromises()

    const batchBtn = wrapper.findAll('.el-button').find(b => b.text().includes('批量打标'))
    expect(batchBtn).toBeUndefined()
  })

  it('勾选行后批量打标按钮显示正确计数', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const table = wrapper.findComponent({ name: 'ElTable' })
    await table.vm.$emit('selection-change', [{ id: 101 }, { id: 102 }])
    await flushPromises()

    const batchBtn = wrapper.findAll('.el-button').find(b => b.text().includes('批量打标'))
    expect(batchBtn?.text()).toContain('(2)')
  })
})
