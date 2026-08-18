// useAdminTable：管理端列表状态机的接口级测试。
// seam：composable 接口——fetch 用内存 fixture，actions 用内存 stub，不触达 API 层。
import { describe, it, expect, vi } from 'vitest'
import { ElMessageBox } from 'element-plus'
import { useAdminTable } from '@/composables/useAdminTable'

interface Row {
  id: number
  name: string
}

function makeTable() {
  const seen: Array<{ page: number; pageSize: number; filters: Record<string, unknown> }> = []
  const table = useAdminTable<Row>({
    fetch: async (paging, filters) => {
      seen.push({ page: paging.page, pageSize: paging.pageSize, filters })
      return {
        list: [{ id: paging.page, name: `row-${paging.page}` }],
        total: 25
      }
    },
    actions: {
      rename: row => {
        row.name = `renamed-${row.id}`
      }
    },
    searchable: true
  })
  return { table, seen }
}

describe('useAdminTable（admin 列表状态机）', () => {
  it('load 调用 fetch 并写入 list/total', async () => {
    const { table, seen } = makeTable()

    await table.load()

    expect(seen).toEqual([{ page: 1, pageSize: 10, filters: {} }])
    expect(table.list.value).toHaveLength(1)
    expect(table.list.value[0].id).toBe(1)
    expect(table.total.value).toBe(25)
    expect(table.loading.value).toBe(false)
  })

  it('search 回到第一页并携带 keyword', async () => {
    const { table, seen } = makeTable()
    table.currentPage.value = 3
    table.searchKeyword.value = '张三'

    await table.search()

    expect(table.currentPage.value).toBe(1)
    expect(seen[0].filters.keyword).toBe('张三')
  })

  it('applyFilters 替换 filters 并回到第一页', async () => {
    const { table, seen } = makeTable()
    table.currentPage.value = 4

    await table.applyFilters({ status: '1' })

    expect(table.currentPage.value).toBe(1)
    expect(seen[0].filters).toEqual({ status: '1' })
  })

  it('handleAction 分发到注入的 action', async () => {
    const { table } = makeTable()
    const row = { id: 2, name: 'a' }

    await table.handleAction('rename', row)

    expect(row.name).toBe('renamed-2')
  })

  it('confirmDelete 确认后执行 action 并刷新列表', async () => {
    vi.spyOn(ElMessageBox, 'confirm').mockResolvedValue('confirm' as never)
    const { table, seen } = makeTable()
    const action = vi.fn()

    await table.confirmDelete({ id: 1, name: 'a' }, action, '确定删除？')

    expect(action).toHaveBeenCalled()
    expect(seen.length).toBe(1)
  })

  it('confirmDelete 取消时不执行 action、不刷新', async () => {
    vi.spyOn(ElMessageBox, 'confirm').mockRejectedValue('cancel' as never)
    const { table, seen } = makeTable()
    const action = vi.fn()

    await table.confirmDelete({ id: 1, name: 'a' }, action, '确定删除？')

    expect(action).not.toHaveBeenCalled()
    expect(seen.length).toBe(0)
  })
})
