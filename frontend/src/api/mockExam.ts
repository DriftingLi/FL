import { unwrappedRequest } from './request'
import type { Question } from '@/types/question'

export interface StartMockExamPayload {
  course_id?: number
  category?: string
  question_count?: number
  duration_minutes?: number
}

export interface MockExamProgressPayload {
  current_index?: number
  answers_state?: Record<string, unknown>
  remaining_seconds?: number
  answers?: unknown
  remaining_time?: number
}

export interface MockExamHistoryQuery {
  page?: number
  page_size?: number
}

/** 模拟考历史记录项（paper_id 为真题卷来源，omitempty，按卷考试才有 —— #390 契约） */
export interface MockExamHistoryItem {
  id: number
  score?: number | null
  total_score?: number
  status?: string
  finished_at?: string
  paper_id?: number
}

/** 模拟考结果 */
export interface MockExamResult {
  score?: number
  total_score?: number
  correct_count?: number
}

export const mockExamApi = {
  startMockExam(data: StartMockExamPayload) {
    return unwrappedRequest.post<{ mock_exam_id: number; questions: Question[]; remaining_time: number }>('/mock-exam/start', data)
  },

  saveProgress(mockExamId: number, data: MockExamProgressPayload) {
    return unwrappedRequest.post<null>(`/mock-exam/${mockExamId}/save`, data)
  },

  resumeMockExam(mockExamId: number) {
    return unwrappedRequest.get<{ questions: Question[]; remaining_time: number }>(`/mock-exam/${mockExamId}/resume`)
  },

  submitMockExam(mockExamId: number) {
    return unwrappedRequest.post<MockExamResult>(`/mock-exam/${mockExamId}/submit`)
  },

  getMockExamResult(mockExamId: number) {
    return unwrappedRequest.get<MockExamResult>(`/mock-exam/${mockExamId}/result`)
  },

  getMockExamHistory(params: MockExamHistoryQuery) {
    return unwrappedRequest.get<{ exams: MockExamHistoryItem[] }>('/mock-exam/history', { params })
  }
}
