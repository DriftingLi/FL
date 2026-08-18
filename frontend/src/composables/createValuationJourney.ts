// createValuationJourney：残值评估 / 电池 RUL 共享的结果旅程 module。
// interface：submit / fetch / reset 三个操作 + loading / error / currentId / currentResult / currentDetail 状态。
// 两个 Pinia store 只注入各自的持久化 adapter（ADR-0015）。
import { ref } from 'vue'
import type { Ref } from 'vue'

export interface ValuationJourneyAdapters<TPayload, TSubmitResult, TDetail> {
  submit: (payload: TPayload) => Promise<TSubmitResult>
  fetch: (id: number) => Promise<TDetail>
}

export interface ValuationJourneyOptions<TSubmitResult, TDetail> {
  idOfSubmit: (result: TSubmitResult) => number
  idOfDetail: (detail: TDetail) => number
  /** fetch 成功后是否把 detail 同步写入 currentResult（残值评估路径需要，电池路径不需要） */
  fetchWritesResult?: boolean
  submitErrorFallback?: string
  loadErrorFallback?: string
}

function errorMessage(e: unknown, fallback: string): string {
  return e instanceof Error ? e.message : fallback
}

export function createValuationJourney<TPayload, TSubmitResult, TDetail>(
  adapters: ValuationJourneyAdapters<TPayload, TSubmitResult, TDetail>,
  options: ValuationJourneyOptions<TSubmitResult, TDetail>
) {
  const loading = ref(false)
  const error = ref<string | null>(null)
  const currentId = ref<number | null>(null)
  const currentResult: Ref<TSubmitResult | TDetail | null> = ref(null)
  const currentDetail = ref<TDetail | null>(null)

  async function submit(payload: TPayload): Promise<TSubmitResult> {
    loading.value = true
    error.value = null
    try {
      const result = await adapters.submit(payload)
      currentResult.value = result
      currentId.value = options.idOfSubmit(result)
      return result
    } catch (e) {
      error.value = errorMessage(e, options.submitErrorFallback ?? '提交失败')
      throw e
    } finally {
      loading.value = false
    }
  }

  async function fetch(id: number): Promise<TDetail> {
    loading.value = true
    error.value = null
    try {
      const detail = await adapters.fetch(id)
      currentDetail.value = detail
      currentId.value = options.idOfDetail(detail)
      if (options.fetchWritesResult) {
        currentResult.value = detail
      }
      return detail
    } catch (e) {
      error.value = errorMessage(e, options.loadErrorFallback ?? '加载失败')
      throw e
    } finally {
      loading.value = false
    }
  }

  function reset() {
    currentResult.value = null
    currentDetail.value = null
    currentId.value = null
    error.value = null
  }

  return {
    loading,
    error,
    currentId,
    currentResult,
    currentDetail,
    submit,
    fetch,
    reset
  }
}
