// 品牌 API（按 brand_type 过滤；模块级内存缓存，避免每次进 InputView 都请求）
// 重构说明：从无参 listBrands 改为按品牌类型过滤
import client from './client'
import type { Brand } from '@/types/valuation/brand'

const cache = new Map<string, Brand[]>()
const inflightMap = new Map<string, Promise<Brand[]>>()

/** 拉取指定品牌类型下的所有品牌 */
export async function listBrands(brandType: string, force = false): Promise<Brand[]> {
  if (!force && cache.has(brandType)) return cache.get(brandType)!
  if (inflightMap.has(brandType)) return inflightMap.get(brandType)!

  const inflight = client
    .get<unknown, { data: Brand[] }>('/brands', {
      params: { brand_type: brandType }
    })
    .then((r) => {
      const list = r.data ?? []
      cache.set(brandType, list)
      return list
    })
    .finally(() => {
      inflightMap.delete(brandType)
    })

  inflightMap.set(brandType, inflight)
  return inflight
}

/** 调试用：清空全部缓存 */
export function clearBrandsCache(): void {
  cache.clear()
  inflightMap.clear()
}
