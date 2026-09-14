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

// 名称适配：生成物沿用后端 DTO 命名，前端域词汇不带 DTO 后缀（既有 import 路径与类型名不破）。
export type {
  ContactPlainDTO as RecruitContactPlain,
  ContactRequestDTO as RecruitContactRequest,
  ContactRequestListResult as RecruitContactRequestList,
  RecruitListResult as RecruitResumeListResp,
  RecruitMeDTO,
  RecruitResumeCard as RecruitResumeItem
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
    return unwrappedRequest.get<RecruitListResult>('/recruit/resumes', { params })
  },
  getResume(id: number | string) {
    return unwrappedRequest.get<RecruitResumeCard>(`/recruit/resumes/${id}`)
  },
  createContactRequest(data: { student_user_id: number; message: string }) {
    return unwrappedRequest.post<ContactRequestDTO>('/recruit/contact-requests', data)
  },
  listMyRequests(params?: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<ContactRequestListResult>('/recruit/contact-requests', { params })
  },
  getContact(studentUserId: number | string) {
    return unwrappedRequest.get<ContactPlainDTO>(`/recruit/resumes/${studentUserId}/contact`)
  }
}
