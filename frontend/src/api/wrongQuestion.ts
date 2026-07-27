import request from './request'

export interface WrongQuestionsQuery {
  page?: number
  page_size?: number
  category?: string
  practice_type?: string
  type?: string
}

export const wrongQuestionApi = {
  getWrongQuestions(params: WrongQuestionsQuery) {
    return request.get('/wrong-questions', { params })
  },

  redoWrongQuestion(questionId: number, userAnswer: string) {
    return request.post(`/wrong-questions/${questionId}/redo`, { user_answer: userAnswer })
  },

  removeWrongQuestion(questionId: number) {
    return request.post(`/wrong-questions/${questionId}/remove`)
  },

  getWrongQuestionStats() {
    return request.get('/wrong-questions/stats')
  },

  exportWrongQuestions() {
    return request.get('/wrong-questions/export', { responseType: 'blob' })
  }
}
