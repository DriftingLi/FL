// 与后端 DTO 一一对应的 TypeScript 类型
// 后端路径：backend/internal/model/evaluation.go

/** 叉车类型 */
export type ForkliftType = 'electric' | 'combustion'

/** 使用工况 */
export type WorkCondition = '仓储' | '港口' | '冷库' | '工地' | '其他'

/** 燃料类型（仅内燃） */
export type FuelType = '柴油' | '汽油' | '液化石油气(LPG)' | '天然气(CNG)'

/** 部件状态 */
export type ItemStatus = 'normal' | 'slight_wear' | 'need_repair' | 'need_replace'

/** 品牌档次 */
export type BrandTier =
  | 'tier1_intl'
  | 'tier2_intl'
  | 'tier1_domestic'
  | 'tier2_domestic'

// ========== 评估请求/响应 ==========

/** 提交评估请求体 */
export interface CreateEvaluationRequest {
  forklift_type: ForkliftType
  brand: string
  model?: string
  /** 原始购买价格（万元） */
  original_price: number
  purchase_year: number
  sale_year: number
  /** 累计使用小时 */
  usage_hours: number
  work_condition: WorkCondition
  fuel_type?: FuelType
  can_drive: boolean
  hydraulic_ok: boolean
  items: Array<{
    item_code: string
    status: ItemStatus
  }>
}

/** 后端统一响应包装 */
export interface ApiResponse<T> {
  code: number
  message: string
  data: T
}

/** 评估结果详情（POST /evaluations 响应） */
export interface EvaluationResult {
  id: number
  k_time: number
  k_hours: number
  k_work: number
  k_brand: number
  k_condition: number
  k_market: number
  estimated_value: number
  confidence_low: number
  confidence_high: number
  /** 原始购买价格（万元），用于算残值率 */
  original_price: number
  /** 6 维度评分（中文标签 → 0~1） */
  dimension_scores: Record<string, number>
  /** 文本建议（基于评估结果生成） */
  suggestions: string[]
}

/** 评估条目（详情接口返回） */
export interface EvaluationItem {
  id: number
  evaluation_id: number
  category_code: string
  category_name: string
  item_code: string
  item_name: string
  status: ItemStatus
  category_weight: number
  item_weight: number
  score: number
}

/**
 * 评估详情（GET /evaluations/:id 响应）
 * 注意：后端返回的是扁平结构（与 EvaluationResult 字段并列），不是 {evaluation, items} 嵌套
 */
export interface EvaluationDetail extends EvaluationResult {
  forklift_type: ForkliftType
  brand: string
  model?: string
  original_price: number
  purchase_year: number
  sale_year: number
  usage_hours: number
  work_condition: WorkCondition
  fuel_type?: FuelType
  can_drive: boolean
  hydraulic_ok: boolean
  report_pdf_path?: string
  created_at?: string
  /** 部件状态列表（仅 detail 接口返回） */
  items?: EvaluationItem[]
}

/** 详情接口响应（与 EvaluationDetail 同构） */
export type EvaluationDetailResponse = EvaluationDetail

// ========== 分页/列表 ==========

export interface PageQuery {
  page?: number
  page_size?: number
  forklift_type?: ForkliftType
}

export interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}
