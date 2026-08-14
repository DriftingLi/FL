// useExamSession：答题会话编排 module 的接口级测试（顺序约束 + 断点续传 + reset）。
// seam：composable 接口——三个持久化 adapter 用内存 stub，不触达 API 层。
import { describe, it, expect } from 'vitest'
import { useExamSession } from '@/composables/useExamSession'
import type { ExamSessionAdapters, ExamSubmitPayload } from '@/composables/useExamSession'
import type { Question } from '@/types/question'

function q(id: number): Question {
  return { id, type: 'single_choice', content: `题${id}` } as Question
}

function makeAdapters(overrides: Partial<ExamSessionAdapters> = {}): ExamSessionAdapters & {
  saveCalls: number
  saveSeq: string[]
} {
  const saveSeq: string[] = []
  return {
    enter: async () => ({
      questions: [q(1), q(2), q(3)],
      answers: { '1': 'A', '2': 'B' },
      remaining_time: 120
    }),
    save: async () => {
      saveSeq.push('save')
    },
    submit: async () => {
      saveSeq.push('submit')
      return { ok: true }
    },
    ...overrides,
    saveCalls: saveSeq.length,
    saveSeq
  }
}

const payload: ExamSubmitPayload = {
  is_timeout: false,
  answers: { 1: 'A', 2: 'B', 3: 'C' },
  remaining_time: 60
}

describe('useExamSession（会话编排收敛）', () => {
  it('进入后恢复答案并定位断点续传下标（第一个未答题）', async () => {
    const adapters = makeAdapters()
    const s = useExamSession(adapters)

    await s.start()

    expect(s.inExam.value).toBe(true)
    expect(s.questions.value.length).toBe(3)
    expect(s.currentIdx.value).toBe(2) // 题1/题2 已答，题3 未答
    expect(s.remainingTime.value).toBe(0) // 倒计时由 shell begin 驱动，composable 不直接赋值
  })

  it('submit 前先 save（顺序约束）', async () => {
    const adapters = makeAdapters()
    const s = useExamSession(adapters)
    await s.start()

    await s.submit(payload)

    expect(adapters.saveSeq).toEqual(['save', 'submit'])
  })

  it('save 失败静默，仍继续 submit', async () => {
    const seq: string[] = []
    const adapters = makeAdapters({
      save: async () => {
        seq.push('save')
        throw new Error('boom')
      },
      submit: async () => {
        seq.push('submit')
        return { ok: true }
      }
    })
    const s = useExamSession(adapters)
    await s.start()

    await s.submit(payload)

    expect(seq).toEqual(['save', 'submit'])
  })

  it('reset 清空会话状态', async () => {
    const adapters = makeAdapters()
    const s = useExamSession(adapters)
    await s.start()

    s.reset()

    expect(s.inExam.value).toBe(false)
    expect(s.questions.value).toEqual([])
    expect(s.answers.value).toEqual({})
    expect(s.currentIdx.value).toBe(0)
    expect(s.remainingTime.value).toBe(0)
  })

  it('enter 返回 null 时不进入会话', async () => {
    const adapters = makeAdapters({ enter: async () => null })
    const s = useExamSession(adapters)

    await s.start()

    expect(s.inExam.value).toBe(false)
    expect(s.questions.value).toEqual([])
  })
})
