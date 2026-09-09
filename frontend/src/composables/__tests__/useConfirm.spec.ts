import { describe, it, expect, vi, beforeEach } from 'vitest'

// useConfirm 只依赖 ElMessageBox 的命令式 API，mock 掉避免弹真框。
// vi.mock 会被提升到文件顶，factory 里不能引用未初始化的顶部变量，故用 vi.hoisted。
const { confirmSpy, promptSpy } = vi.hoisted(() => ({
  confirmSpy: vi.fn(),
  promptSpy: vi.fn()
}))
vi.mock('element-plus', () => ({
  ElMessageBox: { confirm: confirmSpy, prompt: promptSpy }
}))

import { useConfirm } from '../useConfirm'

describe('useConfirm', () => {
  beforeEach(() => {
    confirmSpy.mockClear()
    promptSpy.mockClear()
  })

  it('confirm 注入统一按钮文案与 warning 类型', () => {
    const { confirm } = useConfirm()
    confirm('退出登录吗？', '退出登录')
    expect(confirmSpy).toHaveBeenCalledWith('退出登录吗？', '退出登录', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
  })

  it('confirm 的 title 缺省时兜底「提示」', () => {
    const { confirm } = useConfirm()
    confirm('内容')
    expect(confirmSpy.mock.calls[0][1]).toBe('提示')
  })

  it('confirmDanger 注入红色确认钮、默认「删除」文案，且 autofocus 关闭', () => {
    const { confirmDanger } = useConfirm()
    confirmDanger('删除该收藏吗？', '移除收藏')
    const opts = confirmSpy.mock.calls[0][2]
    expect(opts.confirmButtonClass).toBe('el-button--danger')
    expect(opts.confirmButtonText).toBe('删除')
    expect(opts.autofocus).toBe(false)
    expect(opts.type).toBe('warning')
    expect(opts.cancelButtonText).toBe('取消')
  })

  it('调用方 options 覆盖默认值（如把危险确认钮文案改成「清空」）', () => {
    const { confirmDanger } = useConfirm()
    confirmDanger('清空所有草稿？', '清空', { confirmButtonText: '清空' })
    const opts = confirmSpy.mock.calls[0][2]
    expect(opts.confirmButtonText).toBe('清空')
  })

  it('prompt 走 ElMessageBox.prompt 并注入统一按钮文案', () => {
    const { prompt } = useConfirm()
    prompt('驳回原因', '驳回投稿', { inputPattern: /.+/ })
    expect(promptSpy).toHaveBeenCalledWith('驳回原因', '驳回投稿', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      inputPattern: /.+/
    })
  })
})
