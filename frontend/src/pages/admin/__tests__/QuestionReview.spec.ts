// 管理端题库审核：分类筛选、公开原因与审核终态刷新。
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { ElMessage } from 'element-plus'
import { epLite } from '@/test/element-lite'
import UiButton from '@/components/ui/UiButton.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import type { Question } from '@/types/question'
import type { Eligibility } from '@/api/generated/course'

const confirmMock = vi.hoisted(() => vi.fn())

vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: confirmMock })
}))

vi.mock('@/api/questionBank', () => ({
  questionBankApi: {
    getQuestions: vi.fn(),
    publishQuestion: vi.fn(),
    rejectQuestion: vi.fn(),
    batchPublish: vi.fn(),
    batchReject: vi.fn()
  }
}))

vi.mock('@/api/training', () => ({
  trainingApi: {
    getAdminCategories: vi.fn()
  }
}))

import { questionBankApi, type QuestionDTO } from '@/api/questionBank'
import { trainingApi } from '@/api/training'
import QuestionReview from '../QuestionReview.vue'

type ReviewQuestion = Question & {
  category?: { id: number; name: string; status: number }
  public_eligibility?: Eligibility
}

function questionDTO(over: Partial<ReviewQuestion> = {}): ReviewQuestion {
  return {
    id: 11,
    type: 'single_choice',
    content: '液压泵异响如何处理？',
    options: { A: '检查油位' },
    image_url: '',
    status: 'pending',
    reject_reason: '',
    score: 5,
    created_by: 2,
    created_by_type: 'tutor',
    created_at: '2026-11-01T10:00:00+08:00',
    updated_at: '2026-11-01T10:00:00+08:00',
    category: { id: 9, name: '维修', status: 1 },
    public_eligibility: { eligible: false, reasons: ['not_published', 'source_tag_excluded'] },
    ...over
  }
}

const wrappers: VueWrapper[] = []
let listResult: { items: ReviewQuestion[]; total: number }

function mountPage() {
  const wrapper = mount(QuestionReview, {
    attachTo: document.body,
    global: { plugins: [epLite()] }
  })
  wrappers.push(wrapper)
  return wrapper
}

async function openDetail(wrapper: VueWrapper) {
  const row = wrapper.findAll('.el-table__row').find(item => item.text().includes('液压泵异响如何处理'))!
  await row.findComponent({ name: 'ElDropdown' }).vm.$emit('command', 'view')
  await flushPromises()
  return wrapper.findAllComponents(UiDialog).find(dialog => dialog.props('title') === '题目详情')!
}

beforeEach(() => {
  vi.clearAllMocks()
  confirmMock.mockResolvedValue(undefined)
  listResult = { items: [questionDTO()], total: 1 }
  vi.mocked(questionBankApi.getQuestions).mockImplementation(async params => {
    if (params.page_size === 1) return { items: [], total: 0 }
    return listResult
  })
  vi.mocked(trainingApi.getAdminCategories).mockResolvedValue({
    categories: [
      { id: 9, name: '维修', legacy_credential_id: 1, status: 1 },
      { id: 8, name: '旧分类', legacy_credential_id: 2, status: 0 }
    ]
  })
  vi.mocked(questionBankApi.publishQuestion).mockResolvedValue(
    questionDTO({ status: 'published', public_eligibility: { eligible: false, reasons: ['source_tag_excluded'] } }) as unknown as QuestionDTO
  )
  vi.mocked(questionBankApi.rejectQuestion).mockResolvedValue(
    questionDTO({
      status: 'draft',
      reject_reason: '分类信息不足',
      public_eligibility: { eligible: false, reasons: ['not_published', 'source_tag_excluded'] }
    }) as unknown as QuestionDTO
  )
  vi.spyOn(ElMessage, 'success').mockImplementation(() => undefined as never)
})

afterEach(() => {
  for (const wrapper of wrappers.splice(0)) wrapper.unmount()
  document.body.replaceChildren()
})

describe('QuestionReview 分类与公开状态', () => {
  it('分类筛选只列启用分类并提供遗留桶，查询参数传给后端', async () => {
    const wrapper = mountPage()
    await flushPromises()

    expect(questionBankApi.getQuestions).toHaveBeenCalledWith(expect.objectContaining({
      page: 1,
      page_size: 10,
      status: 'pending',
      category_id: 'all'
    }))

    const categorySelect = wrapper.findAllComponents(UiSelect)
      .find(select => select.props('placeholder') === '培训分类')!
    expect(categorySelect.props('options')).toEqual([
      { label: '全部分类', value: 'all' },
      { label: '维修', value: 9 },
      { label: '未分类/分类失效', value: 'legacy' }
    ])

    categorySelect.findComponent({ name: 'ElSelect' }).vm.$emit('update:modelValue', 'legacy')
    categorySelect.findComponent({ name: 'ElSelect' }).vm.$emit('change', 'legacy')
    await flushPromises()
    expect(questionBankApi.getQuestions).toHaveBeenCalledWith(expect.objectContaining({ category_id: 'legacy' }))
  })

  it('列表展示首个原因，点击题干可在详情看全部原因', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const row = wrapper.findAll('.el-table__row')[0]
    expect(row.text()).toContain('维修')
    expect(row.text()).toContain('待审核')
    expect(row.text()).toContain('未发布')
    expect(row.text()).not.toContain('来源标签题不计入公开题库')

    await row.get('button').trigger('click')
    await flushPromises()
    const detail = wrapper.findAllComponents(UiDialog).find(dialog => dialog.props('title') === '题目详情')!
    expect(detail.props('modelValue')).toBe(true)
    expect(document.body.textContent).toContain('分类：维修')
    expect(document.body.textContent).toContain('审核状态：待审核')
    expect(document.body.textContent).toContain('未发布')
    expect(document.body.textContent).toContain('来源标签题不计入公开题库')
  })

  it('批量操作只在表格有选中项时出现', async () => {
    const wrapper = mountPage()
    await flushPromises()
    const table = wrapper.getComponent({ name: 'ElTable' })
    expect(wrapper.text()).not.toContain('批量发布')
    table.vm.$emit('selection-change', [{ id: 11 }])
    await flushPromises()
    expect(wrapper.get('[role="status"]').text()).toBe('已选 1 道题')
    expect(wrapper.text()).toContain('批量驳回')
    table.vm.$emit('selection-change', [])
    await flushPromises()
    expect(wrapper.text()).not.toContain('批量发布')
  })

  it('发布后用接口终态刷新列表和详情，保留剩余公开原因', async () => {
    const wrapper = mountPage()
    await flushPromises()
    const detail = await openDetail(wrapper)

    const publish = detail.findAllComponents(UiButton).find(button => button.text() === '发布')!
    await publish.trigger('click')
    await flushPromises()

    expect(questionBankApi.publishQuestion).toHaveBeenCalledWith(11)
    expect(questionBankApi.getQuestions).toHaveBeenCalledTimes(4)
    expect(detail.props('modelValue')).toBe(true)
    expect(document.body.textContent).toContain('审核状态：已发布')
    expect(document.body.textContent).toContain('来源标签题不计入公开题库')
    expect(document.body.textContent).not.toContain('未发布')
  })

  it('驳回后用接口终态刷新列表和详情，并展示驳回原因', async () => {
    const wrapper = mountPage()
    await flushPromises()
    const detail = await openDetail(wrapper)

    const reject = detail.findAllComponents(UiButton).find(button => button.text() === '驳回')!
    await reject.trigger('click')
    await flushPromises()

    const rejectDialog = wrapper.findAllComponents(UiDialog)
      .find(dialog => dialog.props('title') === '填写驳回理由')!
    const reason = rejectDialog.findComponent(UiInput)
    await reason.find('textarea').setValue('分类信息不足')
    await flushPromises()
    expect(reason.props('modelValue')).toBe('分类信息不足')
    expect((wrapper.vm as unknown as { rejectReason: string }).rejectReason).toBe('分类信息不足')
    expect((wrapper.vm as unknown as { rejecting: boolean }).rejecting).toBe(false)
    await (wrapper.vm as unknown as { confirmReject: () => Promise<void> }).confirmReject()
    await flushPromises()

    expect(questionBankApi.rejectQuestion).toHaveBeenCalledWith(11, '分类信息不足')
    expect(questionBankApi.getQuestions).toHaveBeenCalledTimes(4)
    expect(detail.props('modelValue')).toBe(true)
    expect(document.body.textContent).toContain('审核状态：草稿')
    expect(document.body.textContent).toContain('该题目已被驳回')
    expect(document.body.textContent).toContain('分类信息不足')
  })
})
