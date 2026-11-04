// imageCompress spec（票 2244）：把用户选的图压到长边 ≤1400 的 JPEG。
//
// 为什么值得锁：这是本仓**第一处**图片压缩（此前无先例），参数是照抄外部诊断助手的实测值
// （长边 1400「够 VL 看清仪表报警文字」/ JPEG q0.82）。诊断靠多模态识别仪表盘、铭牌、
// 线束，压太小读不出字，压太大单张 base64 会顶到后端 adapter 的 5MB 上限。
//
// 同时锁住三条降级：非位图不碰、环境无 canvas 原样返回、压完反而更大就用原图 ——
// 压缩是优化不是前置条件，任何一步失败都不能阻断上传。
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { IMAGE_JPEG_QUALITY, IMAGE_MAX_EDGE, compressImage } from '../imageCompress'

/** 可控的 createImageBitmap / canvas 替身：happy-dom 没有真 canvas */
interface FakeBitmap {
  width: number
  height: number
  close: () => void
}
let bitmapSize: { width: number; height: number } = { width: 4000, height: 3000 }
let blobSize = 1000
let canvases: Array<{ width: number; height: number; drawnTo: [number, number, number, number] | null }> = []
let toBlobCalls: Array<{ type: string; quality: number }> = []

function installCanvas() {
  canvases = []
  toBlobCalls = []
  const originalCreate = document.createElement.bind(document)
  vi.spyOn(document, 'createElement').mockImplementation(((tag: string) => {
    if (tag !== 'canvas') return originalCreate(tag)
    const el = { width: 0, height: 0, drawnTo: null as [number, number, number, number] | null } as any
    el.getContext = () => ({
      drawImage: (_bmp: unknown, _x: number, _y: number, w: number, h: number) => {
        el.drawnTo = [0, 0, w, h]
      }
    })
    el.toBlob = (cb: (b: Blob) => void, type: string, quality: number) => {
      toBlobCalls.push({ type, quality })
      cb(new Blob([new Uint8Array(blobSize)], { type }))
    }
    canvases.push(el)
    return el
  }) as typeof document.createElement)
}

function pngFile(name = 'photo.png', size = 5000): File {
  return new File([new Uint8Array(size)], name, { type: 'image/png' })
}

beforeEach(() => {
  bitmapSize = { width: 4000, height: 3000 }
  blobSize = 1000
  ;(globalThis as any).createImageBitmap = vi.fn(async (): Promise<FakeBitmap> => ({
    width: bitmapSize.width,
    height: bitmapSize.height,
    close: vi.fn()
  }))
  installCanvas()
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('压缩参数与几何', () => {
  it('常量就是对方实测的那组（1400 / 0.82）', () => {
    expect(IMAGE_MAX_EDGE).toBe(1400)
    expect(IMAGE_JPEG_QUALITY).toBe(0.82)
  })

  it('横图按长边压到 1400，等比缩放（4000×3000 → 1400×1050）', async () => {
    const { compressed } = await compressImage(pngFile())
    expect(compressed).toBe(true)
    expect(canvases[0].width).toBe(1400)
    expect(canvases[0].height).toBe(1050)
    expect(canvases[0].drawnTo).toEqual([0, 0, 1400, 1050])
  })

  it('竖图同样按长边（含小数时四舍五入）', async () => {
    bitmapSize = { width: 1050, height: 3000 }
    await compressImage(pngFile())
    // 3000 → 1400 的比例是 0.4667，1050×0.4667 = 490
    expect(canvases[0].height).toBe(1400)
    expect(canvases[0].width).toBe(490)
  })

  it('编码一律走 image/jpeg 与 q0.82', async () => {
    await compressImage(pngFile())
    expect(toBlobCalls).toEqual([{ type: 'image/jpeg', quality: 0.82 }])
  })

  it('文件名换成 .jpg（保留原名便于在队列里认出是哪张）', async () => {
    const { file } = await compressImage(pngFile('仪表盘截图.png'))
    expect(file.name).toBe('仪表盘截图.jpg')
    expect(file.type).toBe('image/jpeg')
  })
})

describe('不压的四种情况', () => {
  it('本来就够小（长边 ≤1400）不重编码 —— 再编一次只掉画质', async () => {
    bitmapSize = { width: 1200, height: 900 }
    const input = pngFile()
    const { file, compressed } = await compressImage(input)
    expect(compressed).toBe(false)
    expect(file).toBe(input)
    expect(toBlobCalls).toEqual([])
  })

  it('压完反而更大就退回原图', async () => {
    blobSize = 9000 // > 原图 5000 字节
    const input = pngFile()
    const { file, compressed } = await compressImage(input)
    expect(compressed).toBe(false)
    expect(file).toBe(input)
  })

  it('svg 不进 canvas（重编码会把矢量图糊掉）', async () => {
    const input = new File([new Uint8Array(10)], 'a.svg', { type: 'image/svg+xml' })
    const { file, compressed } = await compressImage(input)
    expect(compressed).toBe(false)
    expect(file).toBe(input)
    expect(canvases).toHaveLength(0)
  })

  it('非图片类型直接原样返回', async () => {
    const input = new File([new Uint8Array(10)], 'a.pdf', { type: 'application/pdf' })
    const { file, compressed } = await compressImage(input)
    expect(compressed).toBe(false)
    expect(file).toBe(input)
  })
})

describe('降级：压缩失败绝不阻断上传', () => {
  it('createImageBitmap 抛错时原样返回', async () => {
    ;(globalThis as any).createImageBitmap = vi.fn(async () => {
      throw new Error('decode failed')
    })
    const input = pngFile()
    const { file, compressed } = await compressImage(input)
    expect(compressed).toBe(false)
    expect(file).toBe(input)
  })

  it('没有 createImageBitmap（老环境）时原样返回', async () => {
    delete (globalThis as any).createImageBitmap
    const input = pngFile()
    const { file, compressed } = await compressImage(input)
    expect(compressed).toBe(false)
    expect(file).toBe(input)
  })

  it('canvas 拿不到 2d 上下文时原样返回', async () => {
    const originalCreate = document.createElement.bind(document)
    vi.spyOn(document, 'createElement').mockImplementation(((tag: string) => {
      if (tag !== 'canvas') return originalCreate(tag)
      return { width: 0, height: 0, getContext: () => null } as any
    }) as typeof document.createElement)
    const input = pngFile()
    const { file, compressed } = await compressImage(input)
    expect(compressed).toBe(false)
    expect(file).toBe(input)
  })

  it('bitmap 用完一定 close（不泄漏解码后的位图内存）', async () => {
    const close = vi.fn()
    ;(globalThis as any).createImageBitmap = vi.fn(async () => ({
      width: 4000,
      height: 3000,
      close
    }))
    await compressImage(pngFile())
    expect(close).toHaveBeenCalled()
  })
})
