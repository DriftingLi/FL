// 部件配置 API
// 后端返回扁平结构（每个 item 一行带 category 字段），前端消费时按 category_code 聚合
import client from './client'
import type { PartConfigList, PartCategory, PartItemDef } from '@/types/valuation/condition'
import type { ForkliftType } from '@/types/valuation/evaluation'

/** 后端扁平结构行 */
interface FlatPartRow {
  category_code: string
  category_name: string
  category_weight: number
  item_code: string
  item_name: string
  item_weight: number
}

/** 将扁平行数组聚合成按 category 分组的嵌套结构 */
function groupByCategory(rows: FlatPartRow[]): PartConfigList {
  const map = new Map<string, PartCategory>()
  // 按 category 出现顺序保留
  for (const r of rows) {
    let cat = map.get(r.category_code)
    if (!cat) {
      cat = {
        category_code: r.category_code,
        category_name: r.category_name,
        category_weight: r.category_weight,
        items: []
      }
      map.set(r.category_code, cat)
    }
    const item: PartItemDef = {
      item_code: r.item_code,
      item_name: r.item_name,
      item_weight: r.item_weight
    }
    cat.items.push(item)
  }
  return Array.from(map.values())
}

/** 拉取某类型叉车的全部部件配置（已按 category 聚合） */
export async function getPartConfigs(forkliftType: ForkliftType): Promise<PartConfigList> {
  const resp = await client.get<unknown, { data: FlatPartRow[] }>('/part-configs', {
    params: { forklift_type: forkliftType }
  })
  return groupByCategory(resp.data)
}
