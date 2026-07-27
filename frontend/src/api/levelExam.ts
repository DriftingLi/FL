import request from './request'

export interface LevelExamSessionQuery {
  page?: number
  page_size?: number
  status?: string
}

export interface LevelExamSessionPayload {
  name: string
  start_time: string
  end_time: string
  duration: number
  question_config?: Record<string, unknown>
  total_score?: number
  pass_score?: number
}

export interface SaveAnswerPayload {
  question_id?: number
  user_answer?: string
  time_used?: number
  answers?: unknown
  remaining_time?: number
  [key: string]: unknown
}

export interface SubmitExamPayload {
  answers: SaveAnswerPayload[]
  submit?: boolean
  [key: string]: unknown
}

export interface ExamHistoryQuery {
  page?: number
  page_size?: number
}

export const levelExamApi = {
  getSessions(params: LevelExamSessionQuery) {
    return request.get('/level-exam/sessions', { params })
  },

  createSession(data: LevelExamSessionPayload) {
    return request.post('/level-exam/sessions', data)
  },

  getSessionDetail(sessionId: number) {
    return request.get(`/level-exam/sessions/${sessionId}`)
  },

  updateSession(sessionId: number, data: LevelExamSessionPayload) {
    return request.put(`/level-exam/sessions/${sessionId}`, data)
  },

  deleteSession(sessionId: number) {
    return request.delete(`/level-exam/sessions/${sessionId}`)
  },

  updateSessionStatus(sessionId: number, status: string) {
    return request.put(`/level-exam/sessions/${sessionId}/status`, { status })
  },

  enterExam(sessionId: number) {
    return request.post(`/level-exam/sessions/${sessionId}/enter`)
  },

  saveAnswer(participantId: number, data: SaveAnswerPayload) {
    return request.post(`/level-exam/participants/${participantId}/save`, data)
  },

  submitExam(participantId: number, data: SubmitExamPayload) {
    return request.post(`/level-exam/participants/${participantId}/submit`, data)
  },

  getExamResult(participantId: number) {
    return request.get(`/level-exam/participants/${participantId}/result`)
  },

  getAvailableExams() {
    return request.get('/level-exam/available')
  },

  getExamHistory(params: ExamHistoryQuery) {
    return request.get('/level-exam/history', { params })
  }
}
