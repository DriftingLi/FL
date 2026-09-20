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

  /** 装载（首屏/翻页/筛选变化共用）：错误收敛为 loadError，绝不 reject */
  async function load() {
    loading.value = true
    loadError.value = false
    loadErrorKind.value = null
    try {
      const payload: Record<string, unknown> = { ...filters.value }
      if (options.searchable && searchKeyword.value) {
        payload.keyword = searchKeyword.value
      }
      const result = await options.fetch({ page: currentPage.value, pageSize: pageSize.value }, payload)
      // 兜底口径不变（票 6 只搬容器宿主）：api 层的 toPage 已归一，这里对空/缺响应再兜一层
      list.value = result.items || []
      total.value = result.total || 0
    } catch (error) {
      // 错误态由 loadError 承载（拦截器已统一 toast），不向上抛：调用点常不 await load()
      loadError.value = true
      loadErrorKind.value = kindOf(error)
    } finally {
      loading.value = false
      retrying.value = false
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
