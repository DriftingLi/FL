// useReportDownload 统一下载实现单测（#230）：
// downloadBlob 优先从 Content-Disposition 解析文件名、拿不到回退显式 fileName；
// parseContentDispositionFilename 覆盖 RFC 5987 UTF-8'' 与 filename= 两种形态。
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  downloadBlob,
  parseContentDispositionFilename,
  dispositionHeaderValue
} from '../useReportDownload'

describe('parseContentDispositionFilename', () => {
  it('缺省/null 返回 null', () => {
    expect(parseContentDispositionFilename(null)).toBeNull()
    expect(parseContentDispositionFilename(undefined)).toBeNull()
    expect(parseContentDispositionFilename('')).toBeNull()
  })

  it('优先 RFC 5987 filename*=UTF-8 并 URL 解码', () => {
    const header = "attachment; filename=\"export.csv\"; filename*=UTF-8''%E5%AD%A6%E5%91%98%E5%90%8D%E5%8D%95.csv"
    expect(parseContentDispositionFilename(header)).toBe('学员名单.csv')
  })

  it('无 filename*= 时回退 filename="..."（去引号）', () => {
    expect(parseContentDispositionFilename("attachment; filename=\"成绩单.csv\"")).toBe('成绩单.csv')
  })

  it('无引号 filename= 直接取用', () => {
    expect(parseContentDispositionFilename('attachment; filename=report.pdf')).toBe('report.pdf')
  })

  it('非法百分号编码时走 filename= 兜底', () => {
    const header = "attachment; filename=\"a.csv\"; filename*=UTF-8''%ZZ"
    expect(parseContentDispositionFilename(header)).toBe('a.csv')
  })
})

describe('dispositionHeaderValue', () => {
  it('字符串原样返回、空字符串/数组/缺省返回 null', () => {
    expect(dispositionHeaderValue('attachment; filename=x.csv')).toBe('attachment; filename=x.csv')
    expect(dispositionHeaderValue('')).toBeNull()
    expect(dispositionHeaderValue(['attachment'])).toBeNull()
    expect(dispositionHeaderValue(undefined)).toBeNull()
    expect(dispositionHeaderValue(null)).toBeNull()
  })
})

describe('downloadBlob 文件名解析', () => {
  const createObjectURL = vi.fn()
  const revokeObjectURL = vi.fn()
  let click: ReturnType<typeof vi.fn>
  // 单一锚点引用：downloadBlob 内部 createElement 返回同一对象，供断言 a.download
  let anchor: { download: string; href: string; style: Record<string, never>; click: ReturnType<typeof vi.fn>; remove: ReturnType<typeof vi.fn> }

  beforeEach(() => {
    click = vi.fn()
    anchor = { download: '', href: '', style: {}, click, remove: vi.fn() }
    createObjectURL.mockReturnValue('blob:mock')
    vi.stubGlobal('URL', { createObjectURL, revokeObjectURL })
    vi.stubGlobal('document', {
      createElement: () => anchor,
      body: { appendChild: vi.fn(), removeChild: vi.fn() }
    })
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.useRealTimers()
  })

  it('有 Content-Disposition 时下载锚点取响应头解析的文件名', () => {
    downloadBlob(new Blob(['x']), '回退.csv', {
      disposition: "attachment; filename=\"export.csv\"; filename*=UTF-8''%E9%A2%98%E5%BA%93.csv"
    })
    expect(click).toHaveBeenCalledTimes(1)
    expect(anchor.download).toBe('题库.csv')
  })

  it('无 Content-Disposition 时回退显式 fileName', () => {
    downloadBlob(new Blob(['x']), '学员名单.csv')
    expect(click).toHaveBeenCalledTimes(1)
    expect(anchor.download).toBe('学员名单.csv')
  })
})