// 报告/文件 Blob 下载统一实现（结果页 PDF、导出文件共用）。
import { ElMessage } from 'element-plus'

/**
 * 从 Content-Disposition 响应头解析文件名：
 * - 优先 RFC 5987 的 filename*=UTF-8''<url-encoded>（后端为文件名唯一真值，#230）；
 * - 其次回退 filename="<ascii>" / filename=<ascii>；
 * - 未命中返回 null（由调用方回退显式 fileName）。
 */
export function parseContentDispositionFilename(header: string | null | undefined): string | null {
  if (!header) return null
  const utf8 = /filename\*\s*=\s*UTF-8''([^;]+)/i.exec(header)
  if (utf8 && utf8[1]) {
    try {
      const name = decodeURIComponent(utf8[1].trim())
      if (name) return name
    } catch {
      /* 非法百分号编码时忽略，走后续回退 */
    }
  }
  const plain = /filename\s*=\s*"?([^";]*)"?/i.exec(header)
  if (plain && plain[1]) return plain[1].trim()
  return null
}

// 从 axios 响应头值（可能是 string / string[] / 缺省）归一化为字符串或 null。
export function dispositionHeaderValue(v: string | string[] | undefined | null): string | null {
  if (typeof v !== 'string' || v.trim() === '') return null
  return v
}

export interface DownloadBlobOptions {
  /** Content-Disposition 响应头（优先解析其中的文件名；拿不到时回退 fileName 参数） */
  disposition?: string | null
}

/** 触发浏览器下载（createObjectURL → a.click → revoke） */
export function downloadBlob(blob: Blob, fileName: string, opts: DownloadBlobOptions = {}) {
  const resolved = (opts.disposition ? parseContentDispositionFilename(opts.disposition) : null) ?? fileName
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = resolved
  a.style.display = 'none'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  setTimeout(() => URL.revokeObjectURL(url), 1500)
}

export interface DownloadReportOptions {
  /** 下载失败时的兜底（如 window.open 直链） */
  fallbackUrl?: string
  /** 下载成功后提示 */
  successMessage?: string
}

/** 报告下载：获取 blob → 落盘，失败可走兜底直链 */
export async function downloadReport(
  fetchBlob: () => Promise<Blob>,
  fileName: string,
  options: DownloadReportOptions = {}
) {
  try {
    const blob = await fetchBlob()
    downloadBlob(blob, fileName)
    if (options.successMessage) ElMessage.success(options.successMessage)
  } catch (e) {
    if (options.fallbackUrl) window.open(options.fallbackUrl, '_blank')
    void e
  }
}
