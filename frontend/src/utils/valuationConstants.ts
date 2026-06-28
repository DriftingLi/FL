// 公共常量
// 重构说明：删除旧 ITEM_STATUS_OPTIONS / WORK_CONDITION_OPTIONS / FUEL_TYPE_OPTIONS / BRAND_TIER_LABEL
//         字典选项统一从后端动态加载，前端只保留系数定义（用于图表/报告展示）

/** 系数 K 对应的中文名 + 颜色（用于 ECharts / 报告卡片）
 *  注意：新版评估结果已移除 k_work，保留 5 个 K 系数
 */
export const COEFFICIENT_DEFS: Array<{
  key: CoefficientKey
  label: string
  color: string
  description: string
}> = [
  { key: 'k_time', label: 'Kt 时间衰减', color: '#0F4C81', description: '按使用年限（成交-出厂）指数衰减' },
  { key: 'k_hours', label: 'Kh 使用强度', color: '#0EA5E9', description: '按累计工时与行业标准的比值区间查表' },
  { key: 'k_brand', label: 'Kb 品牌系数', color: '#A855F7', description: '按品牌类型（来自 brand_types.k_type）与品牌系数加权' },
  { key: 'k_condition', label: 'Kc 车况系数', color: '#F59E0B', description: '按车况评级（A~E）与原漆/证件/保养等加权' },
  { key: 'k_market', label: 'Km 市场系数', color: '#EC4899', description: '按省/市区域系数调节' }
]

/** 5 个 K 系数字段（用于迭代/类型映射） */
export type CoefficientKey = 'k_time' | 'k_hours' | 'k_brand' | 'k_condition' | 'k_market'

export interface CoefficientMap {
  k_time: number
  k_hours: number
  k_brand: number
  k_condition: number
  k_market: number
}

/** 默认展示的 6 维度评分标签顺序（与后端 dimension_scores 顺序对齐，仅用于雷达图稳定渲染） */
export const DIMENSION_LABELS = [
  '时间维度',
  '使用强度',
  '品牌',
  '车况',
  '区域市场',
  '综合'
] as const

/** 车况评级展示色（A 绿 → E 红） */
export const CONDITION_RATING_COLOR: Record<string, string> = {
  A: '#16A34A',
  B: '#0EA5E9',
  C: '#F59E0B',
  D: '#F97316',
  E: '#DC2626'
}
