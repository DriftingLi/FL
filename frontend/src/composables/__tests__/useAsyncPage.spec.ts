// useAsyncPage：学员端列表页三态 + 分页状态机（#388）+ 证件切换失效刷新（#604）的接口级测试。
// seam：composable 接口——loader 用内存 fake（成功/失败/慢速），不触达 API 层；
// 证件信号源用真实 pinia store（setActivePinia），与生产 watch 口径一致。
// append 档（#1101 收编、ADR-0060 §4 换判据）：hasMore 只认响应里的分页信封，
// 本文件按「服务端给的页数」造 fixture（含论坛置顶导致的首批短一格形态）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref, nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { useAsyncPage } from '../useAsyncPage'
import { useCredentialStore } from '@/stores/credential'
import type { CredentialDict } from '@/api/credential'

function credentialOf(id: number): CredentialDict {
  return { id, code: `C${id}`, name: `证件${id}`, description: '', category: 'special_operation', level: null, sort_order: 0, status: 1, created_at: '', updated_at: '' }
}

describe('useAsyncPage（三态 + 分页收编）', () => {
  it('run：成功路径 loading 开合、数据由 loader 写入', async () => {
    const data: number[] = []
    const { loading, loadError, run, total } = useAsyncPage(async () => {
      data.push(1)
      total.value = 42
    })

    expect(loading.value).toBe(false)
    const p = run()
    expect(loading.value).toBe(true)
    await p
    expect(loading.value).toBe(false)
    expect(loadError.value).toBe(false)
    expect(data).toEqual([1])
    expect(total.value).toBe(42)
  })

  it('run：loader 抛错收敛为 loadError，且不向上 reject', async () => {
    const { loading, loadError, run } = useAsyncPage(async () => {
      throw new Error('boom')
    })

    await expect(run()).resolves.toBeUndefined()
    expect(loadError.value).toBe(true)
    expect(loading.value).toBe(false)
  })

  it('retry：重试成功后 loadError 复位；重试中防重入', async () => {
    let release!: () => void
    const gate = new Promise<void>(resolve => { release = resolve })
    let shouldFail = true
    const { loadError, retrying, retry } = useAsyncPage(async () => {
      if (shouldFail) throw new Error('fail')
      // 成功路径挂起在 gate 上：用于观察重试进行中的防重入
      await gate
    })

    await retry()
    expect(loadError.value).toBe(true)

    shouldFail = false
    const first = retry()
    expect(retrying.value).toBe(true)
    // 重试仍在飞行中（卡在 gate），再次调用被防重入拦截
    await retry()
    expect(retrying.value).toBe(true)
    release()
    await first
    expect(loadError.value).toBe(false)
    expect(retrying.value).toBe(false)
  })

  it('分页三件套：翻页重装；改页大小回第一页', async () => {
    const seenPages: number[] = []
    const { page, pageSize, total, handlePageChange, handleSizeChange } = useAsyncPage(
      async () => {
        seenPages.push(page.value)
        total.value = 100
      },
      { defaultPageSize: 10 }
    )

    expect(page.value).toBe(1)
    expect(pageSize.value).toBe(10)

    page.value = 3
    handlePageChange()
    await nextTick()
    expect(seenPages).toEqual([3])

    handleSizeChange()
    expect(page.value).toBe(1)
    await vi.waitFor(() => expect(seenPages).toEqual([3, 1]))
  })

  it('pageRef 注入：外部持有页码（论坛按类别分片场景）', async () => {
    const externalPage = ref(2)
    const { page } = useAsyncPage(async () => {}, { pageRef: externalPage })

    expect(page).toBe(externalPage)
    page.value = 5
    expect(externalPage.value).toBe(5)
  })
})

describe('useAsyncPage 证件切换失效刷新（#604 内聚）', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('当前证件 id 变化触发重新装载：null→id、id→id、id→null 各算一次变化', async () => {
    const store = useCredentialStore()
    const load = vi.fn()
    useAsyncPage(load)

    store.current = credentialOf(1)
    await nextTick()
    expect(load).toHaveBeenCalledTimes(1)

    store.current = credentialOf(2)
    await nextTick()
    expect(load).toHaveBeenCalledTimes(2)

    store.current = null
    await nextTick()
    expect(load).toHaveBeenCalledTimes(3)
  })

  it('id 不变（重设同名证件对象）不重复触发', async () => {
    const store = useCredentialStore()
    store.current = credentialOf(1)
    const load = vi.fn()
    useAsyncPage(load)

    store.current = credentialOf(1)
    await nextTick()
    expect(load).not.toHaveBeenCalled()
  })

  it('切换后回第一页：翻到第 3 页再切证件，loader 读到 page=1', async () => {
    const store = useCredentialStore()
    const seenPages: number[] = []
    const { page, run } = useAsyncPage(async () => {
      seenPages.push(page.value)
    })

    await run()
    page.value = 3
    expect(seenPages).toEqual([1])

    store.current = credentialOf(9)
    await nextTick()
    expect(seenPages).toEqual([1, 1])
    expect(page.value).toBe(1)
  })

  it('切换后回第一页对外部 pageRef 同样生效（页码 ref 注入场景）', async () => {
    const store = useCredentialStore()
    const externalPage = ref(4)
    const seenPages: number[] = []
    const { run } = useAsyncPage(
      async () => {
        seenPages.push(externalPage.value)
      },
      { pageRef: externalPage }
    )

    await run()
    store.current = credentialOf(3)
    await nextTick()
    expect(seenPages).toEqual([4, 1])
    expect(externalPage.value).toBe(1)
  })

  it('筛选词/关键词保留：只重置页码，loader 闭包读到原筛选值', async () => {
    const store = useCredentialStore()
    const keyword = ref('液压')
    const seen: Array<{ keyword: string; page: number }> = []
    const { page, run } = useAsyncPage(async () => {
      seen.push({ keyword: keyword.value, page: page.value })
    })

    await run()
    page.value = 4
    store.current = credentialOf(7)
    await nextTick()

    // 关键词原样保留，页码归一为第一页
    expect(seen).toEqual([
      { keyword: '液压', page: 1 },
      { keyword: '液压', page: 1 }
    ])
    expect(keyword.value).toBe('液压')
  })

  it('单次切换恰好一次装载（无双触发）', async () => {
    const store = useCredentialStore()
    const load = vi.fn()
    useAsyncPage(load)

    store.current = credentialOf(5)
    await nextTick()
    await Promise.resolve()
    await nextTick()
    expect(load).toHaveBeenCalledTimes(1)
  })

  it('opt-out：credentialScoped=false 不随证件切换重装', async () => {
    const store = useCredentialStore()
    store.current = credentialOf(1)
    const { page, run } = useAsyncPage(async () => {}, { credentialScoped: false })

    await run()
    page.value = 3
    store.current = credentialOf(2)
    await nextTick()
    expect(page.value).toBe(3)
  })

  it('Pinia 未激活时静默跳过：不建 watch 也不抛错', () => {
    // 模拟无 store 依赖的页面单测直挂场景：解除激活 pinia
    setActivePinia(undefined)
    expect(() => useAsyncPage(async () => {})).not.toThrow()
  })
})

describe('useAsyncPage 筛选轴与空态判据（#1054）', () => {
  it('filterDeps：任一轴变化 → 回第一页重装一次', async () => {
    const keyword = ref('')
    const specialty = ref(0)
    let seenPage = 0
    const loads: Array<[number, string, number]> = []
    const { page, run, handleSizeChange } = useAsyncPage(
      async () => {
        seenPage = page.value
        loads.push([page.value, keyword.value, specialty.value])
      },
      { filterDeps: [keyword, specialty] }
    )

    await run()
    page.value = 4
    await run()
    expect(seenPage).toBe(4)

    // 改一条筛选轴：回第一页并重装
    keyword.value = '叉车'
    await nextTick()
    await Promise.resolve()
    expect(seenPage).toBe(1)
    // 翻页/改页大小的既有行为不受影响
    handleSizeChange()
    await run()
    expect(loads.at(-1)).toEqual([1, '叉车', 0])
  })

  it('filterDeps：同一同步块内多轴齐变只触发一次重装', async () => {
    const keyword = ref('')
    const specialty = ref(0)
    const load = vi.fn()
    useAsyncPage(load, { filterDeps: [keyword, specialty] })

    await nextTick()
    load.mockClear()
    keyword.value = 'a'
    specialty.value = 7
    await nextTick()
    await Promise.resolve()
    expect(load).toHaveBeenCalledTimes(1)
  })

  it('filterDeps：轴变化与证件切换是两条独立 watch，各触发一次', async () => {
    // 上一个 describe 的末例把 pinia 解除激活了：本用例需要证件信号源，先重建
    setActivePinia(createPinia())
    const store = useCredentialStore()
    store.current = credentialOf(1)
    const keyword = ref('')
    const load = vi.fn()
    useAsyncPage(load, { filterDeps: [keyword] })

    await nextTick()
    load.mockClear()
    keyword.value = 'x'
    store.current = credentialOf(2)
    await nextTick()
    await Promise.resolve()
    await nextTick()
    // 两条 watch 不合并去重：各跑一次（与既有「证件切换即重装」行为一致）
    expect(load).toHaveBeenCalledTimes(2)
  })

  it('isEmpty：装载中与错误态恒为 false；装载成功且条目为空才为 true', async () => {
    const items = ref<number[] | null>(null)
    let shouldFail = false
    let writeEmpty = true
    const { isEmpty, run } = useAsyncPage(
      async () => {
        if (shouldFail) throw new Error('boom')
        items.value = writeEmpty ? [] : [1, 2]
      },
      { itemsRef: items }
    )

    // 装载成功且无条目 → 空
    await run()
    expect(isEmpty.value).toBe(true)

    // 错误态：false（即使上一轮条目是空的）
    shouldFail = true
    await run()
    expect(isEmpty.value).toBe(false)

    // 装载中：false；装载成功且有条目 → false
    shouldFail = false
    writeEmpty = false
    const p = run()
    expect(isEmpty.value).toBe(false)
    await p
    expect(isEmpty.value).toBe(false)
  })

  it('isEmpty：未提供 itemsRef 时恒 false（不替调用方猜空）', async () => {
    const { isEmpty, run } = useAsyncPage(async () => {})
    await run()
    expect(isEmpty.value).toBe(false)
  })

  it('loadErrorKind：404 = 空态（isEmpty 为真、loadError 亦为真），其余错误只是错误态', async () => {
    const items = ref<number[]>([])
    let kind: string | null = 'notfound'
    const { isEmpty, loadError, loadErrorKind, run } = useAsyncPage(
      async () => {
        const e = new Error('boom') as Error & { kind?: string }
        if (kind) e.kind = kind
        throw e
      },
      { itemsRef: items }
    )

    // 404（资源不存在）：空态与错误态同真——UiAsyncSection 里空态优先，页面不必自建 404 分支
    await run()
    expect(loadErrorKind.value).toBe('notfound')
    expect(isEmpty.value).toBe(true)

    // 其余错误（服务端 / 网络 / 权限）：只有错误态
    kind = 'server'
    await run()
    expect(loadError.value).toBe(true)
    expect(loadErrorKind.value).toBe('server')
    expect(isEmpty.value).toBe(false)

    // 无 kind（未分类错误）同样不进空态
    kind = null
    await run()
    expect(loadErrorKind.value).toBe(null)
    expect(isEmpty.value).toBe(false)
  })

  it('loadErrorKind：成功装载后清空（上一轮的错误不残留）', async () => {
    let fail = true
    const { loadErrorKind, run } = useAsyncPage(async () => {
      if (fail) {
        const e = new Error('boom') as Error & { kind?: string }
        e.kind = 'server'
        throw e
      }
    })

    await run()
    expect(loadErrorKind.value).toBe('server')
    fail = false
    await run()
    expect(loadErrorKind.value).toBe(null)
  })
})

describe('useAsyncPage append 形态（#1101 唯一入口；ADR-0060 §4 判据换服务端事实）', () => {
  const BATCH = 20

  /** 造一批：`n` 条条目 + 服务端分页信封（page/pages/total 三者同源，照后端下发形态）。 */
  function batch(page: number, pages: number, total: number, n: number) {
    return {
      items: Array.from({ length: n }, (_, i) => ({ id: page * 100 + i })),
      page,
      pages,
      total
    }
  }

  it('run 装载第 1 批；loadMore 追加下一批并推进页码；末页按服务端页数到底', async () => {
    const items = ref<Array<{ id: number }>>([])
    const seenPages: number[] = []
    const { run, loadMore, hasMore, loadingMore, page, total } = useAsyncPage(
      async (p?: number) => {
        seenPages.push(p ?? 1)
        return p === 2 ? batch(2, 2, 25, 5) : batch(1, 2, 25, BATCH)
      },
      { mode: 'append', batchSize: BATCH, itemsRef: items }
    )

    await run()
    expect(seenPages).toEqual([1])
    expect(items.value).toHaveLength(BATCH)
    expect(hasMore.value).toBe(true)
    // total 由响应写入（页面的「剩余 N 条」直接读它，不再自己数累积）
    expect(total.value).toBe(25)

    await loadMore()
    expect(seenPages).toEqual([1, 2])
    expect(items.value).toHaveLength(BATCH + 5)
    // 第二批到底：判据是「第 2 批 = 总页数 2」，与这批几条无关
    expect(hasMore.value).toBe(false)
    expect(page.value).toBe(2)
    expect(loadingMore.value).toBe(false)
  })

  it('首批短一格仍可翻页（ADR-0060 锁的用例，照论坛置顶的真实形态）', async () => {
    // 论坛详情把被采纳回复钉在首页第一条并给它在首页留一格（后端 forum_service.go），
    // 置顶条没落位时首页就是 19/20 —— 旧判据「本批满一批」在此刻判 false，
    // 「加载更多」直接消失、长帖再也翻不到底。服务端说有三页就必须还能翻。
    const items = ref<Array<{ id: number }>>([])
    const seenPages: number[] = []
    const { run, loadMore, hasMore } = useAsyncPage(
      async (p?: number) => {
        seenPages.push(p ?? 1)
        return p === 1 ? batch(1, 3, 55, 19) : batch(2, 3, 55, BATCH)
      },
      { mode: 'append', batchSize: BATCH, itemsRef: items }
    )

    await run()
    expect(items.value).toHaveLength(19)
    expect(hasMore.value).toBe(true)

    await loadMore()
    expect(seenPages).toEqual([1, 2])
    expect(items.value).toHaveLength(39)
    expect(hasMore.value).toBe(true)
  })

  it('末页恰好满批：服务端说没有下一页，就不给一个点下去是空批的入口', async () => {
    // 旧判据的反向错处：total 恰为批大小整数倍时末页也满一批 ⇒ 按钮赖着不走，
    // 点下去发一个返回空数组的请求（论坛的 offset 折算让这一下特别容易撞上）。
    const items = ref<Array<{ id: number }>>([])
    let calls = 0
    const { run, loadMore, hasMore } = useAsyncPage(
      async (p?: number) => {
        calls++
        return p === 2 ? batch(2, 2, 40, BATCH) : batch(1, 2, 40, BATCH)
      },
      { mode: 'append', batchSize: BATCH, itemsRef: items }
    )

    await run()
    await loadMore()
    expect(items.value).toHaveLength(40)
    expect(hasMore.value).toBe(false)
    await loadMore()
    // 到底后不再发请求
    expect(calls).toBe(2)
  })

  it('响应只给 total（职位广场/简历库形态）：按批大小换算总页数，同样不问批满不满', async () => {
    const items = ref<Array<{ id: number }>>([])
    const { run, loadMore, hasMore } = useAsyncPage(
      async (p?: number) => {
        const n = p === 2 ? 3 : BATCH
        return { items: Array.from({ length: n }, (_, i) => ({ id: (p ?? 1) * 100 + i })), total: 23 }
      },
      { mode: 'append', batchSize: BATCH, itemsRef: items }
    )

    await run()
    // ceil(23 / 20) = 2 页 ⇒ 第 1 批之后还有
    expect(hasMore.value).toBe(true)
    await loadMore()
    expect(hasMore.value).toBe(false)
    expect(items.value).toHaveLength(23)
  })

  it('分页信封整体缺失＝契约违规：记 error 并判到底，绝不退回「本批满一批」的旧猜测', async () => {
    const items = ref<Array<{ id: number }>>([])
    const error = vi.spyOn(console, 'error').mockImplementation(() => {})
    let calls = 0
    const { run, loadMore, hasMore } = useAsyncPage(
      async (p?: number) => {
        calls++
        // 满一批（旧判据会在这里说「还有下一批」），但响应既无 pages 也无 total
        return { items: Array.from({ length: BATCH }, (_, i) => ({ id: (p ?? 1) * 100 + i })) }
      },
      { mode: 'append', batchSize: BATCH, itemsRef: items }
    )

    await run()
    expect(hasMore.value).toBe(false)
    await loadMore()
    expect(calls).toBe(1)
    expect(error).toHaveBeenCalledTimes(1)
    expect(error.mock.calls[0][0]).toContain('既无 pages 也无 total')
    error.mockRestore()
  })

  it('服务端说只有一页时直接到底；到底后 loadMore 不发请求', async () => {
    const items = ref<Array<{ id: number }>>([])
    let calls = 0
    const { run, loadMore, hasMore } = useAsyncPage(
      async () => {
        calls++
        return batch(1, 1, 1, 1)
      },
      { mode: 'append', batchSize: BATCH, itemsRef: items }
    )

    await run()
    expect(hasMore.value).toBe(false)
    await loadMore()
    expect(calls).toBe(1)
  })

  it('reset：清空累积 + 回第 1 批（筛选变化走 filterDeps，语义等价）', async () => {
    const items = ref<Array<{ id: number }>>([])
    const keyword = ref('')
    const seen: Array<[number, string]> = []
    const { run, reset, page } = useAsyncPage(
      async (p?: number) => {
        seen.push([p ?? 1, keyword.value])
        return batch(p ?? 1, 3, 60, BATCH)
      },
      { mode: 'append', batchSize: BATCH, itemsRef: items, filterDeps: [keyword] }
    )

    await run()
    page.value = 3
    await run()
    expect(seen.at(-1)).toEqual([3, ''])

    // 筛选轴变化：累积清空 + 回第 1 批重装（重装后只剩新一批，旧批次不残留）
    keyword.value = '叉车'
    await nextTick()
    await Promise.resolve()
    expect(seen.at(-1)).toEqual([1, '叉车'])
    expect(items.value).toHaveLength(BATCH)

    // 显式 reset（「刷新」按钮）同样是「清空 + 回第 1 批」，且 await 得到新批次落地
    await reset()
    expect(seen.at(-1)).toEqual([1, '叉车'])
    expect(items.value).toHaveLength(BATCH)
    expect(page.value).toBe(1)
  })

  it('本批失败即停：不覆盖已累积条目、页码不推进、入口不消失；再点一次即重试同一批', async () => {
    const items = ref<Array<{ id: number }>>([])
    let failSecond = true
    const pages: number[] = []
    const { run, loadMore, hasMore, page, loadError } = useAsyncPage(
      async (p?: number) => {
        pages.push(p ?? 1)
        if ((p ?? 1) === 2 && failSecond) throw new Error('boom')
        return p === 1 ? batch(1, 2, 23, BATCH) : batch(2, 2, 23, 3)
      },
      { mode: 'append', batchSize: BATCH, itemsRef: items }
    )

    await run()
    await loadMore()
    // 已累积的第 1 批原地保持；整页不因翻页失败被换成错误态（loadError 仍是 false）
    expect(items.value).toHaveLength(BATCH)
    expect(loadError.value).toBe(false)
    expect(page.value).toBe(1)
    expect(pages).toEqual([1, 2])
    // 失败不改动本批判据：入口留在原位（藏掉按钮就等于剥夺了那次重试）
    expect(hasMore.value).toBe(true)

    // 再点一次：重跑第 2 批（页码未推进），成功后继续累积并按服务端页数到底
    failSecond = false
    await loadMore()
    expect(pages).toEqual([1, 2, 2])
    expect(items.value).toHaveLength(BATCH + 3)
    expect(page.value).toBe(2)
    expect(hasMore.value).toBe(false)
  })

  it('loadMore 飞行中 hasMore 不翻转（按钮不被换成「没有更多」），loadingMore 防重入', async () => {
    const items = ref<Array<{ id: number }>>([])
    let release!: (res: unknown) => void
    const gate = new Promise<unknown>(resolve => { release = resolve })
    let calls = 0
    const { run, loadMore, hasMore, loadingMore } = useAsyncPage(
      async (p?: number) => {
        calls++
        if ((p ?? 1) === 1) return batch(1, 3, 60, BATCH)
        return gate
      },
      { mode: 'append', batchSize: BATCH, itemsRef: items }
    )

    await run()
    expect(hasMore.value).toBe(true)
    const flight = loadMore()
    expect(loadingMore.value).toBe(true)
    // 在飞行中：入口不消失（页面据此渲染按钮的 loading 态），重复调用被防重入拦下
    expect(hasMore.value).toBe(true)
    await loadMore()
    expect(calls).toBe(2)
    release(batch(2, 3, 60, BATCH))
    await flight
    expect(loadingMore.value).toBe(false)
    expect(items.value).toHaveLength(BATCH * 2)
  })
})
