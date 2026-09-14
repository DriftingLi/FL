// 估值评估响应类型 —— **唯一事实源**是后端注解 → backend/docs/swagger.json →
// `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3，issue #967 片九）。
//
// 本文件只保留两类东西：
//   1) 生成类型的再导出（旧名 → 生成名，调用点 import 路径与名字不变）；
//   2) 生成器表达力覆盖不到的**前端窄化/入参**类型：ConditionRating / PowerType 是封闭值集
//      联合，注解层目前只有 string（枚举词汇缺口，ADR-0048 片一「已知限制」）；入参
//      （CreateEvaluationRequest / PageQuery）不生成（决策 3）。
import type {
  DimensionScore,
  EvaluationDetail,
  EvaluationResponse,
  ConfigOption,
  VehicleType,
  Series,
  Tonnage,
  MastType,
  MastHeight,
  BatteryTypeDict,
  TransmissionType,
  EngineType,
  SeriesConfigOptions,
  ConditionRating as ConditionRatingItem,
  CoefficientConfig
} from '@/api/generated/valuation'

export type {
  DimensionScore,
  EvaluationDetail,
  EvaluationResponse,
  ConfigOption,
  CoefficientConfig,
  SeriesConfigOptions,
  VehicleType,
  Series,
  Tonnage,
  MastType,
  MastHeight,
  BatteryTypeDict,
  TransmissionType,
  EngineType
}

/** 动力类型（车辆类型字典中的 power_type；生成面是 string，此处收窄） */
export type PowerType = 'electric' | 'combustion'

/** 车况评级（A 优 → E 差）；生成面是 string，此处收窄 */
export type ConditionRating = 'A' | 'B' | 'C' | 'D' | 'E'

// ========== 字典条目类型（生成类型别名，旧名保留） ==========

/** 车辆类型字典项 */
export type VehicleTypeOption = VehicleType

/** 系列字典项 */
export type SeriesOption = Series

/** 吨位字典项 */
export type TonnageOption = Tonnage

/** 配置类型字典项 */
export type ConfigTypeOption = ConfigOption

/** 门架类型字典项 */
export type MastTypeOption = MastType

/** 门架高度字典项 */
export type MastHeightOption = MastHeight

/** 电池类型字典项 */
export type BatteryTypeOption = BatteryTypeDict

/** 传动系统字典项（手波/自波/无级变速/无） */
export type TransmissionTypeOption = TransmissionType

/** 发动机类型字典项（国产发动机/进口发动机/混合动力/无） */
export type EngineTypeOption = EngineType

/** 车况评级字典项（rating 生成面是 string，消费处按 ConditionRating 收窄） */
export type ConditionRatingOption = ConditionRatingItem

// ========== 评估请求/响应 ==========

/** 提交评估请求体（入参，不生成 —— ADR-0048 决策 3） */
export interface CreateEvaluationRequest {
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

/**
 * 评估结果（POST /evaluations 响应）：生成类型别名。
 * 创建响应即含输入参数（后端与详情同源返回，ADR-0004）：匿名用户提交后可直接渲染结果页。
 */
export type EvaluationResult = EvaluationResponse

/** 详情接口响应（GET /evaluations/:id）：与生成类型同构 */
export type EvaluationDetailResponse = EvaluationDetail

// ========== 分页/列表 ==========

/** 分页查询入参（不生成 —— ADR-0048 决策 3） */
export interface PageQuery {
  page?: number
  page_size?: number
  /** 按车辆类型过滤（值来自 vehicle_types 字典的 name） */
  vehicle_type?: string
  /** 按品牌过滤 */
  brand?: string
}

/** 分页响应壳（泛型形状，非某端点线格式） */
export interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}

/**
 * 评估统计（GET /evaluations/stats 响应）。
 * 注解层用 swag 内联 object{total=integer} 描述（无具名 Go 类型），生成器只渲染具名类型，
 * 故此处保留该窄形状 —— 唯一来源仍是注解（见 generated/valuation.ts 头部端点清单）。
 */
export interface EvaluationStats {
  /** 累计评估次数 */
  total: number
}
