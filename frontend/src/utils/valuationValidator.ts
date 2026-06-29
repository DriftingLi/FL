// 前端参数校验工具
// 与后端 model/evaluation.go 中的校验逻辑保持一致
// 重构说明：删除旧 brand/workCondition/items 校验，改用新字典化字段
// 配置类型重构：battery_type 字段已移除，改为三维度（transmission/engine/battery）
import type { ConditionRating } from '@/types/valuation/evaluation'

const CURRENT_YEAR = new Date().getFullYear()

export interface ValidationResult {
  valid: boolean
  message?: string
}

/** 校验字符串非空 */
function isNonEmpty(v: string | undefined | null): v is string {
  return !!v && v.trim().length > 0
}

/** 校验必填字符串字段 */
export function validateRequiredString(value: string | undefined | null, label: string): ValidationResult {
  if (!isNonEmpty(value)) {
    return { valid: false, message: `请选择${label}` }
  }
  return { valid: true }
}

/** 校验数值（> 0） */
export function validatePositiveNumber(value: number | undefined | null, label: string): ValidationResult {
  if (value == null || Number.isNaN(value)) {
    return { valid: false, message: `请填写${label}` }
  }
  if (value <= 0) {
    return { valid: false, message: `${label}必须大于 0` }
  }
  return { valid: true }
}

/** 校验数值（≥ 0） */
export function validateNonNegativeNumber(value: number | undefined | null, label: string): ValidationResult {
  if (value == null || Number.isNaN(value)) {
    return { valid: false, message: `请填写${label}` }
  }
  if (value < 0) {
    return { valid: false, message: `${label}不能为负数` }
  }
  return { valid: true }
}

/** 校验年份：出厂年份合法性（评估年份默认今年，无需校验） */
export function validateYears(factory: number | undefined): ValidationResult {
  if (factory == null) {
    return { valid: false, message: '请填写出厂年份' }
  }
  if (!Number.isInteger(factory)) {
    return { valid: false, message: '年份必须为整数' }
  }
  if (factory < 1980 || factory > CURRENT_YEAR) {
    return { valid: false, message: `出厂年份应在 1980~${CURRENT_YEAR} 之间` }
  }
  return { valid: true }
}

/** 校验累计工时：≥ 0 且 ≤ 100000 */
export function validateUsageHours(hours: number | undefined | null): ValidationResult {
  if (hours == null || Number.isNaN(hours)) {
    return { valid: false, message: '请填写累计使用工时' }
  }
  if (hours < 0) {
    return { valid: false, message: '使用工时不能为负数' }
  }
  if (hours > 100000) {
    return { valid: false, message: '使用工时超出合理范围' }
  }
  return { valid: true }
}

/** 校验吨位：> 0 且 ≤ 100（吨位始终必填，不允许 "无"） */
export function validateTonnage(tonnage: number | undefined | null): ValidationResult {
  if (tonnage == null || Number.isNaN(tonnage)) {
    return { valid: false, message: '请选择吨位' }
  }
  if (tonnage <= 0) {
    return { valid: false, message: '吨位必须大于 0' }
  }
  if (tonnage > 100) {
    return { valid: false, message: '吨位超出合理范围' }
  }
  return { valid: true }
}

/** 校验门架高度：允许 0（表示 "无"），否则 > 0 且 ≤ 20000（mm） */
export function validateMastHeight(value: number | undefined | null): ValidationResult {
  if (value == null || Number.isNaN(value)) {
    return { valid: false, message: '请选择门架高度' }
  }
  // 0 表示 "无"，合法
  if (value === 0) {
    return { valid: true }
  }
  if (value < 0) {
    return { valid: false, message: '门架高度不能为负数' }
  }
  if (value > 20000) {
    return { valid: false, message: '门架高度超出合理范围' }
  }
  return { valid: true }
}

/** 校验车况评级：必须为 A/B/C/D/E */
const VALID_RATINGS: ConditionRating[] = ['A', 'B', 'C', 'D', 'E']
export function validateConditionRating(rating: string | undefined | null): ValidationResult {
  if (!rating || !VALID_RATINGS.includes(rating as ConditionRating)) {
    return { valid: false, message: '请选择车况评级' }
  }
  return { valid: true }
}

/** 整体表单校验上下文：包含三维度配置及各维度可用选项 */
export interface FormValidationContext {
  brand_type: string | undefined
  brand: string | undefined
  vehicle_type: string | undefined
  series: string | undefined
  tonnage: number | undefined
  config_type: string | undefined // 由三维度 computed 而来
  mast_type: string | undefined
  mast_height_mm: number | undefined
  factory_year: number | undefined
  sale_year: number | undefined
  usage_hours: number | undefined
  province: string | undefined
  city: string | undefined
  condition_rating: string | undefined
  // 三维度配置字段
  transmission?: string | undefined
  engine?: string | undefined
  battery?: string | undefined
  // 各维度可用选项（数组非空即该维度启用）
  transmissionOptions?: string[]
  engineOptions?: string[]
  batteryOptions?: string[]
}

/** 整体校验：依次检查关键字段 */
export function validateForm(ctx: FormValidationContext): ValidationResult {
  const checks: Array<() => ValidationResult> = [
    () => validateRequiredString(ctx.brand_type, '品牌类型'),
    () => validateRequiredString(ctx.brand, '品牌'),
    () => validateRequiredString(ctx.vehicle_type, '车辆类型'),
    () => validateRequiredString(ctx.series, '系列'),
    () => validateTonnage(ctx.tonnage),
    // config_type 由三维度拼接，校验放在维度校验之后
    () => validateRequiredString(ctx.mast_type, '门架类型'),
    () => validateMastHeight(ctx.mast_height_mm),
    () => validateYears(ctx.factory_year),
    () => validateUsageHours(ctx.usage_hours),
    () => validateRequiredString(ctx.province, '所在省份'),
    () => validateRequiredString(ctx.city, '所在城市'),
    () => validateConditionRating(ctx.condition_rating),
    // 三维度校验：维度启用（选项数组非空）时要求对应字段必填
    () => {
      if (ctx.transmissionOptions && ctx.transmissionOptions.length > 0 && !ctx.transmission) {
        return { valid: false, message: '请选择传动系统配置' }
      }
      return { valid: true }
    },
    () => {
      if (ctx.engineOptions && ctx.engineOptions.length > 0 && !ctx.engine) {
        return { valid: false, message: '请选择发动机类型配置' }
      }
      return { valid: true }
    },
    () => {
      if (ctx.batteryOptions && ctx.batteryOptions.length > 0 && !ctx.battery) {
        return { valid: false, message: '请选择电池配置' }
      }
      return { valid: true }
    },
    // 最终校验拼接后的 config_type 非空
    () => validateRequiredString(ctx.config_type, '配置类型')
  ]
  for (const c of checks) {
    const r = c()
    if (!r.valid) return r
  }
  return { valid: true }
}
