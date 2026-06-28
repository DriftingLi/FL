// 字典 API：封装所有学生端字典查询接口（只读 GET）
// 后端契约：返回扁平数组，前端直接消费；不写入/不缓存到模块级（避免脏数据）
import client from './client'
import type {
  BrandTypeOption,
  VehicleTypeOption,
  SeriesOption,
  TonnageOption,
  ConfigTypeOption,
  MastTypeOption,
  MastHeightOption,
  BatteryTypeOption,
  ConditionRatingOption,
  CoefficientConfig
} from '@/types/valuation/evaluation'

/** 品牌类型 */
export async function listBrandTypes(): Promise<BrandTypeOption[]> {
  const resp = await client.get<unknown, { data: BrandTypeOption[] }>('/dictionaries/brand-types')
  return resp.data ?? []
}

/** 品牌（按品牌类型过滤） */
export async function listBrandsByType(brand_type: string): Promise<
  Array<{ id: number; name: string; brand_type: string; k_brand: number; is_active: boolean }>
> {
  const resp = await client.get<unknown, { data: Array<{ id: number; name: string; brand_type: string; k_brand: number; is_active: boolean }> }>('/dictionaries/brands', {
    params: { brand_type }
  })
  return resp.data ?? []
}

/** 车辆类型 */
export async function listVehicleTypes(): Promise<VehicleTypeOption[]> {
  const resp = await client.get<unknown, { data: VehicleTypeOption[] }>('/dictionaries/vehicle-types')
  return resp.data ?? []
}

/** 系列（按品牌过滤） */
export async function listSeries(brand: string): Promise<SeriesOption[]> {
  const resp = await client.get<unknown, { data: SeriesOption[] }>('/dictionaries/series', {
    params: { brand }
  })
  return resp.data ?? []
}

/** 吨位 */
export async function listTonnages(): Promise<TonnageOption[]> {
  const resp = await client.get<unknown, { data: TonnageOption[] }>('/dictionaries/tonnages')
  return resp.data ?? []
}

/** 配置类型 */
export async function listConfigTypes(): Promise<ConfigTypeOption[]> {
  const resp = await client.get<unknown, { data: ConfigTypeOption[] }>('/dictionaries/config-types')
  return resp.data ?? []
}

/** 门架类型 */
export async function listMastTypes(): Promise<MastTypeOption[]> {
  const resp = await client.get<unknown, { data: MastTypeOption[] }>('/dictionaries/mast-types')
  return resp.data ?? []
}

/** 门架高度 */
export async function listMastHeights(): Promise<MastHeightOption[]> {
  const resp = await client.get<unknown, { data: MastHeightOption[] }>('/dictionaries/mast-heights')
  return resp.data ?? []
}

/** 电池类型 */
export async function listBatteryTypes(): Promise<BatteryTypeOption[]> {
  const resp = await client.get<unknown, { data: BatteryTypeOption[] }>('/dictionaries/battery-types')
  return resp.data ?? []
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
