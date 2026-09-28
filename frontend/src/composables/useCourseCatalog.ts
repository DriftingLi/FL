// 课程目录筛选 module：方向/等级双卡片筛选与双向计数联动的唯一实现。
// interface：directions/levels/totalAll/scopedTotal/countOfDirection/countOfLevel/unmountedCount/
//           selectDirection/selectLevel/loadCatalog/applyCatalog/fetchCatalog/levelNameOf/specialtyNameOf。
// data-source adapter：树模式（学员/导师，treeCatalogAdapter）或扁平模式（管理端，自组 adapter）。
import { ref, computed } from 'vue'
import type { CatalogDirectionNode, CatalogLevel, CatalogTree } from '@/api/training'

/** 计数组条目：specialty_id 为 null 代表未挂载（管理端扁平口径），count 为该位置的课程数权重 */
export interface CatalogCountItem {
  specialty_id: number | null
  level_id: number | null
  count: number
}

export interface CatalogAdapterResult {
  directions: CatalogDirectionNode[]
  levels: CatalogLevel[]
  items: CatalogCountItem[]
}

/**
 * 数据源 adapter。
 *
 * `R` 可以**宽于** `CatalogAdapterResult`：页面若在同一次请求里顺带取回自己要用的行（管理端课程列表
 * 就是同一次 `Promise.all` 里拿到的），把那份数据作为 `R` 的额外键**回传**，而不是在 adapter 体内写
 * 页面 ref（#1355）—— 于是页面侧的写回全在 `apply` 槽里、排在批次代数校验之后。
 */
export interface CourseCatalogAdapter<R extends CatalogAdapterResult = CatalogAdapterResult> {
  load(): Promise<R>
}

export interface CourseCatalogOptions<R extends CatalogAdapterResult = CatalogAdapterResult> {
  adapter: CourseCatalogAdapter<R>
  /** 双向计数联动：等级筛选影响方向卡计数与「全部课程」（学员/导师 true，管理端 false） */
  bidirectional?: boolean
  /** 选中变化回调（页面据此重置页码并重新拉取课程列表） */
  onSelect?: () => void
}

/** 未挂载课程的哨兵选择值（管理端），见 CONTEXT.md「未挂方向/等级的课程」 */
export const UNMOUNTED_SPECIALTY_ID = -1

/** 树模式 adapter：课程目录树展开为计数组，全局等级从树节点去重合并（省一次 /levels 请求） */
export function treeCatalogAdapter(fetchTree: () => Promise<CatalogTree>): CourseCatalogAdapter {
  return {
    async load() {
      const tree = await fetchTree()
      const directions = tree.specialties ?? []
      const levelMap = new Map<number, CatalogLevel>()
      const items: CatalogCountItem[] = []
      for (const d of directions) {
        for (const lv of d.levels ?? []) {
          if (!levelMap.has(lv.level_id)) {
            // 等级字典 = 生成 LevelDict：树节点本来就只有字典字段，直接透传（含 code/created_at/description）
            levelMap.set(lv.level_id, {
              code: lv.code,
              created_at: lv.created_at,
              description: lv.description,
              level_id: lv.level_id,
              name: lv.name,
              sort_order: lv.sort_order,
              status: lv.status
            })
          }
          items.push({ specialty_id: d.specialty_id, level_id: lv.level_id, count: lv.courses?.length ?? 0 })
        }
      }
      const levels = [...levelMap.values()].sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0))
      return { directions, levels, items }
    }
  }
}

export function useCourseCatalog<R extends CatalogAdapterResult = CatalogAdapterResult>(
  options: CourseCatalogOptions<R>
) {
  const { adapter, bidirectional = true, onSelect } = options

  const directions = ref<CatalogDirectionNode[]>([])
  const levels = ref<CatalogLevel[]>([])
  const items = ref<CatalogCountItem[]>([])
  const specialtyId = ref<number | null>(null)
  const levelId = ref<number | null>(null)

  /** 当前方向筛选范围内的计数组条目（-1 = 未挂载课程） */
  const scopedItems = computed(() => {
    if (specialtyId.value === null) return items.value
    if (specialtyId.value === UNMOUNTED_SPECIALTY_ID) return items.value.filter(i => i.specialty_id === null)
    return items.value.filter(i => i.specialty_id === specialtyId.value)
  })

  function sumOf(list: CatalogCountItem[], filter: (i: CatalogCountItem) => boolean): number {
    return list.reduce((acc, i) => acc + (filter(i) ? i.count : 0), 0)
  }

  /** 「全部课程」计数：树模式随等级筛选联动，管理端恒为全量 */
  const totalAll = computed(() => {
    if (!bidirectional) return sumOf(items.value, () => true)
    return sumOf(scopedItems.value, i => levelId.value === null || i.level_id === levelId.value)
  })

  /** 「全部等级」计数：随方向筛选联动 */
  const scopedTotal = computed(() => sumOf(scopedItems.value, () => true))

  /** 各方向计数：树模式随等级筛选联动（只数该方向内匹配等级课程） */
  function countOfDirection(id: number): number {
    return sumOf(items.value, i => {
      if (i.specialty_id !== id) return false
      if (bidirectional && levelId.value !== null && i.level_id !== levelId.value) return false
      return true
    })
  }

  /** 各等级计数：随方向筛选联动 */
  function countOfLevel(id: number): number {
    return sumOf(scopedItems.value, i => i.level_id === id)
  }

  /** 未挂载课程计数（管理端 facet） */
  const unmountedCount = computed(() => sumOf(items.value, i => i.specialty_id === null))

  function selectDirection(id: number | null) {
    specialtyId.value = id
    onSelect?.()
  }

  function selectLevel(id: number | null) {
    levelId.value = id
    onSelect?.()
  }

  /**
   * 只取数出口（#1355）：调 adapter 并**原样回传**，不写本 module 的任何状态。
   * 页面用它配 `useAsyncPage` 的 loader（回传值）+ `apply` 槽（写回），于是「写回只发生在最新的
   * 一轮」这条判据（ADR-0069 决策 1）在委托本 module 的页面上同样成立 —— 此前页面的 loader 只是
   * `await fetchCatalog()`，写回住在 adapter/本 module 里，R4 看不见、R4c 只能登记例外。
   */
  async function loadCatalog(): Promise<R> {
    return await adapter.load()
  }

  /**
   * 写回出口（#1355，与 `useAsyncPage` 的 `apply` 槽同位）：把 `loadCatalog()` 的结果写进本 module
   * 的三个 ref。入参类型只吃 `CatalogAdapterResult` 那一面 —— adapter 回传的额外键属页面自己的状态，
   * 由页面在它的 `apply` 里写回，本 module 不认得也不替它写。
   */
  function applyCatalog(data: CatalogAdapterResult): void {
    directions.value = data.directions
    levels.value = data.levels
    items.value = data.items
  }

  /** 取数 + 写回一体的旧入口（本 module 的 8 条用例与非 seam 调用方用这条；页面走 loadCatalog + applyCatalog）。 */
  async function fetchCatalog(): Promise<void> {
    applyCatalog(await loadCatalog())
  }

  function levelNameOf(id?: number | null): string {
    if (!id) return ''
    return levels.value.find(l => l.level_id === id)?.name || ''
  }

  function specialtyNameOf(id?: number | null): string {
    if (!id) return ''
    return directions.value.find(d => d.specialty_id === id)?.name || ''
  }

  return {
    directions,
    levels,
    specialtyId,
    levelId,
    totalAll,
    scopedTotal,
    countOfDirection,
    countOfLevel,
    unmountedCount,
    selectDirection,
    selectLevel,
    loadCatalog,
    applyCatalog,
    fetchCatalog,
    levelNameOf,
    specialtyNameOf
  }
}
