// usePracticeSession：练习会话状态机 module 的接口级测试。
// 断言 external behavior：三态反序列化（null/[]/absent）、断点恢复、
// 退出清空、buildAnswersState round-trip、游标推进/进度保存编排。
// seam：composable 接口——三个 adapter（start/submit/saveProgress）用内存 stub，
// API 与进度 key 语义由 adapter 注入，本测试不触达 API 层。
import { describe, it, expect } from 'vitest'
import { usePracticeSession } from '@/composables/usePracticeSession'
import type { PracticeSessionAdapters, PracticeMode } from '@/composables/usePracticeSession'
import type { Question } from '@/types/question'

function q(id: number): Question {
  return { id, type: 'single_choice', content: `题${id}` } as Question
}

function makeAdapters(
  overrides: Partial<PracticeSessionAdapters> = {}
): PracticeSessionAdapters & { saved: { mode: PracticeMode; index: number }[] } {
  const saved: { mode: PracticeMode; index: number }[] = []
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
      saved.push({ mode: payload.mode, index: payload.index })
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
    expect(s.resultMap.value[1].is_correct).toBe(true)
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
    expect(adapters.saved).toEqual([{ mode: 'sequential', index: 1 }])
  })

  it('prevQuestion 回退游标且不再保存进度', async () => {
    const adapters = makeAdapters({ start: async () => ({ questions: [q(1), q(2)], startIndex: 1, answersState: null }) })
    const s = usePracticeSession(adapters)
    await s.start('sequential')

    s.prevQuestion()
    expect(s.currentIdx.value).toBe(0)

    await s.nextQuestion()
    expect(adapters.saved).toEqual([{ mode: 'sequential', index: 1 }])
  })
})

describe('usePracticeSession（退出清空）', () => {
  it('quit 先保存进度再清空会话状态', async () => {
    const adapters = makeAdapters()
    const s = usePracticeSession(adapters)
    await s.start('sequential')
    s.answers.value[1] = 'A'

    await s.quit()

    expect(adapters.saved.at(-1)).toEqual({ mode: 'sequential', index: 0 })
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

describe('usePracticeSession（buildAnswersState round-trip）', () => {
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

    expect(s.buildAnswersState()).toEqual(answersState)
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