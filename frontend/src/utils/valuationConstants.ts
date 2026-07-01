// 公共常量
// 重构说明：删除旧 ITEM_STATUS_OPTIONS / WORK_CONDITION_OPTIONS / FUEL_TYPE_OPTIONS / BRAND_TIER_LABEL
//         字典选项统一从后端动态加载，前端只保留系数定义（用于图表/报告展示）

/** 系数 K 对应的中文名 + 颜色（用于 ECharts / 报告卡片）
 *  COEFFICIENT_DEFS 用于明细网格展示 5 维（与雷达图维度一致）
 *  残值公式不变：残值 = 原价 × Kt_adj × Kc × Km，其中 Kt_adj = Kt^(Kh/Kb)
 *  雷达图维度还原为 5 个独立 K 系数展示
 */
export const COEFFICIENT_DEFS: Array<{
  key: CoefficientKey
  label: string
  color: string
  description: string
}> = [
  { key: 'k_time', label: '出厂时间', color: '#0F4C81', description: '时间衰减系数 Kt = e^(-λ·age)' },
  { key: 'k_hours', label: '使用强度', color: '#8B5CF6', description: '使用强度系数 Kh，按累计工时与行业标准比值查表' },
  { key: 'k_brand', label: '品牌价值', color: '#EC4899', description: '品牌系数 Kb = 品牌类型系数 × 品牌系数' },
  { key: 'k_market', label: '市场需求', color: '#0EA5E9', description: '市场系数 Km，按省/市区域系数调节' },
  { key: 'k_condition', label: '车辆情况', color: '#F59E0B', description: '车况系数 Kc，按车况评级（A~E）与原漆/证件/保养等加权' }
]

/** 5 个系数/维度字段（用于迭代/类型映射） */
export type CoefficientKey = 'k_time' | 'k_hours' | 'k_brand' | 'k_market' | 'k_condition'

export interface CoefficientMap {
  k_time: number
  k_hours: number
  k_brand: number
  k_market: number
  k_condition: number
}

/** 默认展示的 5 维度评分标签顺序（与后端 dimension_scores 顺序对齐，仅用于雷达图稳定渲染） */
export const DIMENSION_LABELS = [
  '出厂时间',
  '使用强度',
  '品牌价值',
  '市场需求',
  '车辆情况'
] as const

/** 车况评级展示色（A 绿 → E 红） */
export const CONDITION_RATING_COLOR: Record<string, string> = {
  A: '#16A34A',
  B: '#0EA5E9',
  C: '#F59E0B',
  D: '#F97316',
  E: '#DC2626'
}
