// usePracticeSession：练习会话状态机 module 的接口级测试。
// 断言 external behavior：三态反序列化（null/[]/absent）、断点恢复、
// 退出清空、序列化 round-trip（经 saveProgress 可观测）、游标推进/进度保存编排、
// onQuestionEnter 题目进入生命周期钩子（进入/切题/回退/退出按序触发，同题渲染不误触发）。
// seam：composable 接口——三个 adapter（start/submit/saveProgress）用内存 stub，
// API 与进度 key 语义由 adapter 注入，本测试不触达 API 层。
import { describe, it, expect } from 'vitest'
import { nextTick } from 'vue'
import { usePracticeSession } from '@/composables/usePracticeSession'
import type { PracticeSessionAdapters, PracticeMode } from '@/composables/usePracticeSession'
import type { Question } from '@/types/question'

function q(id: number): Question {
  return { id, type: 'single_choice', content: `题${id}` } as Question
}

function makeAdapters(
  overrides: Partial<PracticeSessionAdapters> = {}
): PracticeSessionAdapters & { saved: { mode: PracticeMode; index: number; answersState: Record<string, unknown> }[] } {
  const saved: { mode: PracticeMode; index: number; answersState: Record<string, unknown> }[] = []
  return {
    start: async () => ({
      questions: [q(1), q(2), q(3)],
      startIndex: 0,
      answersState: null
    }),
    submit: async () => ({
      is_correct: true,
      correct_answer: 'A',
      explanation: '解析',
      question_id: 1,
      user_answer: 'A'
    }),
    saveProgress: async payload => {
      saved.push({ mode: payload.mode, index: payload.index, answersState: payload.answersState })
    },
    ...overrides,
    saved
  }
}

describe('usePracticeSession（三态反序列化）', () => {
  it.each([
    ['null', null as unknown as Record<string, unknown> | null],
    ['空数组[]', [] as unknown as Record<string, unknown> | null],
    ['缺席(undefined→null)', undefined as unknown as Record<string, unknown> | null]
  ])('answers_state 为%s 时恢复为空状态/空映射', async (_label, answersState) => {
    const adapters = makeAdapters({
      start: async () => ({ questions: [q(1), q(2)], startIndex: 0, answersState })
    })
    const s = usePracticeSession(adapters)

    expect(await s.start('sequential')).toBe(true)

    expect(s.submittedMap.value).toEqual({})
    expect(s.resultMap.value).toEqual({})
    expect(s.correctCount.value).toBe(0)
    expect(s.wrongCount.value).toBe(0)
    expect(s.answers.value).toEqual({})
  })
})

describe('usePracticeSession（断点恢复）', () => {
  it('根据 answers_state 恢复 submittedMap/resultMap/correctCount/answers', async () => {
    const answersState = {
      '1': { is_correct: true, correct_answer: 'A', explanation: 'ok', question_id: 1, user_answer: 'A' },
      '2': { is_correct: false, correct_answer: 'B', explanation: 'no', question_id: 2, user_answer: 'A' }
    }
    const adapters = makeAdapters({
      start: async () => ({ questions: [q(1), q(2), q(3)], startIndex: 0, answersState })
    })
    const s = usePracticeSession(adapters)

    await s.start('sequential')

    expect(s.submittedMap.value).toEqual({ 1: true, 2: true })
    expect(s.resultMap.value[1]).toEqual(answersState['1'])
    expect(s.resultMap.value[2]).toEqual(answersState['2'])
    expect(s.correctCount.value).toBe(1)
    expect(s.wrongCount.value).toBe(1)
    // 字符串作答同时回填 answers 与简答文本 map
    expect(s.answers.value).toEqual({ 1: 'A', 2: 'A' })
  })

  it('从断点起始下标开始，游标定位正确', async () => {
    const adapters = makeAdapters({
      start: async () => ({ questions: [q(1), q(2), q(3)], startIndex: 2, answersState: null })
    })
    const s = usePracticeSession(adapters)

    await s.start('sequential')

    expect(s.currentIdx.value).toBe(2)
    expect(s.submittedMap.value).toEqual({})
  })
})

describe('usePracticeSession（答题/编排）', () => {
  it('提交答案写回 resultMap/submittedMap 并推进统计', async () => {
    const adapters = makeAdapters()
    const s = usePracticeSession(adapters)
    await s.start('sequential')

    s.answers.value[1] = 'A'
    await s.submitAnswer()

    expect(s.submittedMap.value[1]).toBe(true)
    expect(s.resultMap.value[1]?.is_correct).toBe(true)
    expect(s.correctCount.value).toBe(1)
    expect(s.wrongCount.value).toBe(0)
  })

  it('nextQuestion 推进游标并触发进度保存编排', async () => {
    const adapters = makeAdapters()
    const s = usePracticeSession(adapters)
    await s.start('sequential')
    expect(s.currentIdx.value).toBe(0)

    await s.nextQuestion()

    expect(s.currentIdx.value).toBe(1)
    expect(adapters.saved).toEqual([{ mode: 'sequential', index: 1, answersState: {} }])
  })

  it('prevQuestion 回退游标且不再保存进度', async () => {
    const adapters = makeAdapters({ start: async () => ({ questions: [q(1), q(2)], startIndex: 1, answersState: null }) })
    const s = usePracticeSession(adapters)
    await s.start('sequential')

    s.prevQuestion()
    expect(s.currentIdx.value).toBe(0)

    await s.nextQuestion()
    expect(adapters.saved).toEqual([{ mode: 'sequential', index: 1, answersState: {} }])
  })
})

describe('usePracticeSession（退出清空）', () => {
  it('quit 先保存进度再清空会话状态', async () => {
    const adapters = makeAdapters()
    const s = usePracticeSession(adapters)
    await s.start('sequential')
    s.answers.value[1] = 'A'

    await s.quit()

    expect(adapters.saved.at(-1)).toEqual({ mode: 'sequential', index: 0, answersState: {} })
    expect(s.mode.value).toBeNull()
    expect(s.questions.value).toEqual([])
    expect(s.currentIdx.value).toBe(0)
    expect(s.answers.value).toEqual({})
    expect(s.submittedMap.value).toEqual({})
    expect(s.resultMap.value).toEqual({})
    expect(s.correctCount.value).toBe(0)
    expect(s.wrongCount.value).toBe(0)
  })
})

describe('usePracticeSession（序列化 round-trip 经 saveProgress 可观测）', () => {
  it('恢复后再序列化得到一致的独立字面量（key 为题目ID字符串）', async () => {
    const answersState = {
      '1': { is_correct: true, correct_answer: 'A', explanation: 'ok', question_id: 1, user_answer: 'A' },
      '2': { is_correct: false, correct_answer: 'B', explanation: 'no', question_id: 2, user_answer: 'B' }
    }
    const adapters = makeAdapters({
      start: async () => ({ questions: [q(1), q(2)], startIndex: 0, answersState })
    })
    const s = usePracticeSession(adapters)
    await s.start('sequential')

    // 内部 buildAnswersState 序列化结果经 saveCurrentProgress → saveProgress 暴露
    await s.nextQuestion()
    expect(adapters.saved.at(-1)?.answersState).toEqual(answersState)
  })
})

describe('usePracticeSession（start 守卫）', () => {
  it('无题目时返回 false 且不进入会话', async () => {
    const adapters = makeAdapters({ start: async () => null })
    const s = usePracticeSession(adapters)

    expect(await s.start('sequential')).toBe(false)
    expect(s.mode.value).toBeNull()
    expect(s.questions.value).toEqual([])
  })
})

describe('usePracticeSession（single 单题变体：#617 错题重做）', () => {
  it("start('single') 进入单题会话：模式记录为 single，进入钩子触发", async () => {
    const single = [q(7)]
    const adapters = makeAdapters({ start: async () => ({ questions: single, startIndex: 0, answersState: null }) })
    const s = usePracticeSession(adapters)
    const entered: (Question | null)[] = []
    s.onQuestionEnter(q => entered.push(q))

    expect(await s.start('single')).toBe(true)

    expect(s.mode.value).toBe('single')
    expect(s.currentQuestion.value).toEqual(single[0])
    await nextTick()
    expect(entered).toEqual([single[0]])
  })

  it('单题即时提交与练习同管线：payload 带 practice_type=single，结果写回并推进统计，进度经 adapter（可跳过）', async () => {
    const submits: { question_id: number; user_answer: unknown; practice_type: PracticeMode | null }[] = []
    const adapters = makeAdapters({
      start: async () => ({ questions: [q(7)], startIndex: 0, answersState: null }),
      submit: async (payload) => {
        submits.push({ ...payload })
        return {
          is_correct: false,
          correct_answer: 'A',
          explanation: '解析',
          question_id: payload.question_id,
          user_answer: payload.user_answer
        }
      }
    })
    const s = usePracticeSession(adapters)
    await s.start('single')
    s.answers.value[7] = 'B'

    await s.submitAnswer()

    expect(submits).toEqual([{ question_id: 7, user_answer: 'B', practice_type: 'single' }])
    expect(s.submittedMap.value[7]).toBe(true)
    expect(s.resultMap.value[7]?.is_correct).toBe(false)
    expect(s.correctCount.value).toBe(0)
    expect(s.wrongCount.value).toBe(1)
    // 进度保存仍走 adapter（携带 single 形态），单题变体是否落盘由 adapter 决定（错题重做 no-op）
    expect(adapters.saved).toEqual([{ mode: 'single', index: 0, answersState: { '7': expect.any(Object) } }])
  })
})

describe('usePracticeSession（onQuestionEnter 题目进入钩子）', () => {
  it('题目变化按序触发：进入会话 → 切题 → 回退 → 退出（多钩子按注册序执行）', async () => {
    const qs = [q(1), q(2), q(3)]
    const adapters = makeAdapters({ start: async () => ({ questions: qs, startIndex: 0, answersState: null }) })
    const s = usePracticeSession(adapters)
    const entered: (Question | null)[] = []
    const order: string[] = []
    s.onQuestionEnter((q) => { order.push('a'); entered.push(q) })
    s.onQuestionEnter(() => { order.push('b') })

    await s.start('sequential')
    await nextTick()
    expect(order).toEqual(['a', 'b'])
    expect(entered).toEqual([qs[0]])

    await s.nextQuestion()
    await nextTick()
    expect(entered).toEqual([qs[0], qs[1]])

    s.prevQuestion()
    await nextTick()
    expect(entered).toEqual([qs[0], qs[1], qs[0]])

    await s.quit()
    await nextTick()
    expect(entered).toEqual([qs[0], qs[1], qs[0], null])
  })

  it('切题不误触发：提交/存进度/改作答（同题重复渲染）不触发钩子', async () => {
    const qs = [q(1), q(2)]
    const adapters = makeAdapters({ start: async () => ({ questions: qs, startIndex: 0, answersState: null }) })
    const s = usePracticeSession(adapters)
    const entered: (Question | null)[] = []
    s.onQuestionEnter((q) => entered.push(q))

    await s.start('sequential')
    await nextTick()
    expect(entered).toEqual([qs[0]])

    s.answers.value[1] = 'A'
    await s.submitAnswer()
    await nextTick()
    await s.saveCurrentProgress(s.currentIdx.value)
    await nextTick()

    expect(entered).toEqual([qs[0]])
  })

  it('单题场景进入语义：start 后立即触发一次；断点起始下标进入即触发该题', async () => {
    const single = [q(1)]
    const s = usePracticeSession(makeAdapters({ start: async () => ({ questions: single, startIndex: 0, answersState: null }) }))
    const entered: (Question | null)[] = []
    s.onQuestionEnter((q) => entered.push(q))
    await s.start('sequential')
    await nextTick()
    expect(entered).toEqual([single[0]])

    const qs = [q(1), q(2), q(3)]
    const s2 = usePracticeSession(makeAdapters({ start: async () => ({ questions: qs, startIndex: 2, answersState: null }) }))
    const entered2: (Question | null)[] = []
    s2.onQuestionEnter((q) => entered2.push(q))
    await s2.start('sequential')
    await nextTick()
    expect(entered2).toEqual([qs[2]])
  })

  it('解绑函数注销钩子', async () => {
    const s = usePracticeSession(makeAdapters())
    const entered: (Question | null)[] = []
    const unbind = s.onQuestionEnter((q) => entered.push(q))
    unbind()

    await s.start('sequential')
    await nextTick()
    expect(entered).toEqual([])
  })
})