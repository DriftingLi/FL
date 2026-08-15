import { unwrappedRequest } from './request'
import type { Question } from '@/types/question'

export interface QuestionsQuery {
  page?: number
  page_size?: number
  keyword?: string
  type?: string
  status?: string
  created_by?: number
  /** 按题库标签筛选（LH-28） */
  tag_id?: number
}

export interface QuestionPayload {
  type: string
  content: string
  options?: Record<string, unknown>
  answer: string
  explanation?: string
  image_url?: string
  reference_answer?: string
  scoring_criteria?: string
  score?: number
  status?: string
  /** 题库标签（LH-28，创建/更新时全量替换） */
  tag_ids?: number[]
}

export interface BatchRejectPayload {
  question_ids: number[]
  reason: string
}

/** 题库统计（学员端卡片用） */
export interface QuestionBankStats {
  total?: number
  published?: number
  pending?: number
  total_count?: number
}

export const questionBankApi = {
  getQuestions(params: QuestionsQuery) {
    return unwrappedRequest.get<{ questions: Question[]; total: number }>('/question-bank/questions', { params })
  },

  createQuestion(data: QuestionPayload) {
    return unwrappedRequest.post<Question>('/question-bank/questions', data)
  },

  getQuestion(id: number) {
    return unwrappedRequest.get<Question>(`/question-bank/questions/${id}`)
  },

  updateQuestion(id: number, data: Partial<QuestionPayload>) {
    return unwrappedRequest.put<Question>(`/question-bank/questions/${id}`, data)
  },

  deleteQuestion(id: number) {
    return unwrappedRequest.delete<null>(`/question-bank/questions/${id}`)
  },

  publishQuestion(id: number) {
    return unwrappedRequest.post<null>(`/question-bank/questions/${id}/publish`)
  },

  rejectQuestion(id: number, reason: string) {
    return unwrappedRequest.post<null>(`/question-bank/questions/${id}/reject`, { reason })
  },

  batchPublish(questionIds: number[]) {
    return unwrappedRequest.post<null>('/question-bank/questions/batch-publish', { question_ids: questionIds })
  },

  batchReject(questionIds: number[], reason: string) {
    return unwrappedRequest.post<null>('/question-bank/questions/batch-reject', { question_ids: questionIds, reason })
  },

  batchImport(questions: QuestionPayload[]) {
    return unwrappedRequest.post<{ success_count?: number; failed_count?: number }>('/question-bank/questions/batch-import', { questions })
  },

  getStats() {
    return unwrappedRequest.get<QuestionBankStats>('/question-bank/stats')
  },

  uploadImage(formData: FormData) {
    return unwrappedRequest.post<{ url: string }>('/question-bank/upload-image', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 30000
    })
  }
}
