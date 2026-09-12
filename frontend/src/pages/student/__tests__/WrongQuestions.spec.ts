// 错题本（WrongQuestions.vue）契约测试：
// 1) 导出：点击「导出错题」→ 调用 exportWrongQuestions 拿到 Blob，并经统一 downloadBlob
//    以 wrong_questions.txt 落盘（复用唯一下载实现，F3）。
// 2) 重做 = 答题会话单题变体（#617）：提交管线/判分与练习同源（usePracticeSession 'single'），
//    外围三件（收藏/知识点/计时）经 questionPeripheralAdapters 工厂接入。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

vi.mock('@/api/wrongQuestion', () => ({
  wrongQuestionApi: {
    getWrongQuestions: vi.fn(),
    exportWrongQuestions: vi.fn(),
    redoWrongQuestion: vi.fn(),
    removeWrongQuestion: vi.fn(),
    batchRemoveWrongQuestions: vi.fn()
  }
}))

vi.mock('@/api/favorite', () => ({
  favoriteApi: { check: vi.fn(), add: vi.fn(), remove: vi.fn() }
}))

vi.mock('@/api/questionInteraction', () => ({
  questionInteractionApi: { listKnowledge: vi.fn() }
}))

vi.mock('@/composables/useReportDownload', () => ({
  downloadBlob: vi.fn()
}))

import { wrongQuestionApi } from '@/api/wrongQuestion'
import { favoriteApi } from '@/api/favorite'
import { questionInteractionApi } from '@/api/questionInteraction'
import { downloadBlob } from '@/composables/useReportDownload'
import WrongQuestions from '../WrongQuestions.vue'
import QuestionOptionPicker from '@/components/student/QuestionOptionPicker.vue'
import AnswerResultCard from '@/components/practice/AnswerResultCard.vue'

const wrongItem = {
  id: 1,
  question_id: 101,
  wrong_count: 2,
  is_redone: false,
  question: { type: 'single_choice', content: '测试题目', options: { A: '选项A', B: '选项B' } }
}

beforeEach(() => {
  vi.mocked(wrongQuestionApi.getWrongQuestions).mockResolvedValue({
    items: [{ ...wrongItem }],
    total: 1
  })
  vi.mocked(wrongQuestionApi.redoWrongQuestion).mockResolvedValue({
    is_correct: true,
    correct_answer: 'A',
    explanation: '解析',
    question_id: 101,
    user_answer: 'A'
  })
  vi.mocked(wrongQuestionApi.exportWrongQuestions).mockResolvedValue(
    new Blob(['题目一\n题目二\n'], { type: 'text/plain; charset=utf-8' })
  )
  vi.mocked(favoriteApi.check).mockResolvedValue({ favorited: false, favorite_id: 0 })
  vi.mocked(favoriteApi.add).mockResolvedValue({
    favorite_id: 9,
    target_type: 'question',
    target_id: 101
  })
  vi.mocked(favoriteApi.remove).mockResolvedValue(null)
  vi.mocked(questionInteractionApi.listKnowledge).mockResolvedValue([])
  vi.mocked(downloadBlob).mockClear()
})

function mountPage() {
  return mount(WrongQuestions, {
    global: {
      plugins: [epLite()],
      // CommentCard / NoteCard 挂载即拉 API（会话装配之外的关注点），stub 掉
      stubs: { CommentCard: true, NoteCard: true }
    }
  })
}

async function openRedo(wrapper: VueWrapper<any>) {
  const btn = wrapper.findAll('button').find(b => b.text().includes('重做'))
  expect(btn).toBeTruthy()
  await btn!.trigger('click')
  await flushPromises()
}

describe('WrongQuestions 错题导出', () => {
  it('点击导出错题：以 exportWrongQuestions 的 Blob 调用 downloadBlob，文件名为 wrong_questions.txt', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const btn = wrapper.findAll('button').find(b => b.text().includes('导出错题'))
    expect(btn).toBeTruthy()
    await btn!.trigger('click')
    await flushPromises()

    expect(wrongQuestionApi.exportWrongQuestions).toHaveBeenCalled()
    expect(downloadBlob).toHaveBeenCalledTimes(1)
    const [blob, fileName] = vi.mocked(downloadBlob).mock.calls[0]
    expect(blob).toBeInstanceOf(Blob)
    expect(fileName).toBe('wrong_questions.txt')
  })
})

describe('WrongQuestions 错题重做（会话单题变体 #617）', () => {
  it('点击重做：进入单题会话，外围收藏工厂按 question_id 查态（与练习同源），未提交不触达 redo 接口', async () => {
    const w = mountPage()
    await flushPromises()
    await openRedo(w)

    expect(favoriteApi.check).toHaveBeenCalledWith({ target_type: 'question', target_id: 101 })
    expect(wrongQuestionApi.redoWrongQuestion).not.toHaveBeenCalled()
    // 作答态渲染共享选项选择器
    expect(w.findComponent(QuestionOptionPicker).exists()).toBe(true)
  })

  it('重做提交走会话提交管线：单选作答经 redoWrongQuestion 判分，结果卡与知识点同装配，答对标记已重做', async () => {
    vi.mocked(questionInteractionApi.listKnowledge).mockResolvedValue([{ id: 3, name: '考点X' }])
    const w = mountPage()
    await flushPromises()
    await openRedo(w)

    await w.findComponent(QuestionOptionPicker).findAll('.q-option')[1].trigger('click') // 选 B
    const submitBtn = w.findAll('button').find(b => b.text().includes('提交'))
    await submitBtn!.trigger('click')
    await flushPromises()

    // 单题提交经共享提交管线 → redo 接口（判分口径后端统一落 practice record）
    expect(wrongQuestionApi.redoWrongQuestion).toHaveBeenCalledWith(101, 'B')
    // 结果区渲染（AnswerResultCard 拿到判定结果与计时）
    const resultCard = w.findComponent(AnswerResultCard)
    expect(resultCard.exists()).toBe(true)
    expect(resultCard.props('userAnswer')).toBe('B')
    expect(resultCard.props('isCorrect')).toBe(true)
    // 知识点外围出结果后按结果题目查询（与练习 'result' 触发形态一致）
    expect(questionInteractionApi.listKnowledge).toHaveBeenCalledWith(101)
    // 答对：列表项标记已重做
    expect(w.text()).toContain('已重做')
  })

  it('多选作答以「, 」拼接经提交管线传给 redo 接口', async () => {
    vi.mocked(wrongQuestionApi.getWrongQuestions).mockResolvedValue({
      items: [{ ...wrongItem, question: { type: 'multi_choice', content: '多选', options: { A: '甲', B: '乙' } } }],
      total: 1
    })
    const w = mountPage()
    await flushPromises()
    await openRedo(w)

    const options = w.findComponent(QuestionOptionPicker).findAll('.q-option')
    await options[0].trigger('click')
    await options[1].trigger('click')
    const submitBtn = w.findAll('button').find(b => b.text().includes('提交'))
    await submitBtn!.trigger('click')
    await flushPromises()

    expect(wrongQuestionApi.redoWrongQuestion).toHaveBeenCalledWith(101, 'A, B')
  })

  it('redo 接口失败：会话保持作答态（不渲染结果卡），错误由拦截器提示', async () => {
    vi.mocked(wrongQuestionApi.redoWrongQuestion).mockRejectedValue(new Error('network'))
    const w = mountPage()
    await flushPromises()
    await openRedo(w)

    await w.findComponent(QuestionOptionPicker).findAll('.q-option')[0].trigger('click')
    const submitBtn = w.findAll('button').find(b => b.text().includes('提交'))
    await submitBtn!.trigger('click')
    await flushPromises()

    expect(w.findComponent(AnswerResultCard).exists()).toBe(false)
    // 作答态仍在（可重新提交）
    expect(w.findComponent(QuestionOptionPicker).exists()).toBe(true)
  })

  it('关闭重做：退出单题会话回到列表操作态', async () => {
    const w = mountPage()
    await flushPromises()
    await openRedo(w)

    const closeBtn = w.findAll('button').find(b => b.text().includes('取消'))
    await closeBtn!.trigger('click')
    await flushPromises()

    expect(w.findComponent(QuestionOptionPicker).exists()).toBe(false)
    expect(w.findAll('button').filter(b => b.text().includes('重做')).length).toBe(1)
  })

  it('重做面板收藏切换：经外围 toggleFavorite 走 question 域收藏 API', async () => {
    const w = mountPage()
    await flushPromises()
    await openRedo(w)

    // 面板内收藏药丸（UiActionChip）点击 → add
    const chip = w.findAll('button, [class*="chip"]').find(el => el.text().includes('收藏'))
    expect(chip).toBeTruthy()
    await chip!.trigger('click')
    await flushPromises()

    expect(favoriteApi.add).toHaveBeenCalledWith({ target_type: 'question', target_id: 101 })
  })
})
