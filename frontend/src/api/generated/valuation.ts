// 生成文件，勿手改（ADR-0019 契约 codegen 专项 / ADR-0048 按域解冻；spec #940 片五③、#952 片一）。
// 域：残值评估（/api/valuation/*：字典 / 评估 / 电池 RUL / 报告 / 平行 auth / 管理端 CRUD）
// 唯一事实源：后端注解 → backend/docs/swagger.json（CI 有新鲜度锁：backend-lint 的 swagger 步骤）。
// 再生成：cd backend && go run ./cmd/gen-apitypes
// 同步契约：backend/internal/apitypes/codegen_test.go 把本文件与注解渲染结果全等比对。
//
// 覆盖端点：
//   GET  /valuation/dictionaries/brands
//   GET  /valuation/dictionaries/vehicle-types
//   GET  /valuation/dictionaries/series
//   GET  /valuation/dictionaries/tonnages
//   GET  /valuation/dictionaries/config-types
//   GET  /valuation/dictionaries/mast-types
//   GET  /valuation/dictionaries/mast-heights
//   GET  /valuation/dictionaries/battery-types
//   GET  /valuation/dictionaries/transmission-types
//   GET  /valuation/dictionaries/engine-types
//   GET  /valuation/dictionaries/series-config-options
//   GET  /valuation/dictionaries/condition-ratings
//   GET  /valuation/dictionaries/provinces
//   GET  /valuation/dictionaries/cities
//   GET  /valuation/dictionaries/coefficient-configs
//   GET  /valuation/dictionaries/earliest-factory-year
//   GET  /valuation/dictionaries/algorithm-parameters
//   GET  /valuation/dictionaries/original-prices
//   GET  /valuation/dictionaries/region-coefficients
//   GET  /valuation/evaluations/stats
//   POST /valuation/evaluations/{id}/report
//   POST /valuation/battery/evaluations/{id}/report
//   POST /valuation/auth/logout
//   POST /valuation/evaluations
//   POST /valuation/battery/evaluations
//   GET  /valuation/evaluations
//   GET  /valuation/evaluations/{id}
//   GET  /valuation/battery/evaluations
//   GET  /valuation/battery/evaluations/{id}
//   GET  /valuation/auth/me
//   POST /valuation/admin/original-prices
//   PUT  /valuation/admin/original-prices/{id}
//   DELETE /valuation/admin/original-prices/{id}
//   POST /valuation/admin/region-coefficients
//   PUT  /valuation/admin/region-coefficients/{id}
//   PUT  /valuation/admin/coefficient-configs/{key}
//   POST /valuation/admin/brands
//   PUT  /valuation/admin/brands/{id}
//   POST /valuation/admin/vehicle-types
//   PUT  /valuation/admin/vehicle-types/{id}
//   POST /valuation/admin/series
//   PUT  /valuation/admin/series/{id}
//   POST /valuation/admin/tonnages
//   POST /valuation/admin/mast-types
//   POST /valuation/admin/mast-heights
//   POST /valuation/admin/battery-types
//   POST /valuation/admin/transmission-types
//   POST /valuation/admin/engine-types
//   POST /valuation/admin/condition-ratings
//   PUT  /valuation/admin/condition-ratings/{id}
//   DELETE /valuation/admin/brands/{id}
//   DELETE /valuation/admin/vehicle-types/{id}
//   DELETE /valuation/admin/series/{id}
//   PUT  /valuation/admin/tonnages/{id}
//   DELETE /valuation/admin/tonnages/{id}
//   PUT  /valuation/admin/mast-types/{id}
//   DELETE /valuation/admin/mast-types/{id}
//   PUT  /valuation/admin/mast-heights/{id}
//   DELETE /valuation/admin/mast-heights/{id}
//   PUT  /valuation/admin/battery-types/{id}
//   DELETE /valuation/admin/battery-types/{id}
//   PUT  /valuation/admin/transmission-types/{id}
//   DELETE /valuation/admin/transmission-types/{id}
//   PUT  /valuation/admin/engine-types/{id}
//   DELETE /valuation/admin/engine-types/{id}
//   DELETE /valuation/admin/condition-ratings/{id}
//   DELETE /valuation/admin/region-coefficients/{id}
//
// 覆盖的 Go 类型：BatteryEvaluation / BatteryEvaluationSummary / BatteryType / CreateBatteryResponse / CycleFeature / DimensionScore / EvaluationDetail / EvaluationResponse / FeatureImportance / ListBatteryResponse / RawStats / AlgorithmParameters / BatteryTypeDict / Brand / CoefficientConfig / ConditionRating / ConfigOption / EngineType / MastHeight / MastType / OriginalPrice / RegionCoefficient / Series / SeriesConfigOptions / Tonnage / TransmissionType / VehicleType
//
// 可空性 / 缺省态由**注解层**表达，生成器只如实转写（Go 结构体 tag）：
//   - extensions:"x-nullable" → 字段渲染 'T | null'：键一定在，值为 null（Go 指针且无 omitempty）；
//   - extensions:"x-optional" → 字段渲染 'T?'：键**可能整个不存在**（Go omitempty）；
//   - 两者可同时标注（'T?' 且 '| null'）；未标注的一律按「键一定在、非 null」渲染 ——
//     swag 看不到 Go 的 omitempty，漏标即契约撒谎。
// 其余已知限制：
//   - Go 侧 any 字段在 swagger 里是空 schema，渲染 'unknown'（不猜结构）；
//   - 不生成 query / body 的入参类型（只生成响应形状）。
// 需要更精确的形状时先在注解层补齐（先例见 spec #940 片五②的差集清单）。

export interface BatteryEvaluation {
  battery_model: string
  battery_type: BatteryType
  confidence: number
  confidence_high: number
  confidence_low: number
  created_at: string
  cycle_count: number
  cycle_features?: CycleFeature[]
  feature_importance?: FeatureImportance[]
  id: number
  report_pdf_path: string
  rul_cycles: number
  soh_percent: number
  suggestions?: string[]
  updated_at: string
}

export interface BatteryEvaluationSummary {
  battery_model: string
  battery_type: BatteryType
  confidence: number
  created_at: string
  cycle_count: number
  id: number
  rul_cycles: number
  soh_percent: number
}

export interface BatteryType {
}

export interface CreateBatteryResponse {
  battery_type: BatteryType
  confidence: number
  confidence_high: number
  confidence_low: number
  created_at: string
  cycle_count: number
  evaluation_id: number
  rul_cycles: number
  soh_percent: number
  suggestions: string[]
}

export interface CycleFeature {
  cycle_index: number
  evaluation_id: number
  feature_vector: number[]
  id?: number
  raw_stats: RawStats
  soh_at_cycle: number
}

export interface DimensionScore {
  label: string
  value: number
}

export interface EvaluationDetail {
  brand: string
  city: string
  condition_rating: string
  confidence_high: number
  confidence_low: number
  config_type: string
  created_at: string
  decay_anchor: number
  dimension_scores: DimensionScore[]
  estimated_value: number
  factory_year: number
  has_license_plate: boolean
  has_maintenance_records: boolean
  has_registration_certificate: boolean
  id: number
  k_brand: number
  k_condition: number
  k_hours: number
  k_market: number
  k_time: number
  k_time_adjusted: number
  lambda_combustion: number
  lambda_electric: number
  mast_height_mm: number
  mast_type: string
  original_paint: boolean
  original_price: number
  province: string
  report_pdf_path?: string
  sale_year: number
  series: string
  suggestions: string[]
  tonnage: number
  updated_at: string
  usage_hours: number
  vehicle_type: string
}

export interface EvaluationResponse {
  brand: string
  city: string
  condition_rating: string
  confidence_high: number
  confidence_low: number
  config_type: string
  decay_anchor: number
  dimension_scores: DimensionScore[]
  estimated_value: number
  factory_year: number
  has_license_plate: boolean
  has_maintenance_records: boolean
  has_registration_certificate: boolean
  id: number
  k_brand: number
  k_condition: number
  k_hours: number
  k_market: number
  k_time: number
  k_time_adjusted: number
  lambda_combustion: number
  lambda_electric: number
  mast_height_mm: number
  mast_type: string
  original_paint: boolean
  original_price: number
  province: string
  sale_year: number
  series: string
  suggestions: string[]
  tonnage: number
  usage_hours: number
  vehicle_type: string
}

export interface FeatureImportance {
  group: string
  index: number
  name: string
  normalized: number
  weight: number
}

export interface ListBatteryResponse {
  items: BatteryEvaluationSummary[]
  total: number
}

export interface RawStats {
  capacity: number
  cc_duration: number
  current_mean: number
  current_std: number
  cv_duration: number
  ic_peak: number
  ic_peak_voltage: number
  voltage_mean: number
  voltage_std: number
}

export interface AlgorithmParameters {
  brands: Brand[]
  coefficients: CoefficientConfig[]
  condition_ratings: ConditionRating[]
  region_coefficients: RegionCoefficient[]
}

export interface BatteryTypeDict {
  id: number
  name: string
}

export interface Brand {
  id: number
  is_active: boolean
  k_brand: number
  name: string
}

export interface CoefficientConfig {
  description: string
  id: number
  key: string
  updated_at: string
  value: number
}

export interface ConditionRating {
  base_coefficient: number
  id: number
  label: string
  rating: string
}

export interface ConfigOption {
  id: number
  name: string
}

export interface EngineType {
  id: number
  name: string
}

export interface MastHeight {
  id: number
  value_mm: number
}

export interface MastType {
  id: number
  name: string
}

export interface OriginalPrice {
  brand: string
  config_type: string
  earliest_factory_year: number
  id: number
  mast_height_mm: number
  mast_type: string
  original_price: number
  series: string
  tonnage: number
  updated_at: string
  vehicle_type: string
}

export interface RegionCoefficient {
  city: string
  coefficient: number
  id: number
  province: string
}

export interface Series {
  brand: string
  earliest_factory_year: number
  id: number
  name: string
}

export interface SeriesConfigOptions {
  battery: string[]
  engine: string[]
  transmission: string[]
}

export interface Tonnage {
  id: number
  value: number
}

export interface TransmissionType {
  id: number
  name: string
}

export interface VehicleType {
  earliest_factory_year: number
  id: number
  name: string
  power_type: string
}
