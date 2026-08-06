// 评估状态：当前结果 + 提交 + 详情拉取（评估旅程状态收敛于此）
// 重构说明：移除 ForkliftType（统一表单不再区分电动/内燃）；
// 提交后拉取详情（含输入字段/建议/λ 锁定值），结果页刷新可按路由 id 兜底恢复
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { createEvaluation, getEvaluationDetail } from '@/api/valuation/evaluation'
import type { CreateEvaluationRequest, EvaluationDetailResponse } from '@/types/valuation/evaluation'

export const useEvaluationStore = defineStore('evaluation', () => {
  /** 最近一次评估结果（详情形态：含输入字段 + 建议 + λ 锁定值） */
  const currentResult = ref<EvaluationDetailResponse | null>(null)
  /** 最近一次评估的 id（用于跳转报告页） */
  const currentId = ref<number | null>(null)
  /** 提交中（按钮禁用态，composable 不再维护第二份状态） */
  const submitting = ref(false)

  /** 写入评估结果 */
  function setResult(r: EvaluationDetailResponse, id: number) {
    currentResult.value = r
    currentId.value = id
  }

  /** 提交评估：创建 → 拉取详情（含输入字段，供结果页展示）→ 落 store，返回 id */
  async function submitEvaluation(payload: CreateEvaluationRequest): Promise<number | null> {
    if (submitting.value) return null
    submitting.value = true
    try {
      const result = await createEvaluation(payload)
      const detail = await getEvaluationDetail(result.id)
      currentResult.value = detail
      currentId.value = detail.id
      return detail.id
    } finally {
      submitting.value = false
    }
  }

  /** 按 id 拉取详情（刷新恢复 / 直达结果页），成功返回 true */
  async function fetchDetail(id: number): Promise<boolean> {
    try {
      const detail = await getEvaluationDetail(id)
      currentResult.value = detail
      currentId.value = detail.id
      return true
    } catch {
      return false
    }
  }

  /** 清空全部 */
  function clearAll() {
    currentResult.value = null
    currentId.value = null
  }

  return { currentResult, currentId, submitting, setResult, submitEvaluation, fetchDetail, clearAll }
})
