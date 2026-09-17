// 已迁移模块：走 unwrappedRequest（拦截器解包信封，成功直接返回业务负载 Promise<T>）。
//
// 响应类型**不再手写**：唯一事实源是后端注解 → backend/docs/swagger.json →
// `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3，issue #963 片五）。
// 本文件只留请求壳、端点装配与名称适配；入参（query / body）类型不生成、仍手写。
import { unwrappedRequest } from './request'
import type {
  ContactPlainDTO,
  ContactRequestDTO,
  ContactRequestListResult,
  RecruitListResult,
  RecruitMeDTO,
  RecruitResumeCard
} from './generated/recruit'
import type { ContactRequestStatus, ContactState } from '@/utils/contactRequestStatus'

// 名称适配：生成物沿用后端 DTO 命名，前端域词汇不带 DTO 后缀（既有 import 路径与类型名不破）。
// 两处状态在生成物里只到 string：这里按 utils/contactRequestStatus 收窄——
// status 是五态 union（与后端 ContactGrantState 对账），contact_state 是它的三值投影。
export type RecruitContactRequest = Omit<ContactRequestDTO, 'status'> & { status: ContactRequestStatus }
export type RecruitContactRequestList = Omit<ContactRequestListResult, 'items'> & { items: RecruitContactRequest[] }
export type RecruitResumeItem = Omit<RecruitResumeCard, 'contact_state'> & { contact_state?: ContactState }
export type RecruitResumeListResp = Omit<RecruitListResult, 'items'> & { items: RecruitResumeItem[] }
export type {
  ContactPlainDTO as RecruitContactPlain,
  RecruitMeDTO
}

/** 简历库筛选（入参，不生成：ADR-0048 决策 3）。 */
export interface RecruitResumeListParams {
  page?: number
  page_size?: number
  region?: string
  position_id?: number
  credential_id?: number
  salary_min?: number
  salary_max?: number
  experience_years?: number
  available_in?: string
  job_nature?: string
}

export const recruitApi = {
  getMe() {
    return unwrappedRequest.get<RecruitMeDTO>('/recruit/me')
  },
  listResumes(params?: RecruitResumeListParams) {
    return unwrappedRequest.get<RecruitResumeListResp>('/recruit/resumes', { params })
  },
  getResume(id: number | string) {
    return unwrappedRequest.get<RecruitResumeItem>(`/recruit/resumes/${id}`)
  },
  createContactRequest(data: { student_user_id: number; message: string }) {
    return unwrappedRequest.post<RecruitContactRequest>('/recruit/contact-requests', data)
  },
  listMyRequests(params?: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<RecruitContactRequestList>('/recruit/contact-requests', { params })
  },
  getContact(studentUserId: number | string) {
    return unwrappedRequest.get<ContactPlainDTO>(`/recruit/resumes/${studentUserId}/contact`)
  }
}
