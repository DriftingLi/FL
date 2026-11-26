export interface FileProbeResult {
  ok: boolean
  status: number | null
  networkError: boolean
}

export async function probeFileUrl(url: string): Promise<FileProbeResult> {
  try {
    const response = await fetch(url, { method: 'HEAD' })
    return { ok: response.ok, status: response.status, networkError: false }
  } catch {
    return { ok: false, status: null, networkError: true }
  }
}

export interface DownloadUrlOptions {
  fileName?: string
  target?: string
  appendToBody?: boolean
}

export function downloadUrl(url: string, options: DownloadUrlOptions = {}): void {
  const link = document.createElement('a')
  link.href = url
  if (options.fileName !== undefined) link.download = options.fileName
  if (options.target !== undefined) link.target = options.target
  if (options.appendToBody) document.body.appendChild(link)
  link.click()
  if (options.appendToBody) document.body.removeChild(link)
}
