// URL auth_token 交接工具（authToken.ts）单测
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { extractAuthTokenFromUrl, consumeAuthTokenFromUrl } from '../authToken'

beforeEach(() => {
  vi.restoreAllMocks()
})

describe('extractAuthTokenFromUrl（纯函数）', () => {
  it('提取 token 并移除参数，保留其余查询串', () => {
    expect(extractAuthTokenFromUrl('?auth_token=abc&next=/training')).toEqual({
      token: 'abc',
      remainingQuery: 'next=%2Ftraining'
    })
  })

  it('token 独占时剩余查询为空串', () => {
    expect(extractAuthTokenFromUrl('?auth_token=abc')).toEqual({ token: 'abc', remainingQuery: '' })
  })

  it('无 auth_token 时 no-op（token 为空）', () => {
    expect(extractAuthTokenFromUrl('?foo=1')).toEqual({ token: '', remainingQuery: 'foo=1' })
  })

  it('空查询串返回空 token', () => {
    expect(extractAuthTokenFromUrl('')).toEqual({ token: '', remainingQuery: '' })
  })

  it('空 auth_token 视为缺失', () => {
    expect(extractAuthTokenFromUrl('?auth_token=')).toEqual({ token: '', remainingQuery: 'auth_token=' })
  })
})

describe('consumeAuthTokenFromUrl（window 封装）', () => {
  it('读取后立即从地址栏移除并返回 token', () => {
    window.history.replaceState({}, '', '/app?auth_token=carried&next=/training')
    const replaceSpy = vi.spyOn(window.history, 'replaceState')

    const token = consumeAuthTokenFromUrl()

    expect(token).toBe('carried')
    expect(replaceSpy).toHaveBeenCalledWith(null, '', '/app?next=%2Ftraining')
    expect(window.location.search).toBe('?next=%2Ftraining')
  })

  it('无 auth_token 时不改写地址栏', () => {
    window.history.replaceState({}, '', '/app?foo=1')
    const replaceSpy = vi.spyOn(window.history, 'replaceState')

    expect(consumeAuthTokenFromUrl()).toBe('')

    expect(replaceSpy).not.toHaveBeenCalled()
  })
})
