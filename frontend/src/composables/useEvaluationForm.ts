// 表单状态与提交 composable
// 共享给 ElectricInputView / CombustionInputView
import { ref, reactive, computed } from 'vue'
import { useRouter } from 'vue-router'
import { createEvaluation } from '@/api/valuation/evaluation'
import { useEvaluationStore } from '@/stores/valuationEvaluation'
import type {
  CreateEvaluationRequest,
  ForkliftType,
  ItemStatus
} from '@/types/valuation/evaluation'

export interface BaseFormState {
  brand: string | undefined
  model: string | undefined
  original_price: number | undefined
  purchase_year: number | undefined
  sale_year: number | undefined
  usage_hours: number | undefined
  work_condition: '仓储' | '港口' | '冷库' | '工地' | '其他' | undefined
  fuel_type?: '柴油' | '汽油' | '液化石油气(LPG)' | '天然气(CNG)' | undefined
  can_drive: boolean
  hydraulic_ok: boolean
}

export function useEvaluationForm(forkliftType: ForkliftType) {
  const router = useRouter()
  const store = useEvaluationStore()
  const submitting = ref(false)

  // 基础信息
  const form = reactive<BaseFormState>({
    brand: undefined,
    model: undefined,
    original_price: undefined,
    purchase_year: undefined,
    sale_year: undefined,
    usage_hours: undefined,
    work_condition: undefined,
    fuel_type: undefined,
    can_drive: true,
    hydraulic_ok: true
  })

  // 部件状态（item_code → status）
  // 关键：用 ref 包装，v-model 赋值给 ref.value 才能正常触发响应式
  // 之前用 reactive({})，v-model 目标为 const reactive 时 emit 出的新值会被丢弃
  const itemStatusMap = ref<Record<string, ItemStatus | undefined>>({})

  /** 已填部件数 / 总数 */
  const filledCount = computed(() => {
    return Object.values(itemStatusMap.value).filter((v) => !!v).length
  })
  const totalCount = computed(() => Object.keys(itemStatusMap.value).length)

  /** 一键全部置为"正常" */
  function setAllNormal() {
    const next: Record<string, ItemStatus> = {}
    for (const k of Object.keys(itemStatusMap.value)) {
      next[k] = 'normal'
    }
    // 整体替换 ref.value，触发响应式
    itemStatusMap.value = next
  }

  /** 重置全部 */
  function reset() {
    form.brand = undefined
    form.model = undefined
    form.original_price = undefined
    form.purchase_year = undefined
    form.sale_year = undefined
    form.usage_hours = undefined
    form.work_condition = undefined
    form.fuel_type = undefined
    form.can_drive = true
    form.hydraulic_ok = true
    const cleared: Record<string, ItemStatus | undefined> = {}
    for (const k of Object.keys(itemStatusMap.value)) {
      cleared[k] = undefined
    }
    itemStatusMap.value = cleared
  }

  /** 构造提交 payload */
  function buildPayload(): CreateEvaluationRequest {
    const items = Object.entries(itemStatusMap.value)
      .filter(([, v]) => !!v)
      .map(([item_code, status]) => ({ item_code, status: status as ItemStatus }))
    return {
      forklift_type: forkliftType,
      brand: form.brand ?? '',
      model: form.model,
      original_price: form.original_price ?? 0,
      purchase_year: form.purchase_year ?? 0,
      sale_year: form.sale_year ?? 0,
      usage_hours: form.usage_hours ?? 0,
      work_condition: form.work_condition ?? '其他',
      fuel_type: forkliftType === 'combustion' ? form.fuel_type : undefined,
      can_drive: form.can_drive,
      hydraulic_ok: form.hydraulic_ok,
      items
    }
  }

  /** 提交评估 */
  async function submit(): Promise<boolean> {
    if (submitting.value) return false
    submitting.value = true
    try {
      const payload = buildPayload()
      const result = await createEvaluation(payload)
      store.setResult(result, result.id, forkliftType)
      // 跳转结果页
      await router.push({ name: 'ValuationResult' })
      return true
    } catch (e) {
      // 错误提示由 axios 拦截器统一处理
      console.error('[submit] failed', e)
      return false
    } finally {
      submitting.value = false
    }
  }

  return {
    form,
    itemStatusMap,
    filledCount,
    totalCount,
    submitting,
    setAllNormal,
    reset,
    submit
  }
}
