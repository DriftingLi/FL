// 评估状态：当前结果 + 草稿
import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { CreateEvaluationRequest, EvaluationResult, ForkliftType } from '@/types/valuation/evaluation'

export const useEvaluationStore = defineStore('evaluation', () => {
  /** 最近一次评估结果（ResultView / ReportView 共享） */
  const currentResult = ref<EvaluationResult | null>(null)
  /** 最近一次评估的 id（用于跳转报告页） */
  const currentId = ref<number | null>(null)
  /** 最近一次评估的类型（用于 ReportView 显示"电动/内燃"） */
  const currentType = ref<ForkliftType | null>(null)
  /** 表单草稿（防丢失） */
  const draftForm = ref<Partial<CreateEvaluationRequest> | null>(null)

  /** 写入评估结果 */
  function setResult(r: EvaluationResult, id: number, type: ForkliftType) {
    currentResult.value = r
    currentId.value = id
    currentType.value = type
  }

  /** 保存草稿 */
  function saveDraft(form: Partial<CreateEvaluationRequest>) {
    draftForm.value = { ...form }
  }

  /** 清空全部 */
  function clearAll() {
    currentResult.value = null
    currentId.value = null
    currentType.value = null
    draftForm.value = null
  }

  return { currentResult, currentId, currentType, draftForm, setResult, saveDraft, clearAll }
})
