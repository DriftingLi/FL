import request from './request'

export interface StartMockExamPayload {
  course_id?: number
  category?: string
  question_count?: number
  duration_minutes?: number
  [key: string]: unknown
}

export interface MockExamProgressPayload {
  current_index?: number
  answers_state?: Record<string, unknown>
  remaining_seconds?: number
  answers?: unknown
  remaining_time?: number
  [key: string]: unknown
}

export interface MockExamHistoryQuery {
  page?: number
  page_size?: number
}

export const mockExamApi = {
  startMockExam(data: StartMockExamPayload) {
    return request.post('/mock-exam/start', data)
  },

  saveProgress(mockExamId: number, data: MockExamProgressPayload) {
    return request.post(`/mock-exam/${mockExamId}/save`, data)
  },

  resumeMockExam(mockExamId: number) {
    return request.get(`/mock-exam/${mockExamId}/resume`)
  },

  submitMockExam(mockExamId: number) {
    return request.post(`/mock-exam/${mockExamId}/submit`)
  },

  getMockExamResult(mockExamId: number) {
    return request.get(`/mock-exam/${mockExamId}/result`)
  },

  getMockExamHistory(params: MockExamHistoryQuery) {
    return request.get('/mock-exam/history', { params })
  }
}
