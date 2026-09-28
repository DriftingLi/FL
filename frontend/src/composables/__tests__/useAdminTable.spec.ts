// useAdminTable：管理端列表状态机的接口级测试。
// seam：composable 接口——fetch 用内存 fixture，actions 用内存 stub，不触达 API 层。
import { describe, it, expect, vi } from 'vitest'
import { nextTick, ref, toRaw } from 'vue'

// 删除确认已收口到 useConfirm（#735），mock 掉按「确认/取消」两种路径放行
const { confirmSpy } = vi.hoisted(() => ({ confirmSpy: vi.fn() }))
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: confirmSpy, confirmDanger: confirmSpy, prompt: vi.fn() })
}))

import { toPage, type Page } from '@/api/page'
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
        items: [{ id: paging.page, name: `row-${paging.page}` }],
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
        return { items: [{ id: 1, name: 'a' }], total: 1 }
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
        return { items: [{ id: 2, name: 'b' }], total: 1 }
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
        return new Promise<Page<Row>>((resolve) => {
          release = () => resolve({ items: [], total: 0 })
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
        return { items: [], total: 0 }
      },
      searchable: true
    })

    await table.load()
    expect(table.loadError.value).toBe(true)

    fail = false
    await table.search()
    expect(table.loadError.value).toBe(false)
  })

  // ---- 空态判据与错误分类（第十一波 #1102，ADR-0056 §9）：
  //      isEmpty 与 useAsyncPage.isEmpty 同源（utils/listState.isEmptyList）----

  it('isEmpty：装载成功且没有条目为真，有条目为假', async () => {
    let rows: Row[] = []
    const table = useAdminTable<Row>({ fetch: async () => ({ items: rows, total: rows.length }) })

    await table.load()
    expect(table.isEmpty.value).toBe(true)

    rows = [{ id: 1, name: 'a' }]
    await table.load()
    expect(table.isEmpty.value).toBe(false)
  })

  // ---- 票 6（ADR-0060 决策 6）：容器由 api 侧 Page<T> 承载；兜底的唯一宿主是 toPage ----

  it('后端整段不回负载（经 toPage 归一）：仍按 [] 与 0 装载，口径与归位前逐字一致', async () => {
    const table = useAdminTable<Row>({
      // 真实链路上信封 data 可能为 null；那一层兜底只在 api/page.ts 的 toPage 里做一次，
      // composable 信任 Page<T> 的声明面（再兜一层就是同一判据的第二宿主）。
      fetch: async () => toPage(undefined, undefined)
    })

    await table.load()

    expect(table.list.value).toEqual([])
    expect(table.total.value).toBe(0)
    expect(table.loadError.value).toBe(false)
    expect(table.isEmpty.value).toBe(true)
  })

  it('loadErrorKind：404 归空态（isEmpty 仍为真）、其余错误是错误态', async () => {
    const notFound = useAdminTable<Row>({
      fetch: async () => {
        throw Object.assign(new Error('nf'), { kind: 'notfound' })
      }
    })
    await notFound.load()
    expect(notFound.loadErrorKind.value).toBe('notfound')
    expect(notFound.isEmpty.value).toBe(true)

    const broken = useAdminTable<Row>({
      fetch: async () => {
        throw Object.assign(new Error('boom'), { kind: 'network' })
      }
    })
    await broken.load()
    expect(broken.loadErrorKind.value).toBe('network')
    expect(broken.isEmpty.value).toBe(false)
  })

  it('loadErrorKind：装载成功即清空上一次的错误分类', async () => {
    let fail = true
    const table = useAdminTable<Row>({
      fetch: async () => {
        if (fail) throw Object.assign(new Error('boom'), { kind: 'server' })
        return { items: [{ id: 1, name: 'a' }], total: 1 }
      }
    })

    await table.load()
    expect(table.loadErrorKind.value).toBe('server')

    fail = false
    await table.load()
    expect(table.loadErrorKind.value).toBe(null)
  })
})

/**
 * #1354（ADR-0069 实施回写段登记的「不在本 ADR 面上的同类」）：列表装载的批次代数。
 *
 * 缺陷形状与真实缺陷 #5 逐字同形：`load()` 在 `await options.fetch(...)` **之后**无条件写
 * `list`/`total`，没有「发起时捕获、落地前比对」的判据 ⇒ 两次装载重叠时后到的旧响应盖掉新状态
 * （筛选词条是新的、列表是旧的）。判据宿主在 composable（与 useAsyncPage.generation 同一份机制），
 * 页面零改动；结构锁见 `loadFlowLocks.spec.ts` R5（本件的 list/total 住在 module 内部，
 * R4 那条「页面 loader 不写回」在这里没有可扫的面）。
 */
describe('useAdminTable 批次代数（#1354：旧轮不落地）', () => {
  it('连发两次 applyFilters（第一次慢、第二次快）：list/total 停在第二次的结果上', async () => {
    let releaseSlow!: () => void
    const slow = new Promise<void>(resolve => {
      releaseSlow = resolve
    })
    const slowItems: Row[] = [{ id: 1, name: 'row-a' }]
    const fastItems: Row[] = [{ id: 2, name: 'row-b' }]
    const table = useAdminTable<Row>({
      fetch: async (_paging, filters) => {
        if (filters.status === 'a') {
          await slow
          return { items: slowItems, total: 99 }
        }
        return { items: fastItems, total: 7 }
      }
    })

    const first = table.applyFilters({ status: 'a' })
    await nextTick()
    await table.applyFilters({ status: 'b' })
    // toRaw：list 是深响应 ref，取回 fetch 出口那个数组本体 —— 比的是**引用同一性**，
    // 旧轮只要写回过一次，这里就不是同一个对象了（深相等断言看不出没发生过写回）。
    expect(toRaw(table.list.value)).toBe(fastItems)
    expect(table.total.value).toBe(7)

    // 旧响应此刻才到：整体作废，连写回都不执行（断言数组**引用**没换过，
    // 不是「先写成 row-a 再被改回 row-b」——那一帧的可看见状态正是本缺陷的形态）
    releaseSlow()
    await first
    expect(toRaw(table.list.value)).toBe(fastItems)
    expect(table.list.value[0].name).toBe('row-b')
    expect(table.total.value).toBe(7)
  })

  it('旧轮的失败后到：不把新一轮打成错误态', async () => {
    let rejectSlow!: (e: unknown) => void
    const slow = new Promise<never>((_resolve, reject) => {
      rejectSlow = reject
    })
    const table = useAdminTable<Row>({
      fetch: async (_paging, filters) => {
        if (filters.status === 'a') return slow
        return { items: [{ id: 2, name: 'row-b' }], total: 7 }
      }
    })

    const first = table.applyFilters({ status: 'a' })
    await nextTick()
    await table.applyFilters({ status: 'b' })
    expect(table.loadError.value).toBe(false)

    rejectSlow(new Error('boom'))
    await first
    // 上一轮的失败不属于这一轮：列表与错误态都停在新一轮的成功结果上
    expect(table.loadError.value).toBe(false)
    expect(table.loadErrorKind.value).toBe(null)
    expect(table.list.value[0].name).toBe('row-b')
  })

  it('旧轮先落地、新轮仍在飞：loading 不由旧轮归零', async () => {
    let releaseOld!: () => void
    const old = new Promise<void>(resolve => {
      releaseOld = resolve
    })
    let releaseNew!: () => void
    const fresh = new Promise<void>(resolve => {
      releaseNew = resolve
    })
    let calls = 0
    const table = useAdminTable<Row>({
      fetch: async () => {
        calls++
        if (calls === 1) return old.then(() => ({ items: [{ id: 1, name: 'old' }], total: 1 }))
        return fresh.then(() => ({ items: [{ id: 2, name: 'new' }], total: 2 }))
      }
    })

    const first = table.load()
    await nextTick()
    const second = table.load()
    expect(table.loading.value).toBe(true)

    // 旧轮此刻落地：它没有权利熄灭新轮的 spinner（旧形状的 finally 会无条件归零）
    releaseOld()
    await first
    expect(table.loading.value).toBe(true)
    expect(table.list.value).toEqual([])

    releaseNew()
    await second
    expect(table.loading.value).toBe(false)
    expect(table.list.value[0].name).toBe('new')
    expect(table.total.value).toBe(2)
  })

  it('判据本身有效：没有代数守卫（本票修复前的形状）时，旧轮照样覆盖新筛选', async () => {
    // 复刻修复前的 load()：await 之后直接写回 —— 证明上面那组用例测的是「守卫在写回之前」，
    // 而不是「这个竞态本来就不存在」（口径同 useAsyncPage.spec 的反例自检）。
    const list = ref<Row[]>([])
    const total = ref(0)
    let releaseSlow!: () => void
    const slow = new Promise<void>(resolve => {
      releaseSlow = resolve
    })
    const fetch = async (filters: Record<string, unknown>) => {
      if (filters.status === 'a') {
        await slow
        return { items: [{ id: 1, name: 'row-a' }], total: 99 }
      }
      return { items: [{ id: 2, name: 'row-b' }], total: 7 }
    }
    const loadOld = async (filters: Record<string, unknown>) => {
      const result = await fetch(filters)
      list.value = result.items
      total.value = result.total
    }

    const first = loadOld({ status: 'a' })
    await loadOld({ status: 'b' })
    expect(list.value[0].name).toBe('row-b')

    releaseSlow()
    await first
    expect(list.value[0].name).toBe('row-a')
    expect(total.value).toBe(99)
  })
})
