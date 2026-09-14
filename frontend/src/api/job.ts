// 已迁移模块：走 unwrappedRequest（拦截器解包信封，成功直接返回业务负载 Promise<T>）。
//
// 响应类型**不再手写**：唯一事实源是后端注解 → backend/docs/swagger.json →
// `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3，issue #963 片五）。
// 本文件只留请求壳、端点装配与名称适配；入参（query / body）类型不生成、仍手写。
import { unwrappedRequest } from './request'
import type {
  ApplicationDTO,
  ApplicationListResult,
  JobListResult,
  JobPostingDTO,
  RecruiterApplicationListResult,
  ReportDTO
} from './generated/job'

// 名称适配：生成物沿用后端 DTO 命名，前端域词汇不带 DTO 后缀（既有 import 路径与类型名不破）。
export type {
  ApplicationDTO as JobApplication,
  ApplicationListResult as ApplicationListResp,
  JobListResult as JobListResp,
  JobPostingDTO as JobPosting,
  RecruiterApplicationListResult as RecruiterApplicationListResp
}

/** 职位发布/编辑入参（不生成：ADR-0048 决策 3）。 */
export interface JobPostingInput {
  title: string
  position_id?: number | null
  region?: string
  salary_min?: number | null
  salary_max?: number | null
  salary_text?: string
  experience_req?: string
  description?: string
}

export const jobApi = {
  // 企业侧
  createJob(data: JobPostingInput) {
    return unwrappedRequest.post<JobPostingDTO>('/recruit/jobs', data)
  },
  updateJob(id: number, data: JobPostingInput) {
    return unwrappedRequest.put<JobPostingDTO>(`/recruit/jobs/${id}`, data)
  },
  toggleJobStatus(id: number) {
    return unwrappedRequest.post<JobPostingDTO>(`/recruit/jobs/${id}/toggle-status`)
  },
  listMyJobs(params?: { page?: number; page_size?: number; position_id?: number }) {
    return unwrappedRequest.get<JobListResult>('/recruit/jobs', { params })
  },
  getMyJob(id: number) {
    return unwrappedRequest.get<JobPostingDTO>(`/recruit/jobs/${id}`)
  },
  // 学员侧
  listPublicJobs(params?: { page?: number; page_size?: number; position_id?: number; region?: string; salary_min?: number; salary_max?: number; experience?: string }) {
    return unwrappedRequest.get<JobListResult>('/jobs', { params })
  },
  getPublicJob(id: number) {
    return unwrappedRequest.get<JobPostingDTO>(`/jobs/${id}`)
  },
  // 投递（投递即授权）
  applyJob(id: number) {
    return unwrappedRequest.post<ApplicationDTO>(`/jobs/${id}/apply`)
  },
  // 我的投递
  listMyApplications(params?: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<ApplicationListResult>('/resume/applications', { params })
  },
  withdrawApplication(id: number, revokeContact: boolean) {
    return unwrappedRequest.post<ApplicationDTO>(`/resume/applications/${id}/withdraw`, { revoke_contact: revokeContact })
  },
  // 举报
  reportJob(id: number, reason: string) {
    return unwrappedRequest.post<ReportDTO>(`/jobs/${id}/report`, { reason })
  },
  // 企业侧投递处理
  listJobApplications(jobId: number, params?: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<RecruiterApplicationListResult>(`/recruit/jobs/${jobId}/applications`, { params })
  },
  getApplicationDetail(id: number) {
    return unwrappedRequest.get<ApplicationDTO>(`/recruit/applications/${id}`)
  },
  rejectApplication(id: number) {
    return unwrappedRequest.post<ApplicationDTO>(`/recruit/applications/${id}/reject`)
  }
}
