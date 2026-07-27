import request from './request'

export interface GradeAnswerPayload {
  score: number
  comment?: string
}

export interface GradingStatsQuery {
  days?: number
}

export interface GradingParticipantsQuery {
  page?: number
  page_size?: number
  status?: string
  course_id?: number
}

export const gradingApi = {
  getSubmittedParticipants(params?: GradingParticipantsQuery) {
    return request.get('/grading/participants', { params })
  },

  getParticipantDetail(participantId: number) {
    return request.get(`/grading/participants/${participantId}`)
  },

  gradeAnswer(answerId: number, data: GradeAnswerPayload) {
    return request.post(`/grading/${answerId}/grade`, data)
  },

  regradeAnswer(answerId: number, data: GradeAnswerPayload) {
    return request.post(`/grading/${answerId}/regrade`, data)
  },

  getGradingStats(params: GradingStatsQuery) {
    return request.get('/grading/stats', { params })
  },

  confirmAiGrading(answerId: number) {
    return request.post(`/grading/${answerId}/confirm-ai`)
  },

  aiGradeAnswer(answerId: number) {
    return request.post(`/grading/${answerId}/ai-grade`)
  },

  confirmObjectiveAnswers(participantId: number) {
    return request.post(`/grading/participants/${participantId}/confirm-objective`)
  }
}
