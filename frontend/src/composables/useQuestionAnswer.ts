// 答题交互共享模块：选项切换（多选 toggle）、对/错模板、作答状态序列化。
// 练习 / 模拟考试 / 定级考试 / 错题重做共享同一交互形态；持久化由各页面回调注入，不在此统一。
// 倒计时见 useCountdown。
import { ref } from 'vue'
import type { Ref } from 'vue'

// ===== 对/错模板 =====
export const TRUE_FALSE_OPTIONS: ReadonlyArray<{ key: string; label: string }> = [
  { key: '对', label: '正确' },
  { key: '错', label: '错误' }
]

/** 题目选项 → 渲染选项记录（判断题渲染对/错模板） */
export function buildQuestionOptions(question: { type?: string; options?: Record<string, string> | null }): Record<string, string> {
  if (question?.type === 'true_false') return { 对: '正确', 错: '错误' }
  return question?.options || {}
}

// ===== 选项切换（纯函数） =====

/** 切换选项：多选在数组中 toggle，单选直接替换为当前 key */
export function toggleAnswer(prev: unknown, key: string | number, multiChoice: boolean): unknown {
  if (multiChoice) {
    const arr = Array.isArray(prev) ? [...prev] : []
    const idx = arr.indexOf(key)
    if (idx > -1) arr.splice(idx, 1)
    else arr.push(key)
    return arr
  }
  return key
}

/** 判断 key 是否被选中 */
export function isAnswerSelected(answer: unknown, key: string | number, multiChoice: boolean): boolean {
  if (answer === undefined || answer === null) return false
  if (multiChoice) return Array.isArray(answer) && answer.includes(key)
  return answer === key
}

/** 作答是否为空（未选/空串/空数组） */
export function isAnswerEmpty(answer: unknown): boolean {
  if (answer === undefined || answer === null || answer === '') return true
  return Array.isArray(answer) && answer.length === 0
}

// ===== 作答状态序列化 / 反序列化（纯函数） =====

/** 序列化：{数字/字符串题目ID: 答案} → {字符串题目ID: 答案}（可持久化） */
export function serializeAnswers(answers: Record<number | string, unknown>): Record<string, unknown> {
  const state: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(answers)) {
    if (value !== undefined) state[key] = value
  }
  return state
}

/** 反序列化：{字符串题目ID: 答案} → {数字题目ID: 答案}（round-trip 恢复） */
export function deserializeAnswers(state: Record<string, unknown>): Record<number, unknown> {
  const answers: Record<number, unknown> = {}
  for (const [key, value] of Object.entries(state)) {
    const qid = Number(key)
    if (!Number.isNaN(qid) && qid > 0) answers[qid] = value
  }
  return answers
}

// ===== 答题会话 composable：选项状态 + 切换 =====

export function useQuestionAnswer() {
  const answers: Ref<Record<number | string, unknown>> = ref({})

  /** 切换当前题目的选项（多选 toggle / 单选替换） */
  function toggleOption(qid: number | string, key: string | number, multiChoice: boolean): void {
    answers.value[qid] = toggleAnswer(answers.value[qid], key, multiChoice)
  }

  /** 判断某 key 是否被选中 */
  function isOptionSelected(qid: number | string, key: string | number, multiChoice: boolean): boolean {
    return isAnswerSelected(answers.value[qid], key, multiChoice)
  }

  /** 整表重置/恢复（断点续练恢复、退出清空） */
  function reset(newAnswers: Record<number | string, unknown> = {}): void {
    answers.value = { ...newAnswers }
  }

  return { answers, toggleOption, isOptionSelected, reset }
}
