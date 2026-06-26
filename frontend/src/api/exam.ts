import request from './request'

export const examApi = {
  getExamQuestions(courseId) {
    return request.get(`/exam/${courseId}`)
  },

  submitExam(courseId, answers) {
    return request.post(`/exam/${courseId}/submit`, { answers })
  },

  getExamResult(courseId) {
    return request.get(`/exam/${courseId}/result`)
  },

  getExamHistory() {
    return request.get('/exam/history')
  }
}
