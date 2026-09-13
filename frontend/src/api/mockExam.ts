import { unwrappedRequest } from './request'
import type { Question } from '@/types/question'
import type {
  MockExamHistoryDTO,
  MockExamHistoryItemDTO,
  MockExamResultDTO,
  MockExamResumeDTO,
  MockExamStartDTO,
  MockExamSubmitDTO
} from './generated/mockExam'

// 响应类型**不再手写**：唯一事实源是后端注解 → backend/docs/swagger.json →
// `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3，spec #952 片一）。
// 生成物名带后端 DTO 后缀，既有前端名（MockExamHistoryItem）在下面起别名收口。
//
// 题目元素同 practiceMode：本片不接线，仍是 UI 模型 Question（见该文件头部的边界说明）。
export type {
  MockExamHistoryDTO,
  MockExamHistoryItemDTO as MockExamHistoryItem,
  MockExamResultDTO,
  MockExamResumeDTO,
  MockExamStartDTO,
  MockExamSubmitDTO
}

/** 用 UI 模型的题目元素替换生成 DTO 的 questions（见文件头边界说明）。 */
type WithUIQuestions<T extends { questions: unknown }> = Omit<T, 'questions'> & { questions: Question[] }

// 入参（query / body）类型**不生成**（ADR-0048 决策 3：swag 对 body 描述弱），故仍是手写；
// 写成 type 而非 interface 以区别于「手写响应类型」——本模块的响应形状一律来自生成物。
export type StartMockExamPayload = {
  course_id?: number
  category?: string
  question_count?: number
  duration_minutes?: number
}

export type MockExamProgressPayload = {
  current_index?: number
  answers_state?: Record<string, unknown>
  remaining_seconds?: number
  answers?: unknown
  remaining_time?: number
}

export type MockExamHistoryQuery = {
  page?: number
  page_size?: number
}

export const mockExamApi = {
  startMockExam(data: StartMockExamPayload) {
    return unwrappedRequest.post<WithUIQuestions<MockExamStartDTO>>('/mock-exam/start', data, { params: {} })
  },

  saveProgress(mockExamId: number, data: MockExamProgressPayload) {
    return unwrappedRequest.post<null>(`/mock-exam/${mockExamId}/save`, data)
  },

  resumeMockExam(mockExamId: number) {
    return unwrappedRequest.get<WithUIQuestions<MockExamResumeDTO>>(`/mock-exam/${mockExamId}/resume`)
  },

  submitMockExam(mockExamId: number) {
    return unwrappedRequest.post<MockExamSubmitDTO>(`/mock-exam/${mockExamId}/submit`)
  },

  getMockExamResult(mockExamId: number) {
    return unwrappedRequest.get<MockExamResultDTO>(`/mock-exam/${mockExamId}/result`)
  },

  getMockExamHistory(params: MockExamHistoryQuery) {
    return unwrappedRequest.get<MockExamHistoryDTO>('/mock-exam/history', { params })
  }
}
