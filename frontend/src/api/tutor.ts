import request from './request'

export const tutorApi = {
  getCourses(params) {
    return request.get('/tutor/courses', { params })
  },

  // 阅卷统计（按天分组），days=7|30
  getGradingStats(params?: { days?: number }) {
    return request.get('/tutor/grading-stats', { params })
  },

  getCourseChapters(courseId) {
    return request.get(`/tutor/course/${courseId}/chapters`)
  },

  getChapterDetail(chapterId) {
    return request.get(`/tutor/chapter/${chapterId}`)
  },

  uploadChapterFile(chapterId, formData, onProgress) {
    return request.post(`/tutor/chapter/${chapterId}/upload`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 300000,
      onUploadProgress: onProgress
    })
  },

  updateChapter(chapterId, data) {
    return request.put(`/tutor/chapter/${chapterId}`, data)
  },

  deleteFile(fileId) {
    return request.delete(`/tutor/file/${fileId}`)
  },

  batchDeleteFiles(data) {
    return request.post('/tutor/files/batch-delete', data)
  }
}
