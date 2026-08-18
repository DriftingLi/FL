// 电池 RUL 评估状态：结果旅程的薄 adapter（ADR-0015）。
// 状态机由 createValuationJourney 唯一实现；本 store 注入电池持久化 adapter。
import { defineStore } from 'pinia'
import type { Ref } from 'vue'
import {
  createBatteryEvaluation,
  getBatteryEvaluation
} from '@/api/valuation/battery'
import type {
  CreateBatteryRequest,
  CreateBatteryResponse,
  BatteryEvaluationDetail
} from '@/types/valuation/battery'
import { createValuationJourney } from '@/composables/createValuationJourney'

export const useBatteryStore = defineStore('battery', () => {
  const journey = createValuationJourney<CreateBatteryRequest, CreateBatteryResponse, BatteryEvaluationDetail>(
    {
      submit: createBatteryEvaluation,
      fetch: getBatteryEvaluation
    },
    {
      idOfSubmit: r => r.evaluation_id,
      idOfDetail: d => d.id,
      submitErrorFallback: '提交失败',
      loadErrorFallback: '加载失败'
    }
  )

  // fetchWritesResult=false，因此 currentResult 只会写入提交结果；类型收窄到 CreateBatteryResponse。
  const currentResult = journey.currentResult as Ref<CreateBatteryResponse | null>
  const currentDetail = journey.currentDetail

  /** 提交循环数据并预测 */
  function submitCycles(payload: CreateBatteryRequest): Promise<CreateBatteryResponse> {
    return journey.submit(payload)
  }

  /** 按 ID 拉取详情 */
  function fetchDetail(id: number): Promise<BatteryEvaluationDetail> {
    return journey.fetch(id)
  }

  /** 重置全部状态（用于离开结果页后） */
  function reset() {
    journey.reset()
  }

  return {
    currentResult,
    currentDetail,
    currentId: journey.currentId,
    loading: journey.loading,
    error: journey.error,
    submitCycles,
    fetchDetail,
    reset
  }
})
