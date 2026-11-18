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

  // ── 图片加载不出来（2026-11 用户反馈「word 文档预览的图片有时候会加载不出来」）──
  //
  // docx-preview 有两类静默留白，实测都不是「网络抖动」：
  //   1. 外部链接图片（TargetMode="External"）⇒ img.src 被设成字符串 "null"、naturalWidth 0；
  //   2. EMF/WMF 这类浏览器不认的矢量图 ⇒ src 正常、complete 为 true，但 naturalWidth 0。
  // 组件要能识别它们并给出可见提示，而不是让作者对着一块空白猜。
  describe('图片加载不出来的识别', () => {
    /** 让 renderAsync 往正文容器里塞指定图片，模拟 docx-preview 的产物 */
    function renderProducing(images: Array<{ src: string | null; naturalWidth: number }>) {
      renderAsync.mockImplementation((_blob: Blob, body: HTMLElement) => {
        body.innerHTML = ''
        for (const spec of images) {
          const img = document.createElement('img')
          if (spec.src !== null) img.setAttribute('src', spec.src)
          // happy-dom 里 naturalWidth 恒为 0，用 defineProperty 钉住，模拟「解码成功/失败」
          Object.defineProperty(img, 'complete', { value: true, configurable: true })
          Object.defineProperty(img, 'naturalWidth', { value: spec.naturalWidth, configurable: true })
          body.appendChild(img)
        }
        return Promise.resolve()
      })
    }

    it('外部链接的图片（src 是字符串 "null"）标成 external 并给出提示', async () => {
      renderProducing([{ src: null, naturalWidth: 0 }])
      const wrapper = mountDoc(DOCX_URL)
      await flushPromises()

      const img = wrapper.find('.docx-document__body img')
      expect(img.exists()).toBe(true)
      expect(img.attributes('data-docx-image-failed')).toBe('external')
      expect(wrapper.text()).toContain('有 1 张图片无法显示')
      wrapper.unmount()
    })

    it('浏览器解不开的格式（complete 但 naturalWidth 0）标成 format', async () => {
      renderProducing([{ src: 'blob:http://localhost/emf', naturalWidth: 0 }])
      const wrapper = mountDoc(DOCX_URL)
      await flushPromises()

      expect(wrapper.find('.docx-document__body img').attributes('data-docx-image-failed')).toBe('format')
      expect(wrapper.text()).toContain('有 1 张图片无法显示')
      wrapper.unmount()
    })

    it('正常图片不误伤：不标记、不弹提示', async () => {
      renderProducing([{ src: 'blob:http://localhost/ok', naturalWidth: 200 }])
      const wrapper = mountDoc(DOCX_URL)
      await flushPromises()

      const img = wrapper.find('.docx-document__body img')
      expect(img.attributes('data-docx-image-failed')).toBeUndefined()
      expect(wrapper.text()).not.toContain('张图片无法显示')
      wrapper.unmount()
    })

    it('多张里只有坏的那几张计数，提示写的是坏图数', async () => {
      renderProducing([
        { src: 'blob:http://localhost/ok', naturalWidth: 200 },
        { src: null, naturalWidth: 0 },
        { src: 'blob:http://localhost/emf', naturalWidth: 0 }
      ])
      const wrapper = mountDoc(DOCX_URL)
      await flushPromises()

      expect(wrapper.text()).toContain('有 2 张图片无法显示')
      wrapper.unmount()
    })

    it('还没解码完的图片（complete=false）先不判失败，等 load 之后再复查', async () => {
      renderAsync.mockImplementation((_blob: Blob, body: HTMLElement) => {
        body.innerHTML = ''
        const img = document.createElement('img')
        img.setAttribute('src', 'blob:http://localhost/pending')
        // complete=false + naturalWidth 0：这是「正在加载」，不是「解不开」
        Object.defineProperty(img, 'complete', { value: false, configurable: true })
        Object.defineProperty(img, 'naturalWidth', { value: 0, configurable: true })
        body.appendChild(img)
        return Promise.resolve()
      })
      const wrapper = mountDoc(DOCX_URL)
      await flushPromises()

      // 加载中的图不该被误判
      expect(wrapper.find('.docx-document__body img').attributes('data-docx-image-failed')).toBeUndefined()
      expect(wrapper.text()).not.toContain('张图片无法显示')

      // 模拟 load 事件落地后仍然 naturalWidth 0（真解不开）
      const img = wrapper.find('.docx-document__body img').element as HTMLImageElement
      Object.defineProperty(img, 'complete', { value: true, configurable: true })
      img.dispatchEvent(new Event('load'))
      await flushPromises()

      expect(wrapper.find('.docx-document__body img').attributes('data-docx-image-failed')).toBe('format')
      wrapper.unmount()
    })
  })
})
