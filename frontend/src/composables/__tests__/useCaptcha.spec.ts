// useCaptcha：图形验证码交互 module 测试。
// seam：composable 接口 + authApi mock（拦截器错误提示语义由 catch 表达）。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useCaptcha } from '@/composables/useCaptcha'
import { authApi } from '@/api/auth'

vi.mock('@/api/auth', () => ({
  authApi: { getCaptcha: vi.fn() }
}))

const getCaptcha = vi.mocked(authApi.getCaptcha)

beforeEach(() => {
  vi.clearAllMocks()
})

describe('useCaptcha 加载/刷新', () => {
  it('加载成功：写入 id 与图片、清空已输入值、loading 复位', async () => {
    getCaptcha.mockResolvedValue({ id: 'cap-1', image: 'data:image/png;base64,AAA' })
    const { captchaId, captchaImage, captchaValue, captchaLoading, refreshCaptcha } = useCaptcha()
    captchaValue.value = '1234'

    const p = refreshCaptcha()
    expect(captchaLoading.value).toBe(true)
    await p

    expect(captchaId.value).toBe('cap-1')
    expect(captchaImage.value).toBe('data:image/png;base64,AAA')
    expect(captchaValue.value).toBe('')
    expect(captchaLoading.value).toBe(false)
  })

  it('刷新成功后再次输入值，再次刷新会清空', async () => {
    getCaptcha.mockResolvedValue({ id: 'cap-2', image: 'data:image/png;base64,BBB' })
    const { captchaValue, refreshCaptcha } = useCaptcha()
    await refreshCaptcha()
    captchaValue.value = '99'
    await refreshCaptcha()
    expect(captchaValue.value).toBe('')
  })

  it('加载失败：静默不抛（拦截器已提示）、loading 复位、状态保持为空', async () => {
    getCaptcha.mockRejectedValue(new Error('network'))
    const { captchaId, captchaImage, captchaLoading, refreshCaptcha } = useCaptcha()

    await expect(refreshCaptcha()).resolves.toBeUndefined()
    expect(captchaLoading.value).toBe(false)
    expect(captchaId.value).toBe('')
    expect(captchaImage.value).toBe('')
  })
})
