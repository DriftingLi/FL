import { unwrappedRequest } from './request'

export interface RecruitResumeItem {
  user_id: number
  real_name: string
  real_name_masked: string
  expected_specialty_id?: number | null
  expected_specialty_extra: string
  expected_regions: string[]
  salary_min?: number | null
  salary_max?: number | null
  salary_negotiable: boolean
  available_in: string
  job_nature: string
  experience_years: number
  self_intro: string
  resume_experiences: any[]
  resume_certifications: any[]
  updated_at: string
  // #489：企业视角联系状态
  contact_state?: 'none' | 'pending' | 'approved'
  contact_source?: 'recruiter' | 'application'
}

export interface RecruitResumeListResp {
  items: RecruitResumeItem[]
  total: number
}

export interface RecruitResumeListParams {
  page?: number
  page_size?: number
  region?: string
  specialty_id?: number
  credential_id?: number
  salary_min?: number
  salary_max?: number
  experience_years?: number
  available_in?: string
}

export const recruitApi = {
  getMe() {
    return unwrappedRequest.get<{ user_id: number; account: string; role: string }>('/recruit/me')
  },
  listResumes(params?: RecruitResumeListParams) {
    return unwrappedRequest.get<RecruitResumeListResp>('/recruit/resumes', { params })
  },
  getResume(id: number | string) {
    return unwrappedRequest.get<RecruitResumeItem>(`/recruit/resumes/${id}`)
  },
  createContactRequest(data: { student_user_id: number; message: string }) {
    return unwrappedRequest.post<any>('/recruit/contact-requests', data)
  },
  listMyRequests(params?: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<{ items: any[]; total: number }>('/recruit/contact-requests', { params })
  },
  getContact(studentUserId: number | string) {
    return unwrappedRequest.get<{ real_name: string; contact_phone: string; wechat: string; resume_file_url: string }>(`/recruit/resumes/${studentUserId}/contact`)
  }
}
