import { unwrappedRequest } from './request'
import type {
  ContributionAuthor,
  ContributionFileDTO,
  ContributionItemDTO,
  ContributionPageResult,
  ContributionReportItemDTO,
  ContributionReportPageResult,
  DownloadResult
} from './generated/contribution'

/**
 * 投稿域（/api/contributions/*、/api/admin/contributions/*）。
 *
 * 响应类型**不再手写**：唯一事实源是后端注解 → backend/docs/swagger.json →
 * `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3，spec #952 片一）。
 * 本文件只留请求壳、端点装配与**名称适配**——生成物沿用后端 DTO 命名，
 * 前端域词汇不带 DTO 后缀，故在下面起别名（既有 import 路径与类型名不破）。
 * 可空 / 缺省态（file_id?、files?、reject_reason?）由注解层的 x-optional 表达，不在这里手改。
 *
 * 别名规则（片一统一口径）：旧名与生成形状确实对应时保留旧名（本模块全部如此）；
 * 旧名对应错形状时删旧名、改出生成名（见 practiceMode / mockExam 的文件头）。
 */
export type {
  ContributionAuthor,
  ContributionFileDTO as ContributionFile,
  ContributionFileDTO as ContributionUploadData,
  ContributionItemDTO as ContributionItem,
  ContributionPageResult as ContributionPageData,
  ContributionReportItemDTO as ContributionReportItem,
  ContributionReportPageResult as ContributionReportPageData,
  DownloadResult as ContributionDownloadData
}

/** 投稿状态：注解层只到 string（未标 enum），这里是前端收窄，消费处需断言。 */
export type ContributionStatus = 'pending' | 'approved' | 'rejected' | 'withdrawn' | 'archived'

export const contributionApi = {
  /** 先传文件（multipart），返回暂存 URL 与元数据。
   *  显式 multipart 头：共享 client 默认 Content-Type 为 application/json，
   *  不覆盖则 FormData 不会带 boundary，后端 FormFile 解析不到文件（同 forum.uploadImage 口径）。
   *  大文件（≤20MB/投稿 ≤50MB）上传耗时超默认 30s，超时放宽到 120s。 */
  uploadFile(file: File) {
    const fd = new FormData()
    fd.append('file', file)
    return unwrappedRequest.post<ContributionFileDTO>('/contributions/upload-file', fd, {
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
    return unwrappedRequest.post<ContributionItemDTO>('/contributions', payload)
  },
  /** 公开广场（仅 approved，按证件过滤） */
  listPublic(params: { credential_id?: number; sort?: 'latest' | 'hot'; page?: number; page_size?: number }) {
    return unwrappedRequest.get<ContributionPageResult>('/contributions', { params })
  },
  /** 我的投稿（全部状态） */
  listMine(params?: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<ContributionPageResult>('/contributions/mine', { params })
  },
  /** 详情 */
  detail(id: number) {
    return unwrappedRequest.get<ContributionItemDTO>(`/contributions/${id}`)
  },
  /** 下载（计数幂等） */
  download(id: number) {
    return unwrappedRequest.post<DownloadResult>(`/contributions/${id}/download`)
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
    return unwrappedRequest.get<ContributionPageResult>('/admin/contributions/pending', { params })
  },
  approve(id: number) {
    return unwrappedRequest.post<ContributionItemDTO>(`/admin/contributions/${id}/approve`)
  },
  reject(id: number, reason: string) {
    return unwrappedRequest.post<ContributionItemDTO>(`/admin/contributions/${id}/reject`, { reason })
  },
  archive(id: number, reason: string) {
    return unwrappedRequest.post<ContributionItemDTO>(`/admin/contributions/${id}/archive`, { reason })
  },
  /** 举报队列：status 0 待处理 / 1 已处理，缺省全部 */
  listReports(params?: { status?: number; page?: number; page_size?: number }) {
    return unwrappedRequest.get<ContributionReportPageResult>('/admin/contributions/reports', { params })
  },
  handleReport(id: number, action: 'archive' | 'dismiss') {
    return unwrappedRequest.post(`/admin/contributions/reports/${id}/handle`, { action })
  }
}
