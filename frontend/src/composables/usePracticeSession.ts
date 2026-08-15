// usePracticeSession：练习会话状态机 module（题库练习页的会话编排收敛）。
// deep module：小 interface（start / submit / saveProgress 三个持久化 adapter）
// 藏大量 implementation：mode/currentIdx/questions/answers/submittedMap/resultMap/
// correctCount 会话状态、answers_state 三态反序列化（null/[]/absent）、断点恢复、
// 退出清空、buildAnswersState 序列化 round-trip、游标推进 + 进度保存编排。
// 后端调用（startSequential/startFree/startTagPractice/saveProgress/submitAnswer）
// 与进度 key 语义由调用方注入 adapter，本 composable 不 import 任何 api。
// 三个 start 模式（顺序/自由/标签）通过 mode 参数区分，adapter 内解析各模式参数。
import { ref, computed } from 'vue'
import type { Ref } from 'vue'
import type { Question, SubmitResult } from '@/types/question'
import {
  buildQuestionOptions,
  serializeAnswers,
  deserializeAnswers,
  useQuestionAnswer
} from './useQuestionAnswer'

/** 练习模式：顺序 / 自由（随机或专项）/ 标签 */
export type PracticeMode = 'sequential' | 'free' | 'tag'

/** 进入/续练某模式时 adapter 返回的数据（questions + 断点进度） */
export interface PracticeStartData {
  questions: Question[]
  /** 断点续练起始下标 */
  startIndex: number
  /** 已持久化答题状态（answers_state），可为 null / 空数组 / 缺席（调用方归一为 null） */
  answersState: Record<string, unknown> | null
}

/** 单题提交载荷（adapter 据此调后端 submitAnswer） */
export interface PracticeSubmitPayload {
  question_id: number
  user_answer: unknown
  practice_type: PracticeMode | null
}

/** 进度保存载荷（index 为游标，answersState 由 composable 序列化） */
export interface PracticeSavePayload {
  mode: PracticeMode
  index: number
  total: number
  answersState: Record<string, unknown>
}

/** 三个持久化 adapter：进入/续练、提交单题、保存进度（API 与进度 key 语义在 adapter 内） */
export interface PracticeSessionAdapters {
  /** 进入/续练：拉取题目并解析断点进度；无题目/失败返回 null */
  start: (mode: PracticeMode) => Promise<PracticeStartData | null>
  /** 提交单题答案并判定，返回结果；失败返回 null */
  submit: (payload: PracticeSubmitPayload) => Promise<SubmitResult | null>
  /** 保存进度与答题状态；无断点模式（随机）由 adapter 内部跳过，失败静默 */
  saveProgress: (payload: PracticeSavePayload) => Promise<void>
}

export function usePracticeSession(adapters: PracticeSessionAdapters) {
  const mode = ref<PracticeMode | null>(null)
  const questions = ref<Question[]>([])
  const currentIdx = ref(0)
  const submittedMap = ref<Record<number, boolean>>({})
  const resultMap = ref<Record<number, SubmitResult>>({})
  const correctCount = ref(0)
  const wrongCount = ref(0)
  const textAnswerMap = ref<Record<number, string>>({})
  const loading = ref(false)

  const { answers, toggleOption, reset: resetAnswers } = useQuestionAnswer()

  // ===== 当前题目派生状态 =====
  const currentQuestion = computed(() => questions.value[currentIdx.value] || ({} as Question))
  // 当前题目的简答文本（v-model 双向绑定到 Map）
  const textAnswer: Ref<string> = computed({
    get: () => (currentQuestion.value.id ? textAnswerMap.value[currentQuestion.value.id] || '' : ''),
    set: (v) => {
      if (currentQuestion.value.id) textAnswerMap.value[currentQuestion.value.id] = v
    }
  })
  // 当前题目是否已提交
  const submitted = computed(() => (currentQuestion.value.id ? !!submittedMap.value[currentQuestion.value.id] : false))
  // 当前题目的解析结果
  const lastResult = computed(() =>
    currentQuestion.value.id ? resultMap.value[currentQuestion.value.id] || ({} as SubmitResult) : ({} as SubmitResult)
  )
  // 当前题目渲染用选项（判断题渲染对/错模板）
  const currentOptions = computed(() => buildQuestionOptions(currentQuestion.value))
  // 当前题目已选中的选项 keys
  const selectedOptionKeys = computed((): (string | number)[] => {
    const q = currentQuestion.value
    if (!q || !q.id) return []
    const ans = answers.value[q.id]
    if (ans === undefined || ans === null) return []
    if (q.type === 'multi_choice') return Array.isArray(ans) ? (ans as (string | number)[]) : []
    return [ans as string | number]
  })
  const canSubmit = computed(() => {
    const q = currentQuestion.value
    if (!q || !q.id) return false
    if (q.type === 'short_answer') return textAnswer.value.trim() !== ''
    const ans = answers.value[q.id]
    if (ans === undefined || ans === null) return false
    if (Array.isArray(ans)) return ans.length > 0
    return ans !== ''
  })

  // ===== 会话状态机 =====

  /** 构建可序列化的答题状态对象（key 为题目ID字符串） */
  function buildAnswersState(): Record<string, unknown> {
    return serializeAnswers(resultMap.value)
  }

  /** 从后端答题状态恢复 submittedMap/resultMap/correctCount/wrongCount/answers */
  function restoreState(answersState: Record<string, unknown> | null) {
    // 三态（null/空数组/缺席→null）统一视为无断点，保持空状态
    if (!answersState || Object.keys(answersState).length === 0) return
    const restored = deserializeAnswers(answersState)
    const newAnswers: Record<number, unknown> = {}
    const newSubmittedMap: Record<number, boolean> = {}
    const newResultMap: Record<number, SubmitResult> = {}
    const newTextAnswerMap: Record<number, string> = {}
    let correct = 0
    let wrong = 0
    for (const [key, val] of Object.entries(restored)) {
      const qid = Number(key)
      if (!qid) continue
      const result = val as SubmitResult
      newResultMap[qid] = result
      newSubmittedMap[qid] = true
      if (result.user_answer !== undefined && result.user_answer !== null) {
        newAnswers[qid] = result.user_answer
        if (typeof result.user_answer === 'string') {
          newTextAnswerMap[qid] = result.user_answer
        }
      }
      if (result.is_correct === true) correct++
      else if (result.is_correct === false) wrong++
    }
    resetAnswers(newAnswers)
    submittedMap.value = newSubmittedMap
    resultMap.value = newResultMap
    textAnswerMap.value = newTextAnswerMap
    correctCount.value = correct
    wrongCount.value = wrong
  }

  /** 整段会话重置到起始下标（清空答题状态） */
  function resetSession(startIdx: number) {
    currentIdx.value = startIdx
    resetAnswers()
    textAnswerMap.value = {}
    submittedMap.value = {}
    resultMap.value = {}
    correctCount.value = 0
    wrongCount.value = 0
  }

  /** 进入/续练某模式；返回是否成功进入会话 */
  async function start(startMode: PracticeMode): Promise<boolean> {
    loading.value = true
    try {
      const data = await adapters.start(startMode)
      if (!data) return false
      questions.value = data.questions
      mode.value = startMode
      resetSession(data.startIndex)
      restoreState(data.answersState)
      return true
    } finally {
      loading.value = false
    }
  }

  /** 保存当前进度（游标 + 答题状态）；adapter 处理 key 语义与失败降级 */
  async function saveCurrentProgress(index: number) {
    if (!mode.value) return
    try {
      await adapters.saveProgress({
        mode: mode.value,
        index,
        total: questions.value.length,
        answersState: buildAnswersState()
      })
    } catch (e) {
      // 保存失败不阻断练习
    }
  }

  /** 提交当前题目答案并判定，写回 resultMap/submittedMap 并推进统计 */
  async function submitAnswer() {
    const q = currentQuestion.value
    if (!canSubmit.value) return
    const userAnswer: unknown = q.type === 'short_answer' ? textAnswer.value : answers.value[q.id]
    try {
      const res = await adapters.submit({
        question_id: q.id,
        user_answer: userAnswer,
        practice_type: mode.value
      })
      resultMap.value[q.id] = res || ({} as SubmitResult)
      submittedMap.value[q.id] = true
      if (resultMap.value[q.id].is_correct) correctCount.value++
      else wrongCount.value++
      // 提交后持久化答题状态（游标不变，仅更新 answers_state）
      await saveCurrentProgress(currentIdx.value)
    } catch (e) {
      // 错误已由 adapter/拦截器处理
    }
  }

  /** 下一题：游标推进 + 保存进度 + 答题状态 */
  async function nextQuestion() {
    currentIdx.value++
    await saveCurrentProgress(currentIdx.value)
  }

  /** 上一题：回到上一题，状态由 Map 自动恢复（进度不回退） */
  function prevQuestion() {
    if (currentIdx.value > 0) currentIdx.value--
  }

  /** 退出并保存进度后清空会话状态 */
  async function quit() {
    if (mode.value) await saveCurrentProgress(currentIdx.value)
    backToEntry()
  }

  /** 返回入口：清空会话状态 */
  function backToEntry() {
    mode.value = null
    questions.value = []
    currentIdx.value = 0
    resetAnswers()
    textAnswerMap.value = {}
    submittedMap.value = {}
    resultMap.value = {}
    correctCount.value = 0
    wrongCount.value = 0
  }

  return {
    // 状态
    mode,
    questions,
    currentIdx,
    answers,
    toggleOption,
    submittedMap,
    resultMap,
    correctCount,
    wrongCount,
    textAnswerMap,
    loading,
    // 推导
    currentQuestion,
    textAnswer,
    submitted,
    lastResult,
    currentOptions,
    selectedOptionKeys,
    canSubmit,
    // 行为
    start,
    submitAnswer,
    nextQuestion,
    prevQuestion,
    quit,
    backToEntry,
    saveCurrentProgress,
    buildAnswersState,
    restoreState,
    resetSession
  }
}