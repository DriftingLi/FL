// 导师文件库（#2494）：分片断点续传上传四段契约 + 列表/删除的请求壳。
//
// 响应类型来自生成面（ADR-0048）：frontend/src/api/generated/tutorfile.ts；
// 入参类型（init body）不生成、仍手写（决策 3）。
import { unwrappedRequest } from './request'
import type {
  FileDTO,
  FileListDTO,
  InitUploadResponse,
  SessionStatusDTO,
  UploadChunkResponse
} from './generated/tutorfile'

export type { FileDTO, FileListDTO, InitUploadResponse, SessionStatusDTO, UploadChunkResponse }

/**
 * 上传文件类型的 accept 串（文件选择器过滤用）。
 * **镜像**后端类型表 backend/internal/filestore/file_type_table.go 的 upload 列四档
 * （video/document/ppt/image）——类型表是唯一事实源，改表时同步此处；
 * 不符的类型服务端 init 也会拒绝（400），这里只是选择器体验层。
 */
export const TUTOR_FILE_ACCEPT =
  '.pdf,.ppt,.pptx,.mp4,.webm,.png,.jpg,.jpeg,.gif,.webp,.bmp'

/** 创建上传会话入参（不生成，ADR-0048 决策 3） */
export interface InitUploadPayload {
  file_name: string
  file_size: number
  chunk_size: number
}

export const tutorFileApi = {
  /** init：创建上传会话（spec A1） */
  initUpload(data: InitUploadPayload) {
    return unwrappedRequest.post<InitUploadResponse>('/tutor/files/upload', data)
  },

  /** chunk：上传分片（原始字节流，同序号重传幂等覆盖，spec A1/A8） */
  uploadChunk(sessionId: string, partNumber: number, chunk: Blob) {
    return unwrappedRequest.put<UploadChunkResponse>(
      `/tutor/files/upload/${sessionId}/chunk/${partNumber}`,
      chunk,
      {
        headers: { 'Content-Type': 'application/octet-stream' },
        timeout: 300000
      }
    )
  },

  /** complete：拼装产出文件行（spec A1/A5） */
  completeUpload(sessionId: string) {
    return unwrappedRequest.post<FileDTO>(`/tutor/files/upload/${sessionId}/complete`)
  },

  /** abort：放弃会话并清理半成品（spec A1） */
  abortUpload(sessionId: string) {
    return unwrappedRequest.post<null>(`/tutor/files/upload/${sessionId}/abort`)
  },

  /** 断点续传清单：会话状态 + 已收分片序号（spec A8，服务端为准） */
  getSession(sessionId: string) {
    return unwrappedRequest.get<SessionStatusDTO>(`/tutor/files/upload/${sessionId}`)
  },

  /** 文件库列表 */
  listFiles() {
    return unwrappedRequest.get<FileListDTO>('/tutor/files')
  },

  /** 删除文件（文件行 + 存储对象） */
  deleteFile(fileId: number) {
    return unwrappedRequest.delete<null>(`/tutor/files/${fileId}`)
  }
}

/** 断点续传的客户端持久化（spec A8）：会话 id + 已完成分片清单，localStorage 单点。 */
export interface ResumableUploadState {
  session_id: string
  file_name: string
  file_size: number
  /** 文件修改时间（File.lastModified）：同名同大小异内容靠它区分，防错挂他人会话 */
  last_modified: number
  done_parts: number[]
}

function uploadStateKey(file: { name: string; size: number; lastModified: number }): string {
  return `tutorfile-upload:${file.name}:${file.size}:${file.lastModified}`
}

function readUploadState(file: { name: string; size: number; lastModified: number }): ResumableUploadState | null {
  try {
    const raw = localStorage.getItem(uploadStateKey(file))
    return raw ? (JSON.parse(raw) as ResumableUploadState) : null
  } catch {
    return null
  }
}

function saveUploadState(file: { name: string; size: number; lastModified: number }, state: ResumableUploadState): void {
  try {
    localStorage.setItem(uploadStateKey(file), JSON.stringify(state))
  } catch {
    // 持久化尽力而为：失败不影响本次上传，只是下次不能续传
  }
}

function clearUploadState(file: { name: string; size: number; lastModified: number }): void {
  try {
    localStorage.removeItem(uploadStateKey(file))
  } catch {
    // 同上
  }
}

/**
 * uploadFileInChunks 分片上传编排（spec A2/A8）：分片大小以会话登记的 chunk_size 为准
 * （服务端校验 5–8MiB 档后回传，唯一事实源在后端契约），除末片等大、逐片 PUT。
 *
 * 断点续传（A8）：会话 id + 已完成分片清单持久化在 localStorage；中断/重启后
 * 同一文件（名字+大小）再传，先查服务端清单（服务端为准）只补缺片。
 * 失败保留会话不 abort（正为续传而留；7 天未完成由服务端自动 abort，spec A3）。
 */
export async function uploadFileInChunks(
  file: File,
  onProgress?: (percent: number) => void
): Promise<FileDTO> {
  // 请求档位：契约下限 5MiB、上限 8MiB（服务端校验，spec A2）。
  const requestChunkSize = 8 * 1024 * 1024

  let sessionId = ''
  let chunkSize = requestChunkSize
  let totalChunks = 0
  const done = new Set<number>()

  // 续传：凭持久化的会话 id 查服务端清单（服务端以会话状态为准，本地清单只是进度记忆）
  const stored = readUploadState(file)
  if (stored) {
    try {
      const state = await tutorFileApi.getSession(stored.session_id)
      if (state.status === 'active' && state.file_size === file.size && state.file_name === file.name) {
        sessionId = state.session_id
        chunkSize = state.chunk_size
        totalChunks = state.total_chunks
        for (const n of state.received_parts) done.add(n)
      } else {
        clearUploadState(file)
      }
    } catch {
      clearUploadState(file) // 会话已不在（过期/被清），从头来
    }
  }

  if (!sessionId) {
    const init = await tutorFileApi.initUpload({
      file_name: file.name,
      file_size: file.size,
      chunk_size: requestChunkSize
    })
    sessionId = init.session_id
    chunkSize = init.chunk_size
    totalChunks = init.total_chunks
    saveUploadState(file, { session_id: sessionId, file_name: file.name, file_size: file.size, last_modified: file.lastModified, done_parts: [] })
  }

  onProgress?.(Math.round((done.size / totalChunks) * 100))
  for (let i = 0; i < totalChunks; i++) {
    const partNumber = i + 1
    if (done.has(partNumber)) continue // 只补缺片
    const start = i * chunkSize
    const chunk = file.slice(start, Math.min(start + chunkSize, file.size))
    await tutorFileApi.uploadChunk(sessionId, partNumber, chunk)
    done.add(partNumber)
    saveUploadState(file, {
      session_id: sessionId,
      file_name: file.name,
      file_size: file.size,
      last_modified: file.lastModified,
      done_parts: [...done]
    })
    onProgress?.(Math.round((done.size / totalChunks) * 100))
  }

  const result = await tutorFileApi.completeUpload(sessionId)
  clearUploadState(file)
  return result
}
