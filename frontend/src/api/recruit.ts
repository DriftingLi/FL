import { unwrappedRequest } from './request'

export interface RecruitResumeItem {
  id?: number
  user_id: number
  real_name?: string
  region?: string
  expected_specialty_extra?: string
  experience_years?: number
  visibility?: string
  updated_at?: string
}

export interface RecruitResumeListResp {
  items: RecruitResumeItem[]
  total: number
}

export const recruitApi = {
  getMe() {
    return unwrappedRequest.get<{ user_id: number; account: string; role: string }>('/recruit/me')
  },
  listResumes(params?: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<RecruitResumeListResp>('/recruit/resumes', { params })
  }
}
