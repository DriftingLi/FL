// useAdminTable：管理端标准列表页状态机 module（ADR-0015；三态补齐见 ADR-0039）。
// interface：fetch + actions + searchable；implementation 拥有
// loading / loadError / retrying / list / total / currentPage / pageSize / search /
// action 分发 / confirmDelete。页面只声明 fetch adapter 与行操作 adapter。
//
// 三态语义对齐 useAsyncPage（#790 A1，ADR-0039）：load 失败收敛为 loadError、
// **绝不向上 reject**（此前 try/finally 无 catch，未 await 的调用点会产生
// unhandled rejection）；retry 带 retrying 防重入，可直接驱动错误态重试按钮。
// admin 域统一用本件，非 admin 场景仍用 useAsyncPage（全站 40 处）。
import { ref } from 'vue'
import { useConfirm } from '@/composables/useConfirm'

export interface AdminTablePaging {
  page: number
  pageSize: number
}

export interface AdminTableListResult<T> {
  list: T[]
  total: number
}

export interface AdminTableOptions<T> {
  fetch: (paging: AdminTablePaging, filters: Record<string, unknown>) => Promise<AdminTableListResult<T>>
  actions?: Record<string, (row: T) => void | Promise<void>>
  searchable?: boolean
  pageSize?: number
}

export function useAdminTable<T>(options: AdminTableOptions<T>) {
  const loading = ref(false)
  const loadError = ref(false)
  const retrying = ref(false)
  const list = ref<T[]>([])
  const total = ref(0)
  const currentPage = ref(1)
  const pageSize = ref(options.pageSize ?? 10)
  const searchKeyword = ref('')
  const filters = ref<Record<string, unknown>>({})

  /** 装载（首屏/翻页/筛选变化共用）：错误收敛为 loadError，绝不 reject */
  async function load() {
    loading.value = true
    loadError.value = false
    try {
      const payload: Record<string, unknown> = { ...filters.value }
      if (options.searchable && searchKeyword.value) {
        payload.keyword = searchKeyword.value
      }
      const result = await options.fetch({ page: currentPage.value, pageSize: pageSize.value }, payload)
      list.value = result.list || []
      total.value = result.total || 0
    } catch {
      // 错误态由 loadError 承载（拦截器已统一 toast），不向上抛：调用点常不 await load()
      loadError.value = true
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

  return {
    loading,
    loadError,
    retrying,
    list,
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
