import request from './request'

export const courseApi = {
  getCourses(params) {
    return request.get('/courses', { params })
  },

  getCourseDetail(id) {
    return request.get(`/course/${id}`)
  },

  updateProgress(courseId, data) {
    return request.post(`/course/${courseId}/progress`, data)
  },

  getChapterDetail(courseId, chapterId) {
    return request.get(`/course/${courseId}/chapter/${chapterId}`)
  },

  getChapterSlides(chapterId) {
    return request.get(`/chapter/${chapterId}/slides`)
  },

  regenerateSlides(chapterId) {
    return request.post(`/chapter/${chapterId}/slides/regenerate`)
  }
}
