import request from './request'

export const gradingApi = {
  getSubmittedParticipants(params?) {
    return request.get('/grading/participants', { params })
  },
  getParticipantDetail(participantId) {
    return request.get(`/grading/participants/${participantId}`)
  },
  gradeAnswer(answerId, data) {
    return request.post(`/grading/${answerId}/grade`, data)
  },
  regradeAnswer(answerId, data) {
    return request.post(`/grading/${answerId}/regrade`, data)
  },
  getGradingStats(params) {
    return request.get('/grading/stats', { params })
  },
  confirmAiGrading(answerId) {
    return request.post(`/grading/${answerId}/confirm-ai`)
  },
  aiGradeAnswer(answerId) {
    return request.post(`/grading/${answerId}/ai-grade`)
  },
  confirmObjectiveAnswers(participantId) {
    return request.post(`/grading/participants/${participantId}/confirm-objective`)
  }
}
