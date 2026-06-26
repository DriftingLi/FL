// 品牌类型
import type { BrandTier } from './evaluation'

export interface Brand {
  id: number
  name: string
  tier: BrandTier
  k_brand: number
  is_active: boolean
  /** 该品牌下的常见型号（用于前端下拉） */
  models: string[]
}
