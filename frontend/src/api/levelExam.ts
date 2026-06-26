import request from './request'

export const levelExamApi = {
  getSessions(params) {
    return request.get('/level-exam/sessions', { params })
  },
  createSession(data) {
    return request.post('/level-exam/sessions', data)
  },
  getSessionDetail(sessionId) {
    return request.get(`/level-exam/sessions/${sessionId}`)
  },
  updateSession(sessionId, data) {
    return request.put(`/level-exam/sessions/${sessionId}`, data)
  },
  deleteSession(sessionId) {
    return request.delete(`/level-exam/sessions/${sessionId}`)
  },
  updateSessionStatus(sessionId, status) {
    return request.put(`/level-exam/sessions/${sessionId}/status`, { status })
  },
  enterExam(sessionId) {
    return request.post(`/level-exam/sessions/${sessionId}/enter`)
  },
  saveAnswer(participantId, data) {
    return request.post(`/level-exam/participants/${participantId}/save`, data)
  },
  submitExam(participantId, data) {
    return request.post(`/level-exam/participants/${participantId}/submit`, data)
  },
  getExamResult(participantId) {
    return request.get(`/level-exam/participants/${participantId}/result`)
  },
  getAvailableExams() {
    return request.get('/level-exam/available')
  },
  getExamHistory(params) {
    return request.get('/level-exam/history', { params })
  }
}
