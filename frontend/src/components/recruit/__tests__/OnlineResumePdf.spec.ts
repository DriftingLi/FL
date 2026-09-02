// #485 在线简历 PDF 内嵌组件：带鉴权 blob 取流、加载骨架、失败可重试。
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus from 'element-plus'

vi.mock('@/api/client', () => ({
  getValidAccessToken: vi.fn()
}))

import { getValidAccessToken } from '@/api/client'
import OnlineResumePdf from '../OnlineResumePdf.vue'

function mountComp(endpoint = '/api/recruit/resumes/1/pdf') {
  return mount(OnlineResumePdf, {
    props: { endpoint },
    global: { plugins: [ElementPlus] }
  })
}

beforeEach(() => {
  vi.mocked(getValidAccessToken).mockReset()
  vi.mocked(getValidAccessToken).mockResolvedValue('test-token')
  vi.stubGlobal('fetch', vi.fn())
  vi.stubGlobal('URL', { ...URL, createObjectURL: vi.fn(() => 'blob:mock'), revokeObjectURL: vi.fn() })
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('OnlineResumePdf（#485）', () => {
  it('带鉴权头取流并内嵌 blob URL', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      blob: () => Promise.resolve(new Blob(['%PDF-test'], { type: 'application/pdf' }))
    })
    vi.stubGlobal('fetch', fetchMock)
    const wrapper = mountComp()
    await flushPromises()
    expect(getValidAccessToken).toHaveBeenCalled()
    expect(fetchMock).toHaveBeenCalledWith('/api/recruit/resumes/1/pdf', {
      headers: { Authorization: 'Bearer test-token' }
    })
    const iframe = wrapper.find('iframe')
    expect(iframe.exists()).toBe(true)
    expect(iframe.attributes('src')).toBe('blob:mock')
  })

  it('加载中显示骨架、成功后消失', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      blob: () => Promise.resolve(new Blob(['%PDF'], { type: 'application/pdf' }))
    }))
    const wrapper = mountComp()
    expect(wrapper.text()).toContain('简历加载中')
    await flushPromises()
    expect(wrapper.text()).not.toContain('简历加载中')
  })

  it('失败显示可重试错误态，点重试后成功', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ ok: false, status: 500 })
      .mockResolvedValueOnce({ ok: true, blob: () => Promise.resolve(new Blob(['%PDF'])) })
    vi.stubGlobal('fetch', fetchMock)
    const wrapper = mountComp()
    await flushPromises()
    expect(wrapper.text()).toContain('简历加载失败')
    const retryBtn = wrapper.findAll('button').find((b) => b.text().includes('重试'))!
    await retryBtn.trigger('click')
    await flushPromises()
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(wrapper.find('iframe').exists()).toBe(true)
  })

  it('端点变化时重新取流', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      blob: () => Promise.resolve(new Blob(['%PDF'], { type: 'application/pdf' }))
    })
    vi.stubGlobal('fetch', fetchMock)
    const wrapper = mountComp('/api/recruit/resumes/1/pdf')
    await flushPromises()
    await wrapper.setProps({ endpoint: '/api/recruit/resumes/2/pdf' })
    await flushPromises()
    expect(fetchMock).toHaveBeenCalledWith('/api/recruit/resumes/2/pdf', expect.anything())
  })
})
