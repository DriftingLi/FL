// 字典 API：封装所有学生端字典查询接口（只读 GET）
// 后端契约：返回扁平数组，前端直接消费；不写入/不缓存到模块级（避免脏数据）
// 级联契约：vehicle_types / series / tonnages / config_types / mast_types / mast_heights
//           均支持按前序已选层级过滤，参数全传时走 original_prices DISTINCT 查询
import client from './client'
import type {
  VehicleTypeOption,
  SeriesOption,
  TonnageOption,
  ConfigTypeOption,
  MastTypeOption,
  MastHeightOption,
  BatteryTypeOption,
  TransmissionTypeOption,
  EngineTypeOption,
  SeriesConfigOptions,
  ConditionRatingOption,
  CoefficientConfig
} from '@/types/valuation/evaluation'
import type { Brand } from '@/types/valuation/brand'

/** 全部品牌（按 k_brand 倒序） */
export async function listBrands(): Promise<Brand[]> {
  const resp = await client.get<unknown, { data: Brand[] }>('/dictionaries/brands')
  return resp.data ?? []
}

/** 车辆类型（按品牌级联过滤） */
export async function listVehicleTypes(brand?: string): Promise<VehicleTypeOption[]> {
  const resp = await client.get<unknown, { data: VehicleTypeOption[] }>('/dictionaries/vehicle-types', {
    params: brand ? { brand } : undefined
  })
  return resp.data ?? []
}

/** 系列（按品牌+车辆类型级联过滤） */
export async function listSeries(brand?: string, vehicleType?: string): Promise<SeriesOption[]> {
  const resp = await client.get<unknown, { data: SeriesOption[] }>('/dictionaries/series', {
    params: { brand, vehicle_type: vehicleType }
  })
  return resp.data ?? []
}

/** 吨位（按品牌+车辆类型+系列级联过滤） */
export async function listTonnages(brand?: string, vehicleType?: string, series?: string): Promise<TonnageOption[]> {
  const resp = await client.get<unknown, { data: TonnageOption[] }>('/dictionaries/tonnages', {
    params: { brand, vehicle_type: vehicleType, series }
  })
  return resp.data ?? []
}

/** 配置类型（按前序层级级联过滤） */
export async function listConfigTypes(
  brand?: string, vehicleType?: string, series?: string, tonnage?: number | string
): Promise<ConfigTypeOption[]> {
  const resp = await client.get<unknown, { data: ConfigTypeOption[] }>('/dictionaries/config-types', {
    params: { brand, vehicle_type: vehicleType, series, tonnage }
  })
  return resp.data ?? []
}

/** 门架类型（按前序层级级联过滤） */
export async function listMastTypes(
  brand?: string, vehicleType?: string, series?: string, tonnage?: number | string, configType?: string
): Promise<MastTypeOption[]> {
  const resp = await client.get<unknown, { data: MastTypeOption[] }>('/dictionaries/mast-types', {
    params: { brand, vehicle_type: vehicleType, series, tonnage, config_type: configType }
  })
  return resp.data ?? []
}

/** 门架高度（按前序层级级联过滤） */
export async function listMastHeights(
  brand?: string, vehicleType?: string, series?: string, tonnage?: number | string,
  configType?: string, mastType?: string
): Promise<MastHeightOption[]> {
  const resp = await client.get<unknown, { data: MastHeightOption[] }>('/dictionaries/mast-heights', {
    params: { brand, vehicle_type: vehicleType, series, tonnage, config_type: configType, mast_type: mastType }
  })
  return resp.data ?? []
}

/** 电池类型（按品牌+车型+系列+吨位级联过滤；不传参数时返回全部） */
export async function listBatteryTypes(
  brand?: string, vehicleType?: string, series?: string, tonnage?: number | string
): Promise<BatteryTypeOption[]> {
  const resp = await client.get<unknown, { data: BatteryTypeOption[] }>('/dictionaries/battery-types', {
    params: { brand, vehicle_type: vehicleType, series, tonnage }
  })
  return resp.data ?? []
}

/** 传动系统字典（手波/自波/无级变速/无） */
export async function listTransmissionTypes(): Promise<TransmissionTypeOption[]> {
  const resp = await client.get<unknown, { data: TransmissionTypeOption[] }>('/dictionaries/transmission-types')
  return resp.data ?? []
}

/** 发动机类型字典（国产发动机/进口发动机/混合动力/无） */
export async function listEngineTypes(): Promise<EngineTypeOption[]> {
  const resp = await client.get<unknown, { data: EngineTypeOption[] }>('/dictionaries/engine-types')
  return resp.data ?? []
}

/** 系列配置选项（按品牌+系列查询三维度可选项；数组为空表示该 series 不支持此维度） */
export async function listSeriesConfigOptions(brand: string, series: string): Promise<SeriesConfigOptions> {
  const resp = await client.get<unknown, { data: SeriesConfigOptions }>('/dictionaries/series-config-options', {
    params: { brand, series }
  })
  return resp.data ?? { transmission: [], engine: [], battery: [] }
}

/** 车况评级 */
export async function listConditionRatings(): Promise<ConditionRatingOption[]> {
  const resp = await client.get<unknown, { data: ConditionRatingOption[] }>('/dictionaries/condition-ratings')
  return resp.data ?? []
}

/** 省份列表 */
export async function listProvinces(): Promise<string[]> {
  const resp = await client.get<unknown, { data: string[] }>('/dictionaries/provinces')
  return resp.data ?? []
}

/** 城市列表（按省份过滤） */
export async function listCities(province: string): Promise<string[]> {
  const resp = await client.get<unknown, { data: string[] }>('/dictionaries/cities', {
    params: { province }
  })
  return resp.data ?? []
}

/** 算法参数（系数表） */
export async function listCoefficients(): Promise<CoefficientConfig[]> {
  const resp = await client.get<unknown, { data: CoefficientConfig[] }>('/dictionaries/coefficient-configs')
  return resp.data ?? []
}
