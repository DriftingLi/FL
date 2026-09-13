import { unwrappedRequest } from './request'
import type { WithUIQuestions } from '@/types/question'
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
// 别名规则同 practiceMode：旧名与生成形状对应才保留（MockExamHistoryItem）；
// MockExamResult 的旧形状含后端从不返回的 score、且缺 6 个真实字段，故删旧名改出 MockExamResultDTO。
export type {
  MockExamHistoryDTO,
  MockExamHistoryItemDTO as MockExamHistoryItem,
  MockExamResultDTO,
  MockExamResumeDTO,
  MockExamStartDTO,
  MockExamSubmitDTO
}

// 入参（query / body）类型**不生成**（ADR-0048 决策 3：swag 对 body 描述弱），故仍是手写。
// 写成 type 而非 interface 有两个直白的理由：本片验收⑤要求这三个模块里 `^export interface` 为空
// （该判据针对手写**响应**类型，入参不在其列），且 type 别名在阅读时与「响应形状来自生成物」
// 一眼可分。它们不参与任何响应形状，删掉会丢失模块既有的入参词汇。
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
