// 图片上传前压缩（长边 1400 / JPEG q0.82）—— 票 2244。
//
// **本仓此前没有图片压缩先例**，这份是新的。参数照抄外部诊断助手前端（`app.js:809-815`）：
// 长边压到 1400px「够 VL 看清仪表报警文字，又不会让报文过大」，JPEG 质量 0.82。
// 不是随便挑的数：诊断要走多模态识别（拍仪表盘 / 铭牌 / 线束），压太小读不出字，
// 压太大则一张图 base64 后能顶到后端 adapter 的 5MB 单张上限（`diagnosisMaxImageCap`）。
//
// **为什么在选图时就压、而不是后端压**：后端拿到的已是 base64 dataURL，
// 服务端再解码-重编码要多占一份内存，且上传带宽已经浪费掉了。压缩发生在浏览器里，
// 传上去的就是压好的那一份。
//
// **降级**：任何一步失败（环境无 canvas / decode 失败 / toBlob 不支持）都**原样返回**，
// 不抛错、不阻断上传——压缩是优化，不是前置条件。真实的尺寸闸门在前端 `maxImages` 与
// 后端 `diagnosisMaxImageCap` 两处，这里退化成「不压」不会击穿它们。

/** 长边上限（px）。与外部助手 `app.js:810` 同值。 */
export const IMAGE_MAX_EDGE = 1400

/** JPEG 编码质量。与外部助手 `app.js:815` 同值。 */
export const IMAGE_JPEG_QUALITY = 0.82

/** 压缩结果：`file` 是要上传的那一份（压缩失败时就是入参本身） */
export interface CompressedImage {
  file: File
  /** 是否真的压过（false = 走了降级，调用方可据此决定要不要提示用户） */
  compressed: boolean
}

/**
 * 把一张图片压到长边 ≤ 1400px 的 JPEG。
 *
 * @param file 用户选的原图
 * @returns 压缩后的 File（保留原文件名，换成 .jpg 后缀与 image/jpeg）
 */
export async function compressImage(file: File): Promise<CompressedImage> {
  // 非位图（svg 等）不进 canvas：重编码会把矢量图糊掉，且 svg 本身通常很小
  if (!file.type.startsWith('image/') || file.type === 'image/svg+xml') {
    return { file, compressed: false }
  }
  if (typeof document === 'undefined' || typeof createImageBitmap !== 'function') {
    return { file, compressed: false }
  }
  let bitmap: ImageBitmap | null = null
  try {
    bitmap = await createImageBitmap(file)
    const { width, height } = bitmap
    const scale = Math.min(1, IMAGE_MAX_EDGE / Math.max(width, height))
    // 已经够小就不重编码：再编一次只会掉画质、省不下多少字节
    if (scale >= 1) return { file, compressed: false }

    const canvas = document.createElement('canvas')
    canvas.width = Math.max(1, Math.round(width * scale))
    canvas.height = Math.max(1, Math.round(height * scale))
    const ctx = canvas.getContext('2d')
    if (!ctx) return { file, compressed: false }
    ctx.drawImage(bitmap, 0, 0, canvas.width, canvas.height)

    const blob = await canvasToBlob(canvas, 'image/jpeg', IMAGE_JPEG_QUALITY)
    // 压完反而更大（小图 / 已高度压缩的 jpeg）就用原图，别为了「压过」而变大
    if (!blob || blob.size >= file.size) return { file, compressed: false }
    return { file: new File([blob], jpegName(file.name), { type: 'image/jpeg' }), compressed: true }
  } catch {
    return { file, compressed: false }
  } finally {
    bitmap?.close?.()
  }
}

function canvasToBlob(canvas: HTMLCanvasElement, type: string, quality: number): Promise<Blob | null> {
  return new Promise((resolve) => {
    if (typeof canvas.toBlob !== 'function') {
      resolve(null)
      return
    }
    canvas.toBlob((b) => resolve(b), type, quality)
  })
}

/** `photo.png` → `photo.jpg`（保留原名便于用户在队列里认出是哪张） */
function jpegName(name: string): string {
  const dot = name.lastIndexOf('.')
  return (dot > 0 ? name.slice(0, dot) : name) + '.jpg'
}
