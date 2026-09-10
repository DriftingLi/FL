// useRejectReasonDialog：#795 —— 三域审核共有的驳回理由弹窗状态机。
// seam：composable 接口 —— onSubmit 用内存 stub，不触达 API 层；ElMessage 断言提示文案。
import { describe, it, expect, vi, beforeEach } from 'vitest'

const { warningSpy } = vi.hoisted(() => ({ warningSpy: vi.fn() }))
vi.mock('element-plus', () => ({
  ElMessage: { warning: warningSpy, success: vi.fn(), error: vi.fn() }
}))

import { useRejectReasonDialog } from '../useRejectReasonDialog'

describe('useRejectReasonDialog（驳回理由弹窗）', () => {
  beforeEach(() => warningSpy.mockClear())

  it('open 打开弹窗并清空上一次的理由', () => {
    const d = useRejectReasonDialog({ onSubmit: vi.fn() })
    d.reason.value = '上次的理由'
    d.open()
    expect(d.visible.value).toBe(true)
    expect(d.reason.value).toBe('')
  })

  it('提交成功：把理由交给 onSubmit 并关闭弹窗', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    const d = useRejectReasonDialog({ onSubmit })
    d.open()
    d.reason.value = '内容不符'

    await d.submit()

    expect(onSubmit).toHaveBeenCalledWith('内容不符')
    expect(d.visible.value).toBe(false)
    expect(d.submitting.value).toBe(false)
  })

  it('空理由拦截提交并提示（默认 requireReason）', async () => {
    const onSubmit = vi.fn()
    const d = useRejectReasonDialog({ onSubmit })
    d.open()
    d.reason.value = '   '

    await d.submit()

    expect(onSubmit).not.toHaveBeenCalled()
    expect(d.visible.value).toBe(true)
    expect(warningSpy).toHaveBeenCalledWith('请填写驳回理由')
  })

  it('requireReason: false 时允许空理由提交', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    const d = useRejectReasonDialog({ onSubmit, requireReason: false })
    d.open()

    await d.submit()

    expect(onSubmit).toHaveBeenCalledWith('')
  })

  it('提交失败：保留理由、弹窗不关闭、submitting 复位', async () => {
    const onSubmit = vi.fn().mockRejectedValue(new Error('boom'))
    const d = useRejectReasonDialog({ onSubmit })
    d.open()
    d.reason.value = '理由仍在'

    await d.submit()

    expect(d.visible.value).toBe(true)
    expect(d.reason.value).toBe('理由仍在')
    expect(d.submitting.value).toBe(false)
  })

  it('提交中防重入：再调不重复触发 onSubmit', async () => {
    let release: (() => void) | null = null
    const onSubmit = vi.fn(
      () => new Promise<void>((resolve) => {
        release = () => resolve()
      })
    )
    const d = useRejectReasonDialog({ onSubmit })
    d.open()
    d.reason.value = '理由'

    const first = d.submit()
    const second = d.submit()
    expect(d.submitting.value).toBe(true)
    release!()
    await Promise.all([first, second])

    expect(onSubmit).toHaveBeenCalledTimes(1)
  })

  it('提交中禁止关闭弹窗', async () => {
    let release: (() => void) | null = null
    const d = useRejectReasonDialog({
      onSubmit: () =>
        new Promise<void>((resolve) => {
          release = () => resolve()
        })
    })
    d.open()
    d.reason.value = '理由'

    const p = d.submit()
    d.close()
    expect(d.visible.value).toBe(true)

    release!()
    await p
    expect(d.visible.value).toBe(false)
  })

  it('单条与批量走同一状态机（提交动作差异由调用方闭包承载）', async () => {
    const single = vi.fn().mockResolvedValue(undefined)
    const batch = vi.fn().mockResolvedValue(undefined)
    let mode: 'single' | 'batch' = 'single'
    const d = useRejectReasonDialog({
      onSubmit: async (reason) => {
        if (mode === 'single') await single(reason)
        else await batch(reason)
      }
    })

    d.open()
    d.reason.value = 'r1'
    await d.submit()

    mode = 'batch'
    d.open()
    d.reason.value = 'r2'
    await d.submit()

    expect(single).toHaveBeenCalledWith('r1')
    expect(batch).toHaveBeenCalledWith('r2')
  })
})
