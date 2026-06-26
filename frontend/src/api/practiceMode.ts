import request from './request'

export const practiceModeApi = {
  getFreeQuestions(params) {
    return request.get('/practice-mode/free', { params })
  },
  getKnowledgePointPractice(params) {
    return request.get('/practice-mode/knowledge-point', { params })
  },
  getKnowledgePointProgress(params) {
    return request.get('/practice-mode/knowledge-point-progress', { params })
  },
  submitAnswer(data) {
    return request.post('/practice-mode/submit', data, { timeout: 60000 })
  },
  getStats() {
    return request.get('/practice-mode/stats')
  },
  getHistory(params) {
    return request.get('/practice-mode/history', { params })
  }
}
