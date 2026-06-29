// 与后端新 DTO 一一对应的 TypeScript 类型
// 后端路径：backend/internal/model/evaluation.go
// 重构说明：删除旧的 ForkliftType / WorkCondition / FuelType / ItemStatus / BrandTier
//         改用统一的字典化字段：brand_type / brand / vehicle_type / series / tonnage 等

/** 动力类型（车辆类型字典中的 power_type） */
export type PowerType = 'electric' | 'combustion'

/** 车况评级（A 优 → E 差） */
export type ConditionRating = 'A' | 'B' | 'C' | 'D' | 'E'

/** 维度评分项（详情/结果中返回的 6 维评分，按维度顺序展示） */
export interface DimensionScore {
  label: string
  value: number
}

// ========== 字典条目类型 ==========
// 仅用于内部类型推导，实际数据全部从后端字典接口加载

/** 品牌类型字典项 */
export interface BrandTypeOption {
  name: string
  k_type: number
}

/** 车辆类型字典项 */
export interface VehicleTypeOption {
  id: number
  name: string
  power_type: PowerType
  /** 该车型最早出厂年份（用于前端级联限制出厂年份选择） */
  earliest_factory_year: number
}

/** 系列字典项 */
export interface SeriesOption {
  id: number
  brand: string
  name: string
  /** 该系列最早出厂年份（用于前端级联限制出厂年份选择） */
  earliest_factory_year: number
}

/** 吨位字典项 */
export interface TonnageOption {
  id: number
  value: number
}

/** 配置类型字典项 */
export interface ConfigTypeOption {
  id: number
  name: string
}

/** 门架类型字典项 */
export interface MastTypeOption {
  id: number
  name: string
}

/** 门架高度字典项 */
export interface MastHeightOption {
  id: number
  value_mm: number
}

/** 电池类型字典项 */
export interface BatteryTypeOption {
  id: number
  name: string
}

/** 传动系统字典项（手波/自波/无级变速/无） */
export interface TransmissionTypeOption {
  id: number
  name: string
}

/** 发动机类型字典项（国产发动机/进口发动机/混合动力/无） */
export interface EngineTypeOption {
  id: number
  name: string
}

/** 系列配置选项：某 series 支持的三维度可选项（数组为空表示该 series 不支持此维度） */
export interface SeriesConfigOptions {
  transmission: string[]
  engine: string[]
  battery: string[]
}

/** 车况评级字典项 */
export interface ConditionRatingOption {
  id: number
  rating: ConditionRating
  label: string
  base_coefficient: number
}

/** 算法参数（系数表） */
export interface CoefficientConfig {
  key: string
  value: number
  description: string
}

// ========== 评估请求/响应 ==========

/** 提交评估请求体（与后端 CreateEvaluationRequest 一致） */
export interface CreateEvaluationRequest {
  brand_type: string
  brand: string
  vehicle_type: string
  series: string
  tonnage: number
  config_type: string
  mast_type: string
  mast_height_mm: number
  factory_year: number
  sale_year: number
  usage_hours: number
  original_paint: boolean
  province: string
  city: string
  has_license_plate: boolean
  has_registration_certificate: boolean
  has_maintenance_records: boolean
  condition_rating: ConditionRating
}

/** 后端统一响应包装 */
export interface ApiResponse<T> {
  code: number
  message: string
  data: T
}

/** 评估结果（POST /evaluations 响应） */
export interface EvaluationResult {
  id: number
  /** 估算残值（万元） */
  estimated_value: number
  /** 置信区间下限 */
  confidence_low: number
  /** 置信区间上限 */
  confidence_high: number
  /** 原始购买价格（万元） */
  original_price: number
  /** 时间衰减系数 */
  k_time: number
  /** 使用强度系数 */
  k_hours: number
  /** 品牌系数 */
  k_brand: number
  /** 车况系数 */
  k_condition: number
  /** 市场系数 */
  k_market: number
  /** 6 维度评分列表 */
  dimension_scores: DimensionScore[]
  /** 文本建议 */
  suggestions: string[]
}

/** 评估详情（GET /evaluations/:id 响应，继承结果字段并补全输入参数） */
export interface EvaluationDetail extends EvaluationResult {
  brand_type: string
  brand: string
  vehicle_type: string
  series: string
  tonnage: number
  config_type: string
  mast_type: string
  mast_height_mm: number
  factory_year: number
  sale_year: number
  usage_hours: number
  original_paint: boolean
  province: string
  city: string
  has_license_plate: boolean
  has_registration_certificate: boolean
  has_maintenance_records: boolean
  condition_rating: ConditionRating
  report_pdf_path?: string
  created_at?: string
}

/** 详情接口响应（与 EvaluationDetail 同构） */
export type EvaluationDetailResponse = EvaluationDetail

// ========== 分页/列表 ==========

export interface PageQuery {
  page?: number
  page_size?: number
  /** 按车辆类型过滤（值来自 vehicle_types 字典的 name） */
  vehicle_type?: string
  /** 按品牌过滤 */
  brand?: string
}

export interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}
