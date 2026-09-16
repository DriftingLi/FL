// useAsyncPage：学员端列表页三态 + 分页状态机（#388）+ 证件切换失效刷新（#604）的接口级测试。
// seam：composable 接口——loader 用内存 fake（成功/失败/慢速），不触达 API 层；
// 证件信号源用真实 pinia store（setActivePinia），与生产 watch 口径一致。
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
})
