import { unwrappedRequest } from './request'

export interface JobPosting {
  id: number
  recruiter_id: number
  title: string
  specialty_id?: number | null
  specialty_name?: string
  region: string
  salary_min?: number | null
  salary_max?: number | null
  salary_text: string
  experience_req: string
  description: string
  status: 'open' | 'closed'
  forced_offline: boolean
  offline_reason?: string
  published_at: string
  created_at: string
  updated_at: string
  company_name?: string
  business_scope?: string
  contact_name?: string
}

export interface JobListResp {
  items: JobPosting[]
  total: number
}

export interface JobPostingInput {
  title: string
  specialty_id?: number | null
  region?: string
  salary_min?: number | null
  salary_max?: number | null
  salary_text?: string
  experience_req?: string
  description?: string
}

export interface JobApplication {
  id: number
  job_posting_id: number
  job_title?: string
  recruiter_id: number
  student_user_id: number
  status: 'applied' | 'rejected' | 'withdrawn'
  resume_updated_at: string
  employer_viewed_at?: string | null
  created_at: string
  updated_at: string
  company_name?: string
}

export interface ApplicationListResp {
  items: JobApplication[]
  total: number
  page: number
  page_size: number
}

export const jobApi = {
  // 企业侧
  createJob(data: JobPostingInput) {
    return unwrappedRequest.post<JobPosting>('/recruit/jobs', data)
  },
  updateJob(id: number, data: JobPostingInput) {
    return unwrappedRequest.put<JobPosting>(`/recruit/jobs/${id}`, data)
  },
  toggleJobStatus(id: number) {
    return unwrappedRequest.post<JobPosting>(`/recruit/jobs/${id}/toggle-status`)
  },
  listMyJobs(params?: { page?: number; page_size?: number; specialty_id?: number }) {
    return unwrappedRequest.get<JobListResp>('/recruit/jobs', { params })
  },
  getMyJob(id: number) {
    return unwrappedRequest.get<JobPosting>(`/recruit/jobs/${id}`)
  },
  // 学员侧
  listPublicJobs(params?: { page?: number; page_size?: number; specialty_id?: number; region?: string; salary_min?: number; salary_max?: number; experience?: string }) {
    return unwrappedRequest.get<JobListResp>('/jobs', { params })
  },
  getPublicJob(id: number) {
    return unwrappedRequest.get<JobPosting>(`/jobs/${id}`)
  },
  // 投递（投递即授权）
  applyJob(id: number) {
    return unwrappedRequest.post<JobApplication>(`/jobs/${id}/apply`)
  },
  // 我的投递
  listMyApplications(params?: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<ApplicationListResp>('/resume/applications', { params })
  },
  withdrawApplication(id: number, revokeContact: boolean) {
    return unwrappedRequest.post<JobApplication>(`/resume/applications/${id}/withdraw`, { revoke_contact: revokeContact })
  }
}