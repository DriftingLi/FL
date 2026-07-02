// 品牌字典类型
// 重构说明：Brand 字段移除 brand_type（已删除 brand_types 表）

/** 品牌字典项（GET /brands 响应元素） */
export interface Brand {
  id: number
  name: string
  /** 品牌系数 */
  k_brand: number
  /** 是否启用 */
  is_active: boolean
}
