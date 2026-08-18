// 评估状态：残值评估结果旅程的薄 adapter（ADR-0015）。
// 状态机由 createValuationJourney 唯一实现；本 store 只注入持久化 adapter 与旧接口兼容语义。
import { defineStore } from 'pinia'
import { computed } from 'vue'
import { createEvaluation, getEvaluationDetail } from '@/api/valuation/evaluation'
import type { CreateEvaluationRequest, EvaluationDetailResponse } from '@/types/valuation/evaluation'
import { createValuationJourney } from '@/composables/createValuationJourney'

export const useEvaluationStore = defineStore('evaluation', () => {
  const journey = createValuationJourney<CreateEvaluationRequest, EvaluationDetailResponse, EvaluationDetailResponse>(
    {
      submit: createEvaluation,
      fetch: getEvaluationDetail
    },
    {
      idOfSubmit: r => r.id,
      idOfDetail: d => d.id,
      fetchWritesResult: true,
      submitErrorFallback: '提交失败',
      loadErrorFallback: '加载失败'
    }
  )

  /** 提交中（按钮禁用态，保留旧 submitting 命名，映射到 journey.loading） */
  const submitting = computed(() => journey.loading.value)

  /** 写入评估结果（保留旧 setResult 兼容入口） */
  function setResult(r: EvaluationDetailResponse, id: number) {
    journey.currentResult.value = r
    journey.currentId.value = id
  }

  /** 提交评估：创建即返回完整结果 → 落 journey，返回 id；在途重复调用返回 null。 */
  async function submitEvaluation(payload: CreateEvaluationRequest): Promise<number | null> {
    if (journey.loading.value) return null
    await journey.submit(payload)
    return journey.currentId.value
  }

  /** 按 id 拉取详情（刷新恢复 / 直达结果页），成功返回 true。 */
  async function fetchDetail(id: number): Promise<boolean> {
    try {
      await journey.fetch(id)
      return true
    } catch {
      return false
    }
  }

  /** 清空全部（保留旧 clearAll 命名，内部为 journey.reset） */
  function clearAll() {
    journey.reset()
  }

  return {
    currentResult: journey.currentResult,
    currentId: journey.currentId,
    loading: journey.loading,
    error: journey.error,
    submitting,
    setResult,
    submitEvaluation,
    fetchDetail,
    clearAll
  }
})
