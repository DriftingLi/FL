// 部件配置相关类型
import type { ItemStatus } from './evaluation'

/** 部件条目定义（不含状态） */
export interface PartItemDef {
  item_code: string
  item_name: string
  item_weight: number
}

/** 部件类别（含条目） */
export interface PartCategory {
  category_code: string
  category_name: string
  category_weight: number
  items: PartItemDef[]
}

/** 部件配置完整结构 */
export type PartConfigList = PartCategory[]

/** 部件状态（录入时的运行时结构） */
export interface ItemStatusEntry {
  item_code: string
  status: ItemStatus
}
