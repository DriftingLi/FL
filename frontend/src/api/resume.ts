import { unwrappedRequest } from './request'
export interface ResumeData {
  user_id: number
  real_name: string
  contact_phone: string
  wechat: string
  region: string
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
  resume_file_url: string
  photos: string[]
  visibility: 'hidden' | 'open'
  created_at: string
  updated_at: string
}
export const resumeApi = {
  get() { return unwrappedRequest.get<ResumeData>('/resume') },
  save(data: any) { return unwrappedRequest.put<ResumeData>('/resume', data) },
  updateVisibility(visibility: 'hidden' | 'open') { return unwrappedRequest.put<ResumeData>('/resume/visibility', { visibility }) },
  uploadPdf(formData: FormData) { return unwrappedRequest.post<{ url: string }>('/resume/pdf', formData, { headers: { 'Content-Type': 'multipart/form-data' }, timeout: 120000 }) },
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
