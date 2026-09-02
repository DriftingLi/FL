import { unwrappedRequest } from './request'
export interface ResumeData {
  user_id: number
  real_name: string
  contact_phone: string
  wechat: string
  region: string
  expected_position_id?: number | null
  expected_position_extra: string
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
  resume_file_url: string
  photos: string[]
  visibility: 'hidden' | 'open'
  created_at: string
  updated_at: string
}
export const resumeApi = {
  // #415：简历拉取走静默通道（X-Silent）——「未建即 404」是契约内空态，页面自行分类呈现，
  // 不再触发请求壳的统一 404 报错提示；个人资料页同样静音（只取数据展示）。
  get() { return unwrappedRequest.get<ResumeData>('/resume', { headers: { 'X-Silent': '1' } }) },
  save(data: any) { return unwrappedRequest.put<ResumeData>('/resume', data) },
  updateVisibility(visibility: 'hidden' | 'open') { return unwrappedRequest.put<ResumeData>('/resume/visibility', { visibility }) },
  uploadPdf(formData: FormData) { return unwrappedRequest.post<{ url: string }>('/resume/pdf', formData, { headers: { 'Content-Type': 'multipart/form-data' }, timeout: 120000 }) },
  deletePdf() { return unwrappedRequest.delete('/resume/pdf') },
  uploadImage(formData: FormData) { return unwrappedRequest.post<{ url: string }>('/resume/image', formData, { headers: { 'Content-Type': 'multipart/form-data' }, timeout: 120000 }) },
  getViewStats() { return unwrappedRequest.get<{ count: number }>('/resume/view-stats') },
  listContactRequests(params?: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<{ items: any[]; total: number }>('/resume/contact-requests', { params })
  },
  approveContactRequest(id: number | string) {
    return unwrappedRequest.post<any>(`/resume/contact-requests/${id}/approve`)
  },
  rejectContactRequest(id: number | string) {
    return unwrappedRequest.post<any>(`/resume/contact-requests/${id}/reject`)
  },
  revokeContactRequest(id: number | string) {
    return unwrappedRequest.post<any>(`/resume/contact-requests/${id}/revoke`)
  }
}
