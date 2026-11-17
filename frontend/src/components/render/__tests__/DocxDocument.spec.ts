// docx 读者面渲染（2026-11 内容精选导入 Word）。
//
// 守三件「少一件就静默出空白」的事：
//   1. 用 fetch 取字节再交给 renderAsync（不能只传 URL）；
//   2. 样式容器与正文容器分开传（docx-preview 往第二个容器塞 <style>）；
//   3. 换文件/卸载时先清容器，避免旧文档节点残留。
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'

const renderAsync = vi.fn()
vi.mock('docx-preview', () => ({ renderAsync: (...args: unknown[]) => renderAsync(...args) }))

import DocxDocument from '../DocxDocument.vue'

const DOCX_URL = '/static/uploads/featured/docx/plan_1700000000000.docx'

let fetchMock: ReturnType<typeof vi.fn>

beforeEach(() => {
  renderAsync.mockReset()
  renderAsync.mockResolvedValue(undefined)
  fetchMock = vi.fn().mockResolvedValue({ ok: true, blob: () => Promise.resolve(new Blob(['PK']) ) })
  vi.stubGlobal('fetch', fetchMock)
})

afterEach(() => {
  vi.unstubAllGlobals()
})

function mountDoc(src: string) {
  return mount(DocxDocument, { props: { src }, global: { plugins: [epLite()] } })
}

describe('DocxDocument', () => {
  it('先 fetch 取字节，再把 blob 与两个容器一起交给 renderAsync', async () => {
    const wrapper = mountDoc(DOCX_URL)
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(DOCX_URL)
    expect(renderAsync).toHaveBeenCalledTimes(1)
    const [blobArg, bodyArg, styleArg, options] = renderAsync.mock.calls[0]
    expect(blobArg).toBeInstanceOf(Blob)
    // 第二个/第三个容器必须是两个不同的 DOM 节点：样式单独挂一处
    expect(bodyArg).toBe(wrapper.find('.docx-document__body').element)
    expect(styleArg).toBe(wrapper.find('.docx-document__style').element)
    expect(bodyArg).not.toBe(styleArg)
    expect(options).toMatchObject({ inWrapper: true, breakPages: false })
    wrapper.unmount()
  })

  it('src 为空时不发请求、不渲染', async () => {
    const wrapper = mountDoc('')
    await flushPromises()
    expect(fetchMock).not.toHaveBeenCalled()
    expect(renderAsync).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('下载失败时给失败态，不抛到页面外', async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 404, blob: () => Promise.resolve(new Blob()) })
    const wrapper = mountDoc(DOCX_URL)
    await flushPromises()
    expect(renderAsync).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('文档加载失败')
    wrapper.unmount()
  })

  it('渲染抛错时同样落到失败态', async () => {
    renderAsync.mockRejectedValue(new Error('bad xml'))
    const wrapper = mountDoc(DOCX_URL)
    await flushPromises()
    expect(wrapper.text()).toContain('文档加载失败')
    wrapper.unmount()
  })

  it('换文件时先清容器再重渲染（不残留旧文档节点）', async () => {
    const wrapper = mountDoc(DOCX_URL)
    await flushPromises()
    const body = wrapper.find('.docx-document__body').element as HTMLElement
    body.innerHTML = '<p>旧文档</p>'

    await wrapper.setProps({ src: '/static/uploads/featured/docx/next_1700000000001.docx' })
    await flushPromises()

    expect(fetchMock).toHaveBeenLastCalledWith('/static/uploads/featured/docx/next_1700000000001.docx')
    expect(renderAsync).toHaveBeenCalledTimes(2)
    expect(body.innerHTML).not.toContain('旧文档')
    wrapper.unmount()
  })
})
