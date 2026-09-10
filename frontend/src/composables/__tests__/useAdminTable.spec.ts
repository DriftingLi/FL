// useAdminTable：管理端列表状态机的接口级测试。
// seam：composable 接口——fetch 用内存 fixture，actions 用内存 stub，不触达 API 层。
import { describe, it, expect, vi } from 'vitest'

// 删除确认已收口到 useConfirm（#735），mock 掉按「确认/取消」两种路径放行
const { confirmSpy } = vi.hoisted(() => ({ confirmSpy: vi.fn() }))
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: confirmSpy, confirmDanger: confirmSpy, prompt: vi.fn() })
}))

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
    confirmSpy.mockResolvedValue('confirm' as never)
    const { table, seen } = makeTable()
    const action = vi.fn()

    await table.confirmDelete({ id: 1, name: 'a' }, action, '确定删除？')

    expect(action).toHaveBeenCalled()
    expect(seen.length).toBe(1)
  })

  it('confirmDelete 取消时不执行 action、不刷新', async () => {
    confirmSpy.mockRejectedValue('cancel' as never)
    const { table, seen } = makeTable()
    const action = vi.fn()

    await table.confirmDelete({ id: 1, name: 'a' }, action, '确定删除？')

    expect(action).not.toHaveBeenCalled()
    expect(seen.length).toBe(0)
  })
  // ---- 三态（#790 A1）：对齐 useAsyncPage（loading / loadError / retrying + 绝不 reject）----

  it('load 失败：收敛为 loadError，且不向上 reject', async () => {
    const table = useAdminTable<Row>({
      fetch: async () => {
        throw new Error('boom')
      }
    })

    await expect(table.load()).resolves.toBeUndefined()
    expect(table.loadError.value).toBe(true)
    expect(table.loading.value).toBe(false)
  })

  it('load 成功：清掉上一次的 loadError', async () => {
    let fail = true
    const table = useAdminTable<Row>({
      fetch: async () => {
        if (fail) throw new Error('boom')
        return { list: [{ id: 1, name: 'a' }], total: 1 }
      }
    })

    await table.load()
    expect(table.loadError.value).toBe(true)

    fail = false
    await table.load()
    expect(table.loadError.value).toBe(false)
    expect(table.list.value).toHaveLength(1)
  })

  it('retry：失败后重试成功，retrying 开合且 loadError 复位', async () => {
    let fail = true
    const table = useAdminTable<Row>({
      fetch: async () => {
        if (fail) throw new Error('boom')
        return { list: [{ id: 2, name: 'b' }], total: 1 }
      }
    })

    await table.load()
    expect(table.loadError.value).toBe(true)

    fail = false
    const p = table.retry()
    expect(table.retrying.value).toBe(true)
    await p
    expect(table.retrying.value).toBe(false)
    expect(table.loadError.value).toBe(false)
    expect(table.list.value[0].id).toBe(2)
  })

  it('retry 防重入：重试进行中再调不重复触发 fetch', async () => {
    let calls = 0
    let release: (() => void) | null = null
    const table = useAdminTable<Row>({
      fetch: () => {
        calls++
        return new Promise<{ list: Row[]; total: number }>((resolve) => {
          release = () => resolve({ list: [], total: 0 })
        })
      }
    })

    const first = table.retry()
    const second = table.retry()
    release!()
    await Promise.all([first, second])

    expect(calls).toBe(1)
  })

  it('search 也清掉 loadError（与 re-load 语义一致）', async () => {
    let fail = true
    const table = useAdminTable<Row>({
      fetch: async () => {
        if (fail) throw new Error('boom')
        return { list: [], total: 0 }
      },
      searchable: true
    })

    await table.load()
    expect(table.loadError.value).toBe(true)

    fail = false
    await table.search()
    expect(table.loadError.value).toBe(false)
  })
})
