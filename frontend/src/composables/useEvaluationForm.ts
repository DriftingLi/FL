// 表单状态与提交 composable（统一表单，供 ValuationInputView 使用）
// 重构说明：删除旧的 ForkliftType 分支 + itemStatusMap；改为字典驱动的统一表单
import { reactive, ref, computed, type ComputedRef } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { createEvaluation } from '@/api/valuation/evaluation'
import { useEvaluationStore } from '@/stores/valuationEvaluation'
import type {
  ConditionRating,
  CreateEvaluationRequest
} from '@/types/valuation/evaluation'
import { validateForm, type FormValidationContext } from '@/utils/valuationValidator'

/** 基础表单状态：覆盖 CreateEvaluationRequest 全部字段 */
export interface BaseFormState {
  brand_type: string | undefined
  brand: string | undefined
  vehicle_type: string | undefined
  series: string | undefined
  tonnage: number | undefined
  config_type: string | undefined
  mast_type: string | undefined
  mast_height_mm: number | undefined
  factory_year: number | undefined
  sale_year: number | undefined
  usage_hours: number | undefined
  original_paint: boolean
  battery_type: string | undefined
  province: string | undefined
  city: string | undefined
  has_license_plate: boolean
  has_registration_certificate: boolean
  has_maintenance_records: boolean
  condition_rating: ConditionRating | undefined
}

export interface UseEvaluationFormOptions {
  /** 车辆类型字典中是否含电动（用于决定 battery_type 字段可见性/必填性） */
  hasElectricVehicleType: ComputedRef<boolean>
}

export function useEvaluationForm(options: UseEvaluationFormOptions) {
  const router = useRouter()
  const store = useEvaluationStore()
  const submitting = ref(false)

  // 基础信息（默认值）
  const form = reactive<BaseFormState>({
    brand_type: undefined,
    brand: undefined,
    vehicle_type: undefined,
    series: undefined,
    tonnage: undefined,
    config_type: undefined,
    mast_type: undefined,
    mast_height_mm: undefined,
    factory_year: undefined,
    sale_year: new Date().getFullYear(),
    usage_hours: undefined,
    original_paint: true,
    battery_type: undefined,
    province: undefined,
    city: undefined,
    has_license_plate: false,
    has_registration_certificate: false,
    has_maintenance_records: false,
    condition_rating: undefined
  })

  /** 构造提交 payload */
  function buildPayload(): CreateEvaluationRequest | null {
    // 必填字段守卫：未填则返回 null
    if (
      !form.brand_type ||
      !form.brand ||
      !form.vehicle_type ||
      !form.series ||
      form.tonnage == null ||
      !form.config_type ||
      !form.mast_type ||
      form.mast_height_mm == null ||
      form.factory_year == null ||
      form.sale_year == null ||
      form.usage_hours == null ||
      !form.province ||
      !form.city ||
      !form.condition_rating
    ) {
      return null
    }
    const payload: CreateEvaluationRequest = {
      brand_type: form.brand_type,
      brand: form.brand,
      vehicle_type: form.vehicle_type,
      series: form.series,
      tonnage: form.tonnage,
      config_type: form.config_type,
      mast_type: form.mast_type,
      mast_height_mm: form.mast_height_mm,
      factory_year: form.factory_year,
      sale_year: form.sale_year,
      usage_hours: form.usage_hours,
      original_paint: form.original_paint,
      // 电池类型仅在字典含电动时下发，避免污染 combustion 请求
      battery_type: options.hasElectricVehicleType.value ? form.battery_type : undefined,
      province: form.province,
      city: form.city,
      has_license_plate: form.has_license_plate,
      has_registration_certificate: form.has_registration_certificate,
      has_maintenance_records: form.has_maintenance_records,
      condition_rating: form.condition_rating
    }
    return payload
  }

  /** 整体表单校验（返回第一个失败的 message） */
  function validate(): { valid: boolean; message?: string } {
    const ctx: FormValidationContext = {
      brand_type: form.brand_type,
      brand: form.brand,
      vehicle_type: form.vehicle_type,
      series: form.series,
      tonnage: form.tonnage,
      config_type: form.config_type,
      mast_type: form.mast_type,
      mast_height_mm: form.mast_height_mm,
      factory_year: form.factory_year,
      sale_year: form.sale_year,
      usage_hours: form.usage_hours,
      province: form.province,
      city: form.city,
      condition_rating: form.condition_rating,
      battery_type: form.battery_type,
      hasElectricVehicleType: options.hasElectricVehicleType.value
    }
    return validateForm(ctx)
  }

  /** 是否可提交（用于按钮禁用态，仅做粗校验；提交时再做完整校验） */
  const isValid = computed(() => validate().valid)

  /** 重置全部 */
  function reset() {
    form.brand_type = undefined
    form.brand = undefined
    form.vehicle_type = undefined
    form.series = undefined
    form.tonnage = undefined
    form.config_type = undefined
    form.mast_type = undefined
    form.mast_height_mm = undefined
    form.factory_year = undefined
    form.sale_year = new Date().getFullYear()
    form.usage_hours = undefined
    form.original_paint = true
    form.battery_type = undefined
    form.province = undefined
    form.city = undefined
    form.has_license_plate = false
    form.has_registration_certificate = false
    form.has_maintenance_records = false
    form.condition_rating = undefined
  }

  /** 提交评估 */
  async function submit(): Promise<boolean> {
    if (submitting.value) return false
    const check = validate()
    if (!check.valid) {
      ElMessage.warning(check.message || '请补全表单')
      return false
    }
    const payload = buildPayload()
    if (!payload) {
      ElMessage.warning('请补全表单')
      return false
    }
    submitting.value = true
    try {
      const result = await createEvaluation(payload)
      store.setResult(result, result.id)
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
    submitting,
    isValid,
    buildPayload,
    validate,
    reset,
    submit
  }
}
