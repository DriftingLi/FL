// 公共常量
// 重构说明：删除旧 ITEM_STATUS_OPTIONS / WORK_CONDITION_OPTIONS / FUEL_TYPE_OPTIONS / BRAND_TIER_LABEL
//         字典选项统一从后端动态加载，前端只保留系数定义（用于图表/报告展示）

/** 系数 K 对应的中文名 + 颜色（用于 ECharts / 报告卡片）
 *  重构说明：品牌系数 Kb 与使用强度系数 Kh 不再直接作用于残值，而是修正时间衰减速率
 *  公式：残值 = 原价 × Kt_adj × Kc × Km，其中 Kt_adj = Kt^(Kh/Kb)
 *  COEFFICIENT_DEFS 用于明细网格展示 4 维（与雷达图维度一致）
 */
export const COEFFICIENT_DEFS: Array<{
  key: CoefficientKey
  label: string
  color: string
  description: string
}> = [
  { key: 'k_time_adjusted', label: 'Kt 修正后', color: '#0F4C81', description: '时间衰减，经品牌/强度修正：Kt_adj = Kt^(Kh/Kb)' },
  { key: 'k_condition', label: 'Kc 车况', color: '#F59E0B', description: '按车况评级（A~E）与原漆/证件/保养等加权' },
  { key: 'k_market', label: 'Km 市场', color: '#EC4899', description: '按省/市区域系数调节' },
  { key: 'residual_rate', label: '残值率', color: '#16A34A', description: '残值/原价（已钳制 ≤ 100%）' }
]

/** 4 个系数/维度字段（用于迭代/类型映射） */
export type CoefficientKey = 'k_time_adjusted' | 'k_condition' | 'k_market' | 'residual_rate'

export interface CoefficientMap {
  k_time_adjusted: number
  k_condition: number
  k_market: number
  residual_rate: number
}

/** 默认展示的 4 维度评分标签顺序（与后端 dimension_scores 顺序对齐，仅用于雷达图稳定渲染） */
export const DIMENSION_LABELS = [
  '时间衰减',
  '车况',
  '市场',
  '残值率'
] as const

/** 车况评级展示色（A 绿 → E 红） */
export const CONDITION_RATING_COLOR: Record<string, string> = {
  A: '#16A34A',
  B: '#0EA5E9',
  C: '#F59E0B',
  D: '#F97316',
  E: '#DC2626'
}
