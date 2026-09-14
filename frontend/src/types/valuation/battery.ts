// 电池 RUL 评估模块类型 —— 响应形状**唯一事实源**是后端注解
// （ADR-0048 决策 1/3，issue #967 片九）：生成物 frontend/src/api/generated/valuation.ts。
//
// 本文件只保留生成器表达力覆盖不到的两类东西：
//   1) 前端窄化联合（BatteryType：注解层是 string，枚举词汇缺口见 ADR-0048 片一）；
//   2) 入参类型（CreateBatteryRequest / CycleData，ADR-0048 决策 3 不生成）。
import type {
  BatteryEvaluation,
  CreateBatteryResponse,
  FeatureImportance,
  ListBatteryResponse
} from '@/api/generated/valuation'

export type { CreateBatteryResponse, FeatureImportance, ListBatteryResponse }

/** 电池类型枚举（生成面是 string，此处收窄） */
export type BatteryType = 'lfp' | 'ncm' | 'other'

/** 电池类型中文标签 */
export const BATTERY_TYPE_LABELS: Record<BatteryType, string> = {
  lfp: '磷酸铁锂（LFP）',
  ncm: '三元锂（NCM）',
  other: '其他'
}

/** 单次循环充放电数据（入参，不生成 —— ADR-0048 决策 3） */
export interface CycleData {
  cycle_index: number
  voltage_series: number[]
  current_series: number[]
  capacity: number
}

/** 评估请求（入参，不生成 —— ADR-0048 决策 3） */
export interface CreateBatteryRequest {
  battery_type: BatteryType
  battery_model?: string
  cycles: CycleData[]
}

/** 评估详情（完整版）：生成类型别名（GET /battery/evaluations/:id） */
export type BatteryEvaluationDetail = BatteryEvaluation

/** 评估列表项（摘要）：生成类型别名 */
export type { BatteryEvaluationSummary as BatteryEvaluationListItem } from '@/api/generated/valuation'

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
