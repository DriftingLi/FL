// useAdminTable：管理端标准列表页状态机 module（ADR-0015）。
// interface：fetch + actions + searchable；implementation 拥有
// loading / list / total / currentPage / pageSize / search / action 分发 / confirmDelete。
// 页面只声明 fetch adapter 与行操作 adapter。
import { ref } from 'vue'
import { ElMessageBox } from 'element-plus'

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
  const list = ref<T[]>([])
  const total = ref(0)
  const currentPage = ref(1)
  const pageSize = ref(options.pageSize ?? 10)
  const searchKeyword = ref('')
  const filters = ref<Record<string, unknown>>({})

  async function load() {
    loading.value = true
    try {
      const payload: Record<string, unknown> = { ...filters.value }
      if (options.searchable && searchKeyword.value) {
        payload.keyword = searchKeyword.value
      }
      const result = await options.fetch({ page: currentPage.value, pageSize: pageSize.value }, payload)
      list.value = result.list || []
      total.value = result.total || 0
    } finally {
      loading.value = false
    }
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
      await ElMessageBox.confirm(message, '提示', {
        type: 'warning',
        confirmButtonText: '确定',
        cancelButtonText: '取消'
      })
    } catch {
      return
    }
    await action(row)
    await load()
  }

  return {
    loading,
    list,
    total,
    currentPage,
    pageSize,
    searchKeyword,
    filters,
    load,
    search,
    applyFilters,
    reset,
    handleAction,
    confirmDelete
  }
}
