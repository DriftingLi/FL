// useAsyncPage：学员端列表页三态 + 分页状态机（#388）的接口级测试。
// seam：composable 接口——loader 用内存 fake（成功/失败/慢速），不触达 API 层。
import { describe, it, expect, vi } from 'vitest'
import { ref, nextTick } from 'vue'
import { useAsyncPage } from '../useAsyncPage'

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
