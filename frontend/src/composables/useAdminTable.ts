// useAdminTable：管理端标准列表页状态机 module（ADR-0015；三态补齐见 ADR-0039）。
// interface：fetch + actions + searchable；implementation 拥有
// loading / loadError / retrying / list / total / currentPage / pageSize / search /
// action 分发 / confirmDelete。页面只声明 fetch adapter 与行操作 adapter。
//
// 三态语义对齐 useAsyncPage（#790 A1，ADR-0039）：load 失败收敛为 loadError、
// **绝不向上 reject**（此前 try/finally 无 catch，未 await 的调用点会产生
// unhandled rejection）；retry 带 retrying 防重入，可直接驱动错误态重试按钮。
// admin 域统一用本件，非 admin 场景仍用 useAsyncPage（全站 40 处）。
//
// 第十一波 #1102（ADR-0056 §9「admin 多列表页的第二档写明归属」）：本件补齐**列表档
// 与只读档共用的那一条空态判据**——isEmpty 与 useAsyncPage.isEmpty 走同一份实现
// （utils/listState.isEmptyList），loadErrorKind 与 useAsyncPage 同名同义（404 = 空态）。
// 于是「分页列表 → useAdminTable；只读/计数 → useAsyncPage」两档的空态与错误语义一致，
// 页面不必再为走本件的那一档手写 list.length === 0（同一条不变式的第二份实现）。
//
// 第十三波票 6（ADR-0060 决策 6）：分页容器归位到 api 侧——本件的 fetch 契约吃 `Page<T>`
// （@/api/page），旧的 AdminTableListResult 副本随之删除。「这一页的行在响应里叫什么键」
// （topics / questions / tutors / requests / list …）是各域 api 模块出口之前的事，页面只
// `return someApi.listX(...)`。不读服务端分页的页面（岗位字典、原价表）在自己出口处
// `toPage(rows, rows.length)` 造一个容器，本件不为它们开特例。
//
// #1354（ADR-0069 实施回写段登记的「同病同类」）：装载批次代数。此前 `load()` 在
// `await options.fetch(...)` 之后**无条件**写 `list`/`total`，两次装载重叠时后到的旧响应
// 盖掉新状态（筛选词条是新的、列表是旧的；页码与内容不同批）—— 与真实缺陷 #5 逐字同形。
// 本件**不复用 useAsyncPage**：admin 档的 interface 拥有 list/total/currentPage/pageSize/filters
// 与 action 分发，且服务端参数驱动分页（currentPage + pageSize，无 credentialScoped /
// filterDeps / append 那一整面），合并两档会把学员端列表页的判据拖进 admin。搬过来的是
// **那一条判据本身**（同名 `generation`、同形的 `invalidate()`、同一位置：写回之前），
// 机检见 `__tests__/loadFlowLocks.spec.ts` R5。
import { computed, ref } from 'vue'
import { useConfirm } from '@/composables/useConfirm'
// 只取类型：client.ts 会建 axios 实例，不因这条依赖把网络侧带进 composable 的运行时
import type { ApiErrorKind } from '@/api/client'
// 分页容器的宿主在 api 侧（ADR-0060 决策 6 / 票 6）：composable 依赖 @/api/page 是**方向正确**
// 的那一条边（反向的 api → composable 已被 ADR 否掉）。本件只吃 Page<T>，不再自带容器副本。
import type { Page } from '@/api/page'
import { isEmptyList } from '@/utils/listState'

export interface AdminTablePaging {
  page: number
  pageSize: number
}

export interface AdminTableOptions<T> {
  /** 页面只负责挑 api 模块里那条列表函数并透传分页/筛选参数，容器由 api 层出口给（票 6）。 */
  fetch: (paging: AdminTablePaging, filters: Record<string, unknown>) => Promise<Page<T>>
  actions?: Record<string, (row: T) => void | Promise<void>>
  searchable?: boolean
  pageSize?: number
}

export function useAdminTable<T>(options: AdminTableOptions<T>) {
  const loading = ref(false)
  const loadError = ref(false)
  /** 最近一次装载失败的错误分类（ApiErrorKind，成功时清空）；404 归空态见 isEmpty。 */
  const loadErrorKind = ref<ApiErrorKind | null>(null)
  const retrying = ref(false)
  const list = ref<T[]>([])
  const total = ref(0)
  const currentPage = ref(1)
  const pageSize = ref(options.pageSize ?? 10)
  const searchKeyword = ref('')
  const filters = ref<Record<string, unknown>>({})

  /** 拦截器把 ApiErrorKind 挂在错误对象上（client.ts attachKind）；无 kind 视为未分类。 */
  function kindOf(error: unknown): ApiErrorKind | null {
    const kind = (error as { kind?: ApiErrorKind } | null)?.kind
    return typeof kind === 'string' ? kind : null
  }

  /**
   * 批次代数（#1354，判据同 useAsyncPage.generation / ADR-0069 决策 1）：`load` 起飞时递增并捕获，
   * **落地前比对** —— 不等即整轮作废：`list`/`total` 不写回、错误态不改写、`loading`/`retrying`
   * 也不由旧轮归零（旧轮归零会把新轮在飞的 spinner 熄掉）。
   * 缺它时的实测形态与真实缺陷 #5 同形：搜索框回车 / 连点筛选 / 删完刷新这些入口都会连发 `load()`，
   * 先起飞的慢响应后到时把新一轮的筛选结果换成旧筛选的行。
   */
  let generation = 0

  /** 一轮装载起飞前的作废动作：旧轮（含在飞响应）就此不落地。 */
  function invalidate(): number {
    generation += 1
    return generation
  }

  /** 装载（首屏/翻页/筛选变化共用）：错误收敛为 loadError，绝不 reject */
  async function load() {
    const gen = invalidate()
    loading.value = true
    loadError.value = false
    loadErrorKind.value = null
    try {
      const payload: Record<string, unknown> = { ...filters.value }
      if (options.searchable && searchKeyword.value) {
        payload.keyword = searchKeyword.value
      }
      const result = await options.fetch({ page: currentPage.value, pageSize: pageSize.value }, payload)
      // 更新的一轮已起飞（再点筛选 / 搜索回车 / 翻页 / 删完刷新）：本轮结果整体作废。
      // 代数校验在写回**之前**（ADR-0069 决策 1 那条判据）：旧轮连 `list`/`total` 都不碰，
      // 不是「先写回再纠正」—— 中间那一帧新筛选配旧行是可看见的。
      if (gen !== generation) return
      // 容器已由 api 层出口保证（Page<T> 的 items/total 非空，兜底单点在 toPage，见 api/page.ts）；
      // 这里再兜一层就是同一判据的第二宿主。
      list.value = result.items
      total.value = result.total
    } catch (error) {
      // 旧轮的失败不属于本轮：新一轮可能已经成功落地，这里把它打成错误态就是假故障
      if (gen !== generation) return
      // 错误态由 loadError 承载（拦截器已统一 toast），不向上抛：调用点常不 await load()
      loadError.value = true
      loadErrorKind.value = kindOf(error)
    } finally {
      // 同样只在本轮仍是最新一轮时才收灯：旧轮归零会让在飞的新轮失去 loading 态
      if (gen === generation) {
        loading.value = false
        retrying.value = false
      }
    }
  }

  /** 错误态重试：retrying 防重入，可直接作为重试按钮 loading */
  async function retry(): Promise<void> {
    if (retrying.value) return
    retrying.value = true
    await load()
  }

  /** 搜索：回到第一页并重新加载。 */
  function search() {
    currentPage.value = 1
    return load()
  }

  /** 页面声明式应用 filters：替换 filters 并回到第一页。 */
  function applyFilters(next: Record<string, unknown>) {
    filters.value = { ...next }
    currentPage.value = 1
    return load()
  }

  /** 清空搜索词与 filters 并回到第一页。 */
  function reset() {
    searchKeyword.value = ''
    filters.value = {}
    currentPage.value = 1
    return load()
  }

  /** 行操作分发：只调用页面注入的 actions，不自行拼装业务分支。 */
  async function handleAction(cmd: string, row: T): Promise<void> {
    const action = options.actions?.[cmd]
    if (action) {
      await action(row)
    }
  }

  /** 通用删除确认：确认后执行 action 并刷新列表；取消静默。 */
  async function confirmDelete(row: T, action: (row: T) => void | Promise<void>, message: string): Promise<void> {
    try {
      await useConfirm().confirmDanger(message, '提示')
    } catch {
      return
    }
    await action(row)
    await load()
  }

  /**
   * 空态判据（第十一波 #1102）：喂 UiAsyncSection 的 empty prop 的默认路径，
   * 与 useAsyncPage.isEmpty 同源（utils/listState.isEmptyList）——「加载成功且没有条目」
   * 或「后端 404」才为真，装载中恒为 false。走本档的页面不再手写 list.length === 0。
   */
  const isEmpty = computed(() =>
    isEmptyList(list.value, { error: loadError.value, kind: loadErrorKind.value })
  )

  return {
    loading,
    loadError,
    loadErrorKind,
    retrying,
    list,
    isEmpty,
    total,
    currentPage,
    pageSize,
    searchKeyword,
    filters,
    load,
    retry,
    search,
    applyFilters,
    reset,
    handleAction,
    confirmDelete
  }
}
