import request from './request'

export interface UpdateProgressPayload {
  progress?: number
  study_duration?: number
  chapter_id?: number
  duration?: number
  [key: string]: unknown
}

export const courseApi = {
  getCourses(params: { page?: number; page_size?: number; category?: string; keyword?: string }) {
    return request.get('/courses', { params })
  },

  getCourseDetail(id: number) {
    return request.get(`/course/${id}`)
  },

  updateProgress(courseId: number, data: UpdateProgressPayload) {
    return request.post(`/course/${courseId}/progress`, data)
  },

  getChapterDetail(courseId: number, chapterId: number) {
    return request.get(`/course/${courseId}/chapter/${chapterId}`)
  },

  getChapterSlides(chapterId: number) {
    return request.get(`/chapter/${chapterId}/slides`)
  },

  regenerateSlides(chapterId: number) {
    return request.post(`/chapter/${chapterId}/slides/regenerate`)
  }
}
