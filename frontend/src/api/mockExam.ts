import request from './request'

export const mockExamApi = {
  startMockExam(data) {
    return request.post('/mock-exam/start', data)
  },
  saveProgress(mockExamId, data) {
    return request.post(`/mock-exam/${mockExamId}/save`, data)
  },
  resumeMockExam(mockExamId) {
    return request.get(`/mock-exam/${mockExamId}/resume`)
  },
  submitMockExam(mockExamId) {
    return request.post(`/mock-exam/${mockExamId}/submit`)
  },
  getMockExamResult(mockExamId) {
    return request.get(`/mock-exam/${mockExamId}/result`)
  },
  getMockExamHistory(params) {
    return request.get('/mock-exam/history', { params })
  }
}
