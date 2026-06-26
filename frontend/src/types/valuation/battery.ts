// 电池 RUL 评估模块 TypeScript 类型定义
// 与后端 model/battery.go DTO 字段一一对应

/** 电池类型枚举 */
export type BatteryType = 'lfp' | 'ncm' | 'other'

/** 电池类型中文标签 */
export const BATTERY_TYPE_LABELS: Record<BatteryType, string> = {
  lfp: '磷酸铁锂（LFP）',
  ncm: '三元锂（NCM）',
  other: '其他'
}

/** 单次循环充放电数据 */
export interface CycleData {
  cycle_index: number
  voltage_series: number[]
  current_series: number[]
  capacity: number
}

/** 评估请求 */
export interface CreateBatteryRequest {
  battery_type: BatteryType
  battery_model?: string
  cycles: CycleData[]
}

/** 评估响应（创建后） */
export interface CreateBatteryResponse {
  evaluation_id: number
  battery_type: BatteryType
  cycle_count: number
  rul_cycles: number
  soh_percent: number
  confidence: number
  confidence_low: number
  confidence_high: number
  suggestions: string[]
  created_at: string
}

/** 特征重要性条目 */
export interface FeatureImportance {
  index: number
  name: string
  group: string
  weight: number
  normalized: number
}

/** 周期特征记录 */
export interface CycleFeature {
  id?: number
  evaluation_id?: number
  cycle_index: number
  feature_vector: number[]
  raw_stats: RawStats
  soh_at_cycle: number
}

/** 原始统计 */
export interface RawStats {
  voltage_mean: number
  voltage_std: number
  current_mean: number
  current_std: number
  capacity: number
  cc_duration: number
  cv_duration: number
  ic_peak: number
  ic_peak_voltage: number
}

/** 评估详情（完整版） */
export interface BatteryEvaluationDetail {
  id: number
  battery_type: BatteryType
  battery_model: string
  cycle_count: number
  rul_cycles: number
  soh_percent: number
  confidence: number
  confidence_low: number
  confidence_high: number
  feature_importance: FeatureImportance[]
  report_pdf_path: string
  created_at: string
  updated_at: string
  cycle_features?: CycleFeature[]
  suggestions?: string[]
}

/** 评估列表项（摘要） */
export interface BatteryEvaluationListItem {
  id: number
  battery_type: BatteryType
  battery_model: string
  cycle_count: number
  rul_cycles: number
  soh_percent: number
  confidence: number
  created_at: string
}

/** 列表响应 */
export interface BatteryListResponse {
  total: number
  items: BatteryEvaluationListItem[]
}

/** 报告生成响应 */
export interface BatteryReportResponse {
  evaluation_id: number
  report_path: string
  generated_at: string
}

/** 6 维特征组（用于雷达图） */
export const BATTERY_FEATURE_GROUPS = [
  '恒流电压',
  '恒压电流',
  '阶段时间',
  '充电容量',
  'ICA峰位',
  '循环演化差分'
] as const

export type BatteryFeatureGroup = (typeof BATTERY_FEATURE_GROUPS)[number]
