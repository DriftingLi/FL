import request from './request'
import type { Question } from '@/types/question'

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

/** 模拟考历史记录项 */
export interface MockExamHistoryItem {
  id: number
  score?: number | null
  total_score?: number
  status?: string
  finished_at?: string
  [key: string]: unknown
}

/** 模拟考结果 */
export interface MockExamResult {
  score?: number
  total_score?: number
  correct_count?: number
  [key: string]: unknown
}

export const mockExamApi = {
  startMockExam(data: StartMockExamPayload) {
    return request.post<{ mock_exam_id: number; questions: Question[]; remaining_time: number }>('/mock-exam/start', data)
  },

  saveProgress(mockExamId: number, data: MockExamProgressPayload) {
    return request.post<null>(`/mock-exam/${mockExamId}/save`, data)
  },

  resumeMockExam(mockExamId: number) {
    return request.get<{ questions: Question[]; remaining_time: number }>(`/mock-exam/${mockExamId}/resume`)
  },

  submitMockExam(mockExamId: number) {
    return request.post<MockExamResult>(`/mock-exam/${mockExamId}/submit`)
  },

  getMockExamResult(mockExamId: number) {
    return request.get<MockExamResult>(`/mock-exam/${mockExamId}/result`)
  },

  getMockExamHistory(params: MockExamHistoryQuery) {
    return request.get<{ exams: MockExamHistoryItem[] }>('/mock-exam/history', { params })
  }
}
