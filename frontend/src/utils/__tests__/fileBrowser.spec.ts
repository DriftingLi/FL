import { afterEach, describe, expect, it, vi } from 'vitest'
import { downloadUrl, probeFileUrl } from '../fileBrowser'

afterEach(() => {
  vi.unstubAllGlobals()
  document.body.replaceChildren()
})

describe('probeFileUrl', () => {
  it('返回 HEAD 状态并保留网络错误标记', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ ok: true, status: 200 })
      .mockResolvedValueOnce({ ok: false, status: 404 })
      .mockRejectedValueOnce(new Error('offline'))
    vi.stubGlobal('fetch', fetchMock)

    await expect(probeFileUrl('/file')).resolves.toEqual({ ok: true, status: 200, networkError: false })
    await expect(probeFileUrl('/missing')).resolves.toEqual({ ok: false, status: 404, networkError: false })
    await expect(probeFileUrl('/offline')).resolves.toEqual({ ok: false, status: null, networkError: true })
    expect(fetchMock).toHaveBeenCalledWith('/file', { method: 'HEAD' })
  })
})

describe('downloadUrl', () => {
  it('按调用方设置 download/target 并清理插入 DOM 的链接', () => {
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})

    downloadUrl('/report.pdf', { fileName: '报告.pdf', target: '_blank', appendToBody: true })

    expect(document.body.children).toHaveLength(0)
    expect(click).toHaveBeenCalledTimes(1)
    click.mockRestore()
  })

  it('不插入 DOM 时保留浏览器直链下载形态', () => {
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})

    downloadUrl('/image.png', { fileName: '' })

    expect(document.body.children).toHaveLength(0)
    expect(click).toHaveBeenCalledTimes(1)
    click.mockRestore()
  })
})
