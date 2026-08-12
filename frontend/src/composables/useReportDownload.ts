// 报告/文件 Blob 下载统一实现（结果页 PDF、导出文件共用）。
import { ElMessage } from 'element-plus'

/** 触发浏览器下载（createObjectURL → a.click → revoke） */
export function downloadBlob(blob: Blob, fileName: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = fileName
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
