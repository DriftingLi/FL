// 品牌 API（模块级内存缓存，避免每次进 InputView 都请求）
import client from './client'
import type { Brand } from '@/types/valuation/brand'

let cache: Brand[] | null = null
let inflight: Promise<Brand[]> | null = null

export async function listBrands(force = false): Promise<Brand[]> {
  if (!force && cache) return cache
  if (inflight) return inflight

  inflight = client
    .get<unknown, { data: Brand[] }>('/brands')
    .then((r) => {
      cache = r.data ?? []
      return cache
    })
    .finally(() => {
      inflight = null
    })

  return inflight
}

/** 调试用：清空缓存 */
export function clearBrandsCache(): void {
  cache = null
}
