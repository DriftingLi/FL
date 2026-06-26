// 电池 RUL 评估状态：当前结果 + 加载/错误态
import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  createBatteryEvaluation,
  getBatteryEvaluation
} from '@/api/valuation/battery'
import type {
  CreateBatteryRequest,
  CreateBatteryResponse,
  BatteryEvaluationDetail
} from '@/types/valuation/battery'

export const useBatteryStore = defineStore('battery', () => {
  /** 当前评估结果摘要（Create 接口返回） */
  const currentResult = ref<CreateBatteryResponse | null>(null)
  /** 评估详情（含 20 维特征、cycle_features、suggestions） */
  const currentDetail = ref<BatteryEvaluationDetail | null>(null)
  /** 当前评估 ID（用于结果页跳报告） */
  const currentId = ref<number | null>(null)
  /** 提交/加载中 */
  const loading = ref(false)
  /** 错误信息 */
  const error = ref<string | null>(null)

  /** 提交循环数据并预测 */
  async function submitCycles(payload: CreateBatteryRequest): Promise<CreateBatteryResponse> {
    loading.value = true
    error.value = null
    try {
      const data = await createBatteryEvaluation(payload)
      currentResult.value = data
      currentId.value = data.evaluation_id
      return data
    } catch (e) {
      const msg = e instanceof Error ? e.message : '提交失败'
      error.value = msg
      throw e
    } finally {
      loading.value = false
    }
  }

  /** 按 ID 拉取详情 */
  async function fetchDetail(id: number): Promise<BatteryEvaluationDetail> {
    loading.value = true
    error.value = null
    try {
      const data = await getBatteryEvaluation(id)
      currentDetail.value = data
      currentId.value = data.id
      return data
    } catch (e) {
      const msg = e instanceof Error ? e.message : '加载失败'
      error.value = msg
      throw e
    } finally {
      loading.value = false
    }
  }

  /** 重置全部状态（用于离开结果页后） */
  function reset() {
    currentResult.value = null
    currentDetail.value = null
    currentId.value = null
    error.value = null
  }

  return {
    currentResult,
    currentDetail,
    currentId,
    loading,
    error,
    submitCycles,
    fetchDetail,
    reset
  }
})
