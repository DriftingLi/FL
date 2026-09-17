// 已迁移模块：走 unwrappedRequest（拦截器解包信封，成功直接返回业务负载 Promise<T>）。
//
// 响应类型**不再手写**：唯一事实源是后端注解 → backend/docs/swagger.json →
// `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3，issue #963 片五）。
// 本文件只留请求壳、端点装配与名称适配；入参（query / body）类型不生成、仍手写。
import { unwrappedRequest } from './request'
import type { ContactRequestDTO, ContactRequestListResult, JobCardDTO } from './generated/resume'
import type { ContactRequestStatus } from '@/utils/contactRequestStatus'

// 名称适配：生成物沿用后端 DTO 命名，前端域词汇不带 DTO 后缀（既有 import 路径与类型名不破）。
// status 在生成物里只到 string（注解层无 enum，ADR-0056 §8 否掉了补 enum 那条路）：
// 这里按 utils/contactRequestStatus 的 union 收窄，取值集合与后端 ContactGrantState 有对账锁。
export type ResumeContactRequest = Omit<ContactRequestDTO, 'status'> & { status: ContactRequestStatus }
export type ResumeContactRequestList = Omit<ContactRequestListResult, 'items'> & { items: ResumeContactRequest[] }
export type { JobCardDTO as ResumeData }

/** PDF / 工作照上传的 data 形状（注解层 inline object，非根类型，故此处内联）。 */
export interface ResumeUploadResult {
  url: string
}

/** 查看留痕聚合的 data 形状（注解层 inline object{count=integer}，非根类型，故此处内联）。 */
export interface ResumeViewStats {
  count: number
}

export const resumeApi = {
  // #415：简历拉取走静默通道（X-Silent）——「未建即 404」是契约内空态，页面自行分类呈现，
  // 不再触发请求壳的统一 404 报错提示；个人资料页同样静音（只取数据展示）。
  get() { return unwrappedRequest.get<JobCardDTO>('/resume', { headers: { 'X-Silent': '1' } }) },
  save(data: any) { return unwrappedRequest.put<JobCardDTO>('/resume', data) },
  updateVisibility(visibility: 'hidden' | 'open') { return unwrappedRequest.put<JobCardDTO>('/resume/visibility', { visibility }) },
  uploadPdf(formData: FormData) { return unwrappedRequest.post<ResumeUploadResult>('/resume/pdf', formData, { headers: { 'Content-Type': 'multipart/form-data' }, timeout: 120000 }) },
  deletePdf() { return unwrappedRequest.delete('/resume/pdf') },
  uploadImage(formData: FormData) { return unwrappedRequest.post<ResumeUploadResult>('/resume/image', formData, { headers: { 'Content-Type': 'multipart/form-data' }, timeout: 120000 }) },
  getViewStats() { return unwrappedRequest.get<ResumeViewStats>('/resume/view-stats') },
  listContactRequests(params?: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<ResumeContactRequestList>('/resume/contact-requests', { params })
  },
  approveContactRequest(id: number | string) {
    return unwrappedRequest.post<ResumeContactRequest>(`/resume/contact-requests/${id}/approve`)
  },
  rejectContactRequest(id: number | string) {
    return unwrappedRequest.post<ResumeContactRequest>(`/resume/contact-requests/${id}/reject`)
  },
  revokeContactRequest(id: number | string) {
    return unwrappedRequest.post<ResumeContactRequest>(`/resume/contact-requests/${id}/revoke`)
  }
}
