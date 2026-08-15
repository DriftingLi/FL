// useVerifyDialog：验证码确认对话框状态机 module 的接口级测试（6 位校验/onSuccess 触发/状态复位）。
// seam：composable 接口——fake adapter（sendCode/submitAsync 用 vi.fn），ElMessage 以 spy 观测。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ElMessage } from 'element-plus'
import { useVerifyDialog } from '@/composables/useVerifyDialog'

vi.mock('element-plus', () => ({
  ElMessage: { warning: vi.fn(), success: vi.fn() }
}))

const warningSpy = vi.mocked(ElMessage.warning)

interface MountOptions {
  submitReject?: boolean
  onSuccess?: () => Promise<void> | void
}

function mountDialog(opts: MountOptions = {}) {
  const sendCode = vi.fn().mockResolvedValue(undefined)
  const submitAsync = opts.submitReject
    ? vi.fn().mockRejectedValue(new Error('server error'))
    : vi.fn().mockResolvedValue(undefined)
  const dialog = useVerifyDialog({
    sendCode,
    submitAsync,
    onSuccess: opts.onSuccess
  })
  return { dialog, sendCode, submitAsync }
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('6 位验证码校验', () => {
  it('空验证码：警告「请输入6位验证码」且不调用提交实现', async () => {
    const { dialog, submitAsync } = mountDialog()
    dialog.visible.value = true
    dialog.target.value = 'user@example.com'
    dialog.code.value = ''
    await dialog.submit()
    expect(warningSpy).toHaveBeenCalledWith('请输入6位验证码')
    expect(submitAsync).not.toHaveBeenCalled()
    expect(dialog.submitting.value).toBe(false)
  })

  it('不足 6 位：警告且不调用提交实现', async () => {
    const { dialog, submitAsync } = mountDialog()
    dialog.visible.value = true
    dialog.target.value = '13800138000'
    dialog.code.value = '12345'
    await dialog.submit()
    expect(warningSpy).toHaveBeenCalledWith('请输入6位验证码')
    expect(submitAsync).not.toHaveBeenCalled()
  })

  it('含非数字：警告且不调用提交实现', async () => {
    const { dialog, submitAsync } = mountDialog()
    dialog.visible.value = true
    dialog.target.value = '13800138000'
    dialog.code.value = '12345a'
    await dialog.submit()
    expect(warningSpy).toHaveBeenCalledWith('请输入6位验证码')
    expect(submitAsync).not.toHaveBeenCalled()
  })

  it('恰好 6 位数字：以 trim 后的 target 与 code 调用提交实现', async () => {
    const { dialog, submitAsync } = mountDialog()
    dialog.visible.value = true
    dialog.target.value = ' user@example.com '
    dialog.code.value = '123456'
    await dialog.submit()
    expect(submitAsync).toHaveBeenCalledTimes(1)
    expect(submitAsync).toHaveBeenCalledWith('user@example.com', '123456')
  })
})

describe('submit 提交', () => {
  it('成功后触发 onSuccess 且关闭对话框', async () => {
    const onSuccess = vi.fn().mockResolvedValue(undefined)
    const { dialog, submitAsync } = mountDialog({ onSuccess })
    dialog.visible.value = true
    dialog.target.value = '13800138000'
    dialog.code.value = '123456'
    await dialog.submit()
    expect(submitAsync).toHaveBeenCalledTimes(1)
    expect(onSuccess).toHaveBeenCalledTimes(1)
    expect(dialog.visible.value).toBe(false)
  })

  it('未注入 onSuccess 时成功提交不报错并关闭对话框', async () => {
    const { dialog } = mountDialog()
    dialog.visible.value = true
    dialog.target.value = '13800138000'
    dialog.code.value = '123456'
    await dialog.submit()
    expect(dialog.visible.value).toBe(false)
  })

  it('提交失败：不触发 onSuccess 且对话框保持打开', async () => {
    const onSuccess = vi.fn()
    const { dialog } = mountDialog({ submitReject: true, onSuccess })
    dialog.visible.value = true
    dialog.target.value = '13800138000'
    dialog.code.value = '123456'
    await dialog.submit()
    expect(onSuccess).not.toHaveBeenCalled()
    expect(dialog.visible.value).toBe(true)
  })
})

describe('sending/submitting 状态复位', () => {
  it('send 过程 sending 置位并在结束后复位、透传 trim 后的 target', async () => {
    const { dialog, sendCode } = mountDialog()
    const p = dialog.send(' 13800138000 ', 'phone')
    expect(dialog.sending.value).toBe(true)
    await p
    expect(dialog.sending.value).toBe(false)
    expect(sendCode).toHaveBeenCalledWith('13800138000', 'phone')
  })

  it('send 失败（拦截器已提示）后 sending 仍复位', async () => {
    const sendCode = vi.fn().mockRejectedValue(new Error('throttled'))
    const dialog = useVerifyDialog({
      sendCode,
      submitAsync: vi.fn()
    })
    await dialog.send('13800138000', 'phone')
    expect(dialog.sending.value).toBe(false)
  })

  it('submit 结束后 submitting 复位', async () => {
    const { dialog } = mountDialog()
    dialog.visible.value = true
    dialog.target.value = '13800138000'
    dialog.code.value = '123456'
    await dialog.submit()
    expect(dialog.submitting.value).toBe(false)
    expect(dialog.sending.value).toBe(false)
  })
})
