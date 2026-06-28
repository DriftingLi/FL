// 品牌类型与品牌字典类型
// 重构说明：Brand 字段从 {tier, models} 改为 {brand_type, k_brand, is_active}
//         新增 BrandTypeOption 已统一在 evaluation.ts 中定义，这里仅保留 Brand
import type { BrandTypeOption } from './evaluation'

// 重新导出避免历史引用断裂（如 brand.ts 被 import { BrandTypeOption })
export type { BrandTypeOption }

/** 品牌字典项（GET /brands?brand_type=xxx 响应元素） */
export interface Brand {
  id: number
  name: string
  /** 所属品牌类型（与 brand_types.name 对应） */
  brand_type: string
  /** 品牌系数 */
  k_brand: number
  /** 是否启用 */
  is_active: boolean
}
