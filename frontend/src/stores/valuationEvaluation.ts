// 评估状态：当前结果 + 草稿
// 重构说明：移除 ForkliftType（统一表单不再区分电动/内燃）
import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { CreateEvaluationRequest, EvaluationResult } from '@/types/valuation/evaluation'

export const useEvaluationStore = defineStore('evaluation', () => {
  /** 最近一次评估结果（ResultView / ReportView 共享） */
  const currentResult = ref<EvaluationResult | null>(null)
  /** 最近一次评估的 id（用于跳转报告页） */
  const currentId = ref<number | null>(null)
  /** 表单草稿（防丢失） */
  const draftForm = ref<Partial<CreateEvaluationRequest> | null>(null)

  /** 写入评估结果 */
  function setResult(r: EvaluationResult, id: number) {
    currentResult.value = r
    currentId.value = id
  }

  /** 保存草稿 */
  function saveDraft(form: Partial<CreateEvaluationRequest>) {
    draftForm.value = { ...form }
  }

  /** 清空全部 */
  function clearAll() {
    currentResult.value = null
    currentId.value = null
    draftForm.value = null
  }

  return { currentResult, currentId, draftForm, setResult, saveDraft, clearAll }
})
