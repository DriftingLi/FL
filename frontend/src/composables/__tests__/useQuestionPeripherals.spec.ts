// useQuestionPeripherals：答题外围交互 module 的接口级测试。
// 断言 external behavior：三 adapter（favorite/knowledge/duration）各自的加载回填、
// 失败降级、切题重置时机（'result' 出结果查 vs 'enter' 进题预取两种知识点触发形态）、
// 收藏切换返回值、未注入 adapter 的 no-op 语义。
// seam：composable 接口——session 用真实 usePracticeSession + 内存 adapter（钩子机制
// 端到端可见），外围 adapter 用 vi.fn() stub，不触达 API 层。
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { nextTick } from 'vue'
import { usePracticeSession } from '@/composables/usePracticeSession'
import type { PracticeSessionAdapters } from '@/composables/usePracticeSession'
import { useQuestionPeripherals } from '@/composables/useQuestionPeripherals'
import type { Question } from '@/types/question'

function q(id: number): Question {
  return { id, type: 'single_choice', content: `题${id}` } as Question
}

/** 真实会话 + 内存 adapter：进入/提交/存进度全内存，钩子经 Vue watch 触发 */
function makeSession(qs: Question[], overrides: Partial<PracticeSessionAdapters> = {}) {
  return usePracticeSession({
    start: async () => ({ questions: qs, startIndex: 0, answersState: null }),
    submit: async (payload) => ({
      is_correct: true,
      correct_answer: 'A',
      explanation: '解析',
      question_id: payload.question_id,
      user_answer: payload.user_answer
    }),
    saveProgress: async () => {},
    ...overrides
  })
}

function makeFavorite() {
  return {
    check: vi.fn().mockResolvedValue({ favorited: false, favorite_id: 0 }),
    add: vi.fn().mockResolvedValue({ favorite_id: 9 }),
    remove: vi.fn().mockResolvedValue(null)
  }
}

describe('useQuestionPeripherals（favorite：进题查状态 + 切换）', () => {
  let qs: Question[]
  beforeEach(() => { qs = [q(1), q(2)] })

  it('进题查收藏状态并回填 favorited', async () => {
    const favorite = makeFavorite()
    favorite.check.mockResolvedValue({ favorited: true, favorite_id: 7 })
    const s = makeSession(qs)
    const p = useQuestionPeripherals(s, { favorite })

    await s.start('sequential')
    await nextTick()

    expect(favorite.check).toHaveBeenCalledWith(1)
    expect(p.favorited.value).toBe(true)
  })

  it('收藏查询失败降级为未收藏（不阻断）', async () => {
    const favorite = makeFavorite()
    favorite.check.mockRejectedValue(new Error('network'))
    const s = makeSession(qs)
    const p = useQuestionPeripherals(s, { favorite })

    await s.start('sequential')
    await nextTick()

    expect(p.favorited.value).toBe(false)
  })

  it('toggleFavorite：未收藏 → add 并返回 added；已收藏 → remove 并返回 removed', async () => {
    const favorite = makeFavorite()
    favorite.check.mockResolvedValue({ favorited: true, favorite_id: 7 })
    const s = makeSession(qs)
    const p = useQuestionPeripherals(s, { favorite })
    await s.start('sequential')
    await nextTick()

    expect(await p.toggleFavorite()).toBe('removed')
    expect(favorite.remove).toHaveBeenCalledWith(7)
    expect(p.favorited.value).toBe(false)

    expect(await p.toggleFavorite()).toBe('added')
    expect(favorite.add).toHaveBeenCalledWith(1)
    expect(p.favorited.value).toBe(true)
  })

  it('toggleFavorite：add 失败返回 null 且状态不变', async () => {
    const favorite = makeFavorite()
    favorite.add.mockRejectedValue(new Error('fail'))
    const s = makeSession(qs)
    const p = useQuestionPeripherals(s, { favorite })
    await s.start('sequential')
    await nextTick()

    expect(await p.toggleFavorite()).toBeNull()
    expect(p.favorited.value).toBe(false)
  })

  it('切题重置并重查新题收藏', async () => {
    const favorite = makeFavorite()
    favorite.check
      .mockResolvedValueOnce({ favorited: true, favorite_id: 7 })
      .mockResolvedValueOnce({ favorited: false, favorite_id: 0 })
    const s = makeSession(qs)
    const p = useQuestionPeripherals(s, { favorite })
    await s.start('sequential')
    await nextTick()
    expect(p.favorited.value).toBe(true)

    await s.nextQuestion()
    await nextTick()

    expect(favorite.check).toHaveBeenLastCalledWith(2)
    expect(p.favorited.value).toBe(false)
  })

  it('会话退出后无当前题：toggleFavorite 返回 null', async () => {
    const favorite = makeFavorite()
    const s = makeSession(qs)
    const p = useQuestionPeripherals(s, { favorite })
    await s.start('sequential')
    await nextTick()
    await s.quit()
    await nextTick()

    expect(await p.toggleFavorite()).toBeNull()
  })

  it('未注入 favorite adapter：toggleFavorite 为 no-op，favorited 恒 false', async () => {
    const s = makeSession(qs)
    const p = useQuestionPeripherals(s, {})
    await s.start('sequential')
    await nextTick()

    expect(await p.toggleFavorite()).toBeNull()
    expect(p.favorited.value).toBe(false)
  })
})

describe('useQuestionPeripherals（knowledge：出结果查 / 进题预取）', () => {
  let qs: Question[]
  beforeEach(() => { qs = [q(1), q(2)] })

  it("trigger 'result'：进题不查；提交后按结果 question_id 查询并回填", async () => {
    const list = vi.fn().mockResolvedValue([{ id: 1, name: '考点A' }])
    const s = makeSession(qs)
    const p = useQuestionPeripherals(s, { knowledge: { trigger: 'result', list } })
    await s.start('sequential')
    await nextTick()
    expect(list).not.toHaveBeenCalled()

    s.answers.value[1] = 'A'
    await s.submitAnswer()
    await nextTick()

    expect(list).toHaveBeenCalledTimes(1)
    expect(list).toHaveBeenCalledWith(1)
    expect(p.knowledgeTags.value).toEqual([{ id: 1, name: '考点A' }])
  })

  it("trigger 'result'：切到无结果题目（未答/退出）清空 tags", async () => {
    const list = vi.fn().mockResolvedValue([{ id: 1, name: '考点A' }])
    const s = makeSession(qs)
    const p = useQuestionPeripherals(s, { knowledge: { trigger: 'result', list } })
    await s.start('sequential')
    s.answers.value[1] = 'A'
    await s.submitAnswer()
    await nextTick()
    expect(p.knowledgeTags.value).toEqual([{ id: 1, name: '考点A' }])

    await s.nextQuestion()
    await nextTick()
    expect(p.knowledgeTags.value).toEqual([])
  })

  it("trigger 'result'：断点恢复进入已答题目即按结果题目查", async () => {
    const list = vi.fn().mockResolvedValue([])
    const s = makeSession(qs, {
      start: async () => ({
        questions: qs,
        startIndex: 0,
        answersState: {
          '1': { is_correct: true, correct_answer: 'A', explanation: 'ok', question_id: 1, user_answer: 'A' }
        }
      })
    })
    useQuestionPeripherals(s, { knowledge: { trigger: 'result', list } })

    await s.start('sequential')
    await nextTick()

    expect(list).toHaveBeenCalledWith(1)
  })

  it("trigger 'result'：查询失败置空 tags", async () => {
    const list = vi.fn().mockRejectedValue(new Error('fail'))
    const s = makeSession(qs)
    const p = useQuestionPeripherals(s, { knowledge: { trigger: 'result', list } })
    await s.start('sequential')
    s.answers.value[1] = 'A'
    await s.submitAnswer()
    await nextTick()

    expect(p.knowledgeTags.value).toEqual([])
  })

  it("trigger 'enter'：进题预取；切题清空再取；出结果不重复查", async () => {
    const list = vi.fn().mockResolvedValue([{ id: 1, name: 'K' }])
    const s = makeSession(qs)
    const p = useQuestionPeripherals(s, { knowledge: { trigger: 'enter', list } })
    await s.start('sequential')
    await nextTick()

    expect(list).toHaveBeenCalledTimes(1)
    expect(list).toHaveBeenCalledWith(1)
    expect(p.knowledgeTags.value).toEqual([{ id: 1, name: 'K' }])

    await s.nextQuestion()
    await nextTick()
    expect(list).toHaveBeenCalledTimes(2)
    expect(list).toHaveBeenLastCalledWith(2)

    s.answers.value[2] = 'A'
    await s.submitAnswer()
    await nextTick()
    expect(list).toHaveBeenCalledTimes(2)
  })

  it("trigger 'enter'：查询失败置空 tags", async () => {
    const list = vi.fn().mockRejectedValue(new Error('fail'))
    const s = makeSession(qs)
    const p = useQuestionPeripherals(s, { knowledge: { trigger: 'enter', list } })
    await s.start('sequential')
    await nextTick()

    expect(p.knowledgeTags.value).toEqual([])
  })

  it('未注入 knowledge adapter：tags 恒空', async () => {
    const s = makeSession(qs)
    const p = useQuestionPeripherals(s, {})
    await s.start('sequential')
    s.answers.value[1] = 'A'
    await s.submitAnswer()
    await nextTick()

    expect(p.knowledgeTags.value).toEqual([])
  })
})

describe('useQuestionPeripherals（duration：进题起表 / 提交前取用时）', () => {
  let qs: Question[]
  beforeEach(() => { qs = [q(1), q(2)] })
  afterEach(() => { vi.useRealTimers() })

  it('进题起表；recordDuration 记录用时秒数；切题重置', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    const s = makeSession(qs)
    const p = useQuestionPeripherals(s, { duration: true })
    await s.start('sequential')
    await nextTick()
    expect(p.lastDuration.value).toBeUndefined()

    vi.setSystemTime(Date.now() + 5000)
    p.recordDuration()
    expect(p.lastDuration.value).toBe(5)

    await s.nextQuestion()
    await nextTick()
    expect(p.lastDuration.value).toBeUndefined()
  })

  it('未启用 duration：recordDuration 为 no-op', async () => {
    const s = makeSession(qs)
    const p = useQuestionPeripherals(s, {})
    await s.start('sequential')
    await nextTick()

    p.recordDuration()
    expect(p.lastDuration.value).toBeUndefined()
  })
})
