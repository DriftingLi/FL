// useExamSession：答题会话编排 module（ADR-0010 后端 answering_session 的前端对称收敛）。
// deep module：小 interface（enter/save/submit 三个持久化 adapter）
// 藏大量 implementation（inExam/remainingTime/currentIdx/shellRef 会话状态、
// submit 前先 save 的顺序约束、断点续传的 resume index、reset）。
// MockExam / LevelExam 只注入三个持久化 adapter，API 与 id 差异落在 adapter 里。
import { ref, nextTick } from 'vue'
import type { Question } from '@/types/question'
import { isAnswerEmpty } from './useQuestionAnswer'
import type AnsweringSessionShell from '@/components/student/AnsweringSessionShell.vue'

/** 进入/开始考试的返回（questions + 恢复答案 + 剩余时间） */
export interface ExamEnterData {
  questions: Question[]
  answers?: Record<string, unknown>
  remaining_time: number
}

/** 交卷载荷（AnsweringSessionShell emit 的 submit 参数） */
export interface ExamSubmitPayload {
  is_timeout: boolean
  answers: Record<number, unknown>
  remaining_time: number
}

/** 三个持久化 adapter：进入 / 保存进度 / 交卷（id 由 adapter 闭包捕获） */
export interface ExamSessionAdapters {
  /** 进入/开始考试，返回题目、恢复答案与剩余时间；失败返回 null */
  enter: () => Promise<ExamEnterData | null>
  /** 保存进度（断点续传/自动保存） */
  save: (answers: Record<number, unknown>, remainingTime: number) => Promise<void>
  /** 交卷，返回结果（页面据此展示） */
  submit: (payload: ExamSubmitPayload) => Promise<unknown>
}

export function useExamSession(adapters: ExamSessionAdapters) {
  const inExam = ref(false)
  const remainingTime = ref(0)
  const currentIdx = ref(0)
  const questions = ref<Question[]>([])
  const answers = ref<Record<number, unknown>>({})
  const shellRef = ref<InstanceType<typeof AnsweringSessionShell> | null>(null)

  /** 断点续传：定位第一个未作答题目的下标（全部已答则 0） */
  function findResumeIndex(qs: { id: number }[], ans: Record<string, unknown>): number {
    for (let i = 0; i < qs.length; i++) {
      if (isAnswerEmpty(ans[qs[i].id])) return i
    }
    return 0
  }

  /** 进入/开始考试：拉取题目 → 恢复答案 → 切进卷面 → 起倒计时 → 定位断点下标 */
  async function start() {
    const res = await adapters.enter()
    if (!res) return
    // 空卷兜底（#702）：成功但题目为空不进卷，抛错由调用方提示——否则渲染空壳 currentQ={} 即空白页
    if (!res.questions || res.questions.length === 0) {
      throw new Error('题库暂无可用的题目')
    }
    questions.value = res.questions
    answers.value = { ...(res.answers || {}) }
    // 先切进卷面再起倒计时：shellRef 在 v-if="inExam" 下挂载，顺序反了 begin 恒为 null、倒计时永远不起（#702）
    inExam.value = true
    currentIdx.value = findResumeIndex(res.questions, res.answers || {})
    await nextTick()
    shellRef.value?.begin(res.remaining_time)
  }

  /** 保存进度（自动保存/交卷前兜底），失败静默（拦截器已提示） */
  async function saveProgress() {
    try {
      await adapters.save(answers.value, remainingTime.value)
    } catch (e) {}
  }

  /** 交卷：先 save 再 submit（顺序约束），返回 submit 结果 */
  async function submit(payload: ExamSubmitPayload): Promise<unknown> {
    try {
      await saveProgress()
    } catch (e) {}
    return adapters.submit(payload)
  }

  /** 退出清空（交卷后 / 返回） */
  function reset() {
    inExam.value = false
    questions.value = []
    answers.value = {}
    currentIdx.value = 0
    remainingTime.value = 0
  }

  return { inExam, remainingTime, currentIdx, questions, answers, shellRef, start, saveProgress, submit, reset }
}
