import request from './request'

export const examApi = {
  getExamQuestions(courseId: number) {
    return request.get(`/exam/${courseId}`)
  },

  submitExam(courseId: number, answers: Record<string, unknown>) {
    return request.post(`/exam/${courseId}/submit`, { answers })
  },

  getExamResult(courseId: number) {
    return request.get(`/exam/${courseId}/result`)
  },

  getExamHistory() {
    return request.get('/exam/history')
  }
}
