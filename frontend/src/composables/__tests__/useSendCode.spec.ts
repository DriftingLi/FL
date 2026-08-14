// useSendCode：发送验证码交互 module 的接口级测试（格式校验/节流/成功文案/失败短路）。
// seam：composable 接口——sendCode adapter 用内存 fixture，ElMessage 以 spy 观测。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ElMessage } from 'element-plus'
import { useSendCode } from '@/composables/useSendCode'
import { isValidEmail, isValidPhone, isValidAccount } from '@/utils/validate'

vi.mock('element-plus', () => ({
  ElMessage: { warning: vi.fn(), success: vi.fn() }
}))

const warningSpy = vi.mocked(ElMessage.warning)
const successSpy = vi.mocked(ElMessage.success)

function mountSendCode(purpose: Parameters<typeof useSendCode>[0]['purpose'] = 'login') {
  const sendCode = vi.fn().mockResolvedValue(undefined)
  const mod = useSendCode({ purpose, sendCode })
  return { mod, sendCode }
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('格式校验（单一实现）', () => {
  it('邮箱校验与后端 net/mail 语义对齐：接受无点域', () => {
    expect(isValidEmail('a@b.c')).toBe(true)
    expect(isValidEmail('user@example.com')).toBe(true)
    expect(isValidEmail('a@b')).toBe(true)
    expect(isValidEmail('a b@c')).toBe(false)
    expect(isValidEmail('abc')).toBe(false)
  })

  it('手机号校验：11 位 1[3-9] 开头', () => {
    expect(isValidPhone('13800138000')).toBe(true)
    expect(isValidPhone('12800138000')).toBe(false)
    expect(isValidPhone('1380013800')).toBe(false)
  })

  it('账号校验与后端 IsValidAccount 对齐：4-20 位字母/数字/下划线', () => {
    expect(isValidAccount('abc1')).toBe(true)
    expect(isValidAccount('a_bc')).toBe(true)
    expect(isValidAccount('abc')).toBe(false)
    expect(isValidAccount('a'.repeat(21))).toBe(false)
    expect(isValidAccount('a-bc')).toBe(false)
  })
})

describe('send（login 目的）', () => {
  it('手机号非法时警告且不调用发送实现', async () => {
    const { mod, sendCode } = mountSendCode('login')
    const ok = await mod.send('1380013800', 'phone')
    expect(ok).toBe(false)
    expect(warningSpy).toHaveBeenCalledWith('请输入正确的手机号')
    expect(sendCode).not.toHaveBeenCalled()
  })

  it('邮箱非法时警告且不调用发送实现', async () => {
    const { mod, sendCode } = mountSendCode('login')
    const ok = await mod.send('abc', 'email')
    expect(ok).toBe(false)
    expect(warningSpy).toHaveBeenCalledWith('请输入正确的邮箱地址')
    expect(sendCode).not.toHaveBeenCalled()
  })

  it('合法目标发送成功后启动 60s 倒计时', async () => {
    const { mod, sendCode } = mountSendCode('login')
    const ok = await mod.send('13800138000', 'phone')
    expect(ok).toBe(true)
    expect(sendCode).toHaveBeenCalledWith('phone', '13800138000')
    expect(successSpy).toHaveBeenCalledWith('验证码已发送，请查收')
    expect(mod.remaining.value).toBe(60)
    expect(mod.sending.value).toBe(false)
  })

  it('发送失败（拦截器已提示）不启动倒计时', async () => {
    const sendCode = vi.fn().mockRejectedValue(new Error('throttled'))
    const mod = useSendCode({ purpose: 'login', sendCode })
    const ok = await mod.send('13800138000', 'phone')
    expect(ok).toBe(false)
    expect(mod.remaining.value).toBe(0)
  })
})

describe('send（account_change 目的）', () => {
  it('跳过目标格式校验（验证码发往已绑定手机号），成功文案特定', async () => {
    const { mod, sendCode } = mountSendCode('account_change')
    const ok = await mod.send('', 'phone')
    expect(ok).toBe(true)
    expect(sendCode).toHaveBeenCalledWith('phone', '')
    expect(successSpy).toHaveBeenCalledWith('验证码已发送至绑定手机号，请查收')
    expect(warningSpy).not.toHaveBeenCalled()
  })
})

describe('send（change_password 目的）', () => {
  it('跳过目标格式校验（验证码发往已绑定手机号）', async () => {
    const { mod, sendCode } = mountSendCode('change_password')
    const ok = await mod.send('', 'phone')
    expect(ok).toBe(true)
    expect(sendCode).toHaveBeenCalledWith('phone', '')
    expect(successSpy).toHaveBeenCalledWith('验证码已发送至绑定手机号，请查收')
    expect(warningSpy).not.toHaveBeenCalled()
  })
})
