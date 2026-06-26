// 部件配置加载与缓存的 composable
// 避免 Electric/Combustion 录入页重复请求
import { ref, type Ref } from 'vue'
import { getPartConfigs } from '@/api/valuation/partConfigs'
import type { ForkliftType } from '@/types/valuation/evaluation'
import type { PartConfigList } from '@/types/valuation/condition'

const cache: Map<ForkliftType, PartConfigList> = new Map()
const loadingMap: Map<ForkliftType, Ref<boolean>> = new Map()
const dataMap: Map<ForkliftType, Ref<PartConfigList | null>> = new Map()

export function usePartConfigs(type: ForkliftType) {
  // 初始化缓存槽
  if (!loadingMap.has(type)) loadingMap.set(type, ref(false))
  if (!dataMap.has(type)) dataMap.set(type, ref(null))

  const loading = loadingMap.get(type)!
  const data = dataMap.get(type)!

  async function load(force = false) {
    if (cache.has(type) && !force) {
      data.value = cache.get(type) ?? null
      return data.value
    }
    loading.value = true
    try {
      const list = await getPartConfigs(type)
      cache.set(type, list)
      data.value = list
      return list
    } finally {
      loading.value = false
    }
  }

  return { data, loading, load }
}

/** 工具：根据 part configs 收集所有 item_code */
export function collectItemCodes(list: PartConfigList | null | undefined): string[] {
  if (!list) return []
  const codes: string[] = []
  for (const cat of list) {
    for (const it of cat.items) codes.push(it.item_code)
  }
  return codes
}

/** 工具：总数（条目） */
export function countItems(list: PartConfigList | null | undefined): number {
  return collectItemCodes(list).length
}
