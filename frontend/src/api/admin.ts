import request from './request'

export const adminApi = {
  getStudents(params) {
    return request.get('/admin/students', { params })
  },

  addStudent(data) {
    return request.post('/admin/student', data)
  },

  deleteStudent(id) {
    return request.delete(`/admin/student/${id}`)
  },

  getTutors(params) {
    return request.get('/admin/tutors', { params })
  },

  addTutor(data) {
    return request.post('/admin/tutor', data)
  },

  deleteTutor(id) {
    return request.delete(`/admin/tutor/${id}`)
  },

  getStatistics() {
    return request.get('/admin/statistics')
  },

  generateContent(data) {
    return request.post('/admin/course/generate-content', data)
  },

  getGenerateStatus(taskId) {
    return request.get(`/admin/course/generate-content/${taskId}`)
  },

  getCourses(params) {
    return request.get('/admin/courses', { params })
  },

  getCourseDetail(id) {
    return request.get(`/admin/course/${id}`)
  },

  createCourse(data) {
    return request.post('/admin/course', data)
  },

  updateCourse(id, data) {
    return request.put(`/admin/course/${id}`, data)
  },

  deleteCourse(id) {
    return request.delete(`/admin/course/${id}`)
  },

  createChapter(courseId, data) {
    return request.post(`/admin/course/${courseId}/chapter`, data)
  },

  updateChapter(chapterId, data) {
    return request.put(`/admin/chapter/${chapterId}`, data)
  },

  deleteChapter(chapterId) {
    return request.delete(`/admin/chapter/${chapterId}`)
  }
}
