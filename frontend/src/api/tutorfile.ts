// 导师文件库（#2494）：分片断点续传上传四段契约 + 列表/删除的请求壳。
//
// 响应类型来自生成面（ADR-0048）：frontend/src/api/generated/tutorfile.ts；
// 入参类型（init body）不生成、仍手写（决策 3）。
import { unwrappedRequest } from './request'
import type {
  FileDTO,
  FileListDTO,
  InitUploadResponse,
  UploadChunkResponse
} from './generated/tutorfile'

export type { FileDTO, FileListDTO, InitUploadResponse, UploadChunkResponse }

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

  /** 文件库列表 */
  listFiles() {
    return unwrappedRequest.get<FileListDTO>('/tutor/files')
  },

  /** 删除文件（文件行 + 存储对象） */
  deleteFile(fileId: number) {
    return unwrappedRequest.delete<null>(`/tutor/files/${fileId}`)
  }
}

/**
 * uploadFileInChunks 分片上传编排（spec A2）：分片大小以 init 返回的 chunk_size 为准
 * （服务端校验 5–8MiB 档后回传，唯一事实源在后端契约），除末片等大、逐片 PUT。
 * 断点续传（会话+已传清单持久化）归后续票（#2495）；此处为最小可用顺序上传。
 */
export async function uploadFileInChunks(
  file: File,
  onProgress?: (percent: number) => void
): Promise<FileDTO> {
  // 请求档位：契约下限 5MiB、上限 8MiB（服务端校验，spec A2）。
  const chunkSize = 8 * 1024 * 1024
  const init = await tutorFileApi.initUpload({
    file_name: file.name,
    file_size: file.size,
    chunk_size: chunkSize
  })
  const size = init.chunk_size
  try {
    for (let i = 0; i < init.total_chunks; i++) {
      const start = i * size
      const chunk = file.slice(start, Math.min(start + size, file.size))
      await tutorFileApi.uploadChunk(init.session_id, i + 1, chunk)
      onProgress?.(Math.round(((i + 1) / init.total_chunks) * 100))
    }
    return await tutorFileApi.completeUpload(init.session_id)
  } catch (err) {
    // 中断即放弃半成品，不把孤儿会话留给服务端等 TTL（#2495 会引入真正的断点续传）。
    await tutorFileApi.abortUpload(init.session_id).catch(() => undefined)
    throw err
  }
}
