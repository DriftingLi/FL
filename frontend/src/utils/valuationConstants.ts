// 公共常量（与后端枚举保持一致）
import type { ItemStatus, WorkCondition, FuelType, BrandTier } from '@/types/valuation/evaluation'

/** 部件状态可选项（中文 label + value） */
export const ITEM_STATUS_OPTIONS: Array<{ value: ItemStatus; label: string; score: number; color: string }> = [
  { value: 'normal', label: '正常', score: 1.0, color: '#16A34A' },
  { value: 'slight_wear', label: '轻微磨损', score: 0.85, color: '#0EA5E9' },
  { value: 'need_repair', label: '需维修', score: 0.6, color: '#F59E0B' },
  { value: 'need_replace', label: '需更换', score: 0.3, color: '#DC2626' }
]

/** 工况可选项 */
export const WORK_CONDITION_OPTIONS: Array<{ value: WorkCondition; label: string; kw: number }> = [
  { value: '仓储', label: '仓储', kw: 1.05 },
  { value: '港口', label: '港口', kw: 0.95 },
  { value: '冷库', label: '冷库', kw: 0.9 },
  { value: '工地', label: '工地', kw: 0.85 },
  { value: '其他', label: '其他', kw: 1.0 }
]

/** 燃料类型（仅内燃） */
export const FUEL_TYPE_OPTIONS: Array<{ value: FuelType; label: string }> = [
  { value: '柴油', label: '柴油' },
  { value: '汽油', label: '汽油' },
  { value: '液化石油气(LPG)', label: '液化石油气' },
  { value: '天然气(CNG)', label: '天然气' }
]

/** 品牌档次 label */
export const BRAND_TIER_LABEL: Record<BrandTier, string> = {
  tier1_intl: '国际一线',
  tier2_intl: '国际二线',
  tier1_domestic: '国内一线',
  tier2_domestic: '国内二线'
}

/** 系数 K 对应的中文名 + 颜色（用于 ECharts） */
export const COEFFICIENT_DEFS: Array<{ key: keyof CoefficientMap; label: string; color: string; description: string }> = [
  { key: 'k_time', label: 'Kt 时间衰减', color: '#0F4C81', description: 'Kt = e^(-λ·t)，t 为使用年限' },
  { key: 'k_hours', label: 'Kh 使用强度', color: '#0EA5E9', description: '按累计小时与行业标准的比值区间查表' },
  { key: 'k_work', label: 'Kw 工况系数', color: '#16A34A', description: '按使用工况（仓储/港口/冷库/工地/其他）' },
  { key: 'k_brand', label: 'Kb 品牌系数', color: '#A855F7', description: '按品牌档次（国际一线/二线/国内一线/二线）' },
  { key: 'k_condition', label: 'Kc 车况系数', color: '#F59E0B', description: '两级加权：类别 × 条目，按状态评分' },
  { key: 'k_market', label: 'Km 市场系数', color: '#EC4899', description: '市场供需调节系数' }
]

/** 6 个 K 系数字段（用于迭代） */
export interface CoefficientMap {
  k_time: number
  k_hours: number
  k_work: number
  k_brand: number
  k_condition: number
  k_market: number
}
