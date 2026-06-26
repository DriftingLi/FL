import request from './request'

export const wrongQuestionApi = {
  getWrongQuestions(params) {
    return request.get('/wrong-questions', { params })
  },
  redoWrongQuestion(questionId, userAnswer) {
    return request.post(`/wrong-questions/${questionId}/redo`, { user_answer: userAnswer })
  },
  removeWrongQuestion(questionId) {
    return request.post(`/wrong-questions/${questionId}/remove`)
  },
  getWrongQuestionStats() {
    return request.get('/wrong-questions/stats')
  },
  exportWrongQuestions() {
    return request.get('/wrong-questions/export', { responseType: 'blob' })
  }
}
