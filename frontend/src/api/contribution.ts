import { unwrappedRequest } from './request'

/** 投稿文件 DTO（对齐后端 service.ContributionFileDTO） */
export interface ContributionFile {
  file_id?: number
  file_name: string
  file_url: string
  file_size: number
  content_type: string
}

/** 投稿作者信息 */
export interface ContributionAuthor {
  user_id: number
  username: string
  anonymous?: boolean
}

/** 投稿状态：pending/approved/rejected/withdrawn/archived */
export type ContributionStatus = 'pending' | 'approved' | 'rejected' | 'withdrawn' | 'archived'

/** 投稿条目（对齐后端 ContributionItemDTO） */
export interface ContributionItem {
  id: number
  credential_id: number
  title: string
  intro: string
  status: ContributionStatus
  is_anonymous?: boolean
  downloads_count: number
  reject_reason?: string
  files?: ContributionFile[]
  author?: ContributionAuthor
  created_at: string
}

/** 分页结果 */
export interface ContributionPageData {
  items: ContributionItem[]
  total: number
  page: number
  page_size: number
}

/** 下载结果 */
export interface ContributionDownloadData {
  is_new: boolean
  tier_awarded: number
}

/** 举报条目（管理端） */
export interface ContributionReportItem {
  id: number
  reporter_id: number
  contribution_id: number
  contribution_title?: string
  reason: string
  status: number
  created_at: string
}

/** 举报分页 */
export interface ContributionReportPageData {
  items: ContributionReportItem[]
  total: number
  page: number
  page_size: number
}

/** 上传文件的响应（单文件） */
export interface ContributionUploadData {
  file_name: string
  file_url: string
  file_size: number
  content_type: string
}

export const contributionApi = {
  /** 先传文件（multipart），返回暂存 URL 与元数据。
   *  显式 multipart 头：共享 client 默认 Content-Type 为 application/json，
   *  不覆盖则 FormData 不会带 boundary，后端 FormFile 解析不到文件（同 forum.uploadImage 口径）。
   *  大文件（≤20MB/投稿 ≤50MB）上传耗时超默认 30s，超时放宽到 120s。 */
  uploadFile(file: File) {
    const fd = new FormData()
    fd.append('file', file)
    return unwrappedRequest.post<ContributionUploadData>('/contributions/upload-file', fd, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 120000
    })
  },
  /** 创建投稿 */
  create(payload: {
    credential_id: number
    title: string
    intro: string
    is_anonymous?: boolean
    files: { file_url: string; file_name: string; file_size: number; content_type: string }[]
  }) {
    return unwrappedRequest.post<ContributionItem>('/contributions', payload)
  },
  /** 公开广场（仅 approved，按证件过滤） */
  listPublic(params: { credential_id?: number; sort?: 'latest' | 'hot'; page?: number; page_size?: number }) {
    return unwrappedRequest.get<ContributionPageData>('/contributions', { params })
  },
  /** 我的投稿（全部状态） */
  listMine(params?: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<ContributionPageData>('/contributions/mine', { params })
  },
  /** 详情 */
  detail(id: number) {
    return unwrappedRequest.get<ContributionItem>(`/contributions/${id}`)
  },
  /** 下载（计数幂等） */
  download(id: number) {
    return unwrappedRequest.post<ContributionDownloadData>(`/contributions/${id}/download`)
  },
  /** 撤回 pending */
  withdraw(id: number) {
    return unwrappedRequest.delete(`/contributions/${id}`)
  },
  /** 举报（已上架） */
  report(id: number, reason: string) {
    return unwrappedRequest.post(`/contributions/${id}/report`, { reason })
  }
}

/** 管理端投稿审核与举报（admin/tutor 鉴权；讲师端 UI 二期） */
export const adminContributionApi = {
  /** 待审核队列 */
  listPending(params?: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<ContributionPageData>('/admin/contributions/pending', { params })
  },
  approve(id: number) {
    return unwrappedRequest.post<ContributionItem>(`/admin/contributions/${id}/approve`)
  },
  reject(id: number, reason: string) {
    return unwrappedRequest.post<ContributionItem>(`/admin/contributions/${id}/reject`, { reason })
  },
  archive(id: number, reason: string) {
    return unwrappedRequest.post<ContributionItem>(`/admin/contributions/${id}/archive`, { reason })
  },
  /** 举报队列：status 0 待处理 / 1 已处理，缺省全部 */
  listReports(params?: { status?: number; page?: number; page_size?: number }) {
    return unwrappedRequest.get<ContributionReportPageData>('/admin/contributions/reports', { params })
  },
  handleReport(id: number, action: 'archive' | 'dismiss') {
    return unwrappedRequest.post(`/admin/contributions/reports/${id}/handle`, { action })
  }
}
