import request from './request'
import type { Question } from '@/types/question'

export interface QuestionsQuery {
  page?: number
  page_size?: number
  keyword?: string
  type?: string
  status?: string
  category?: string
  knowledge_point_id?: number
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
  knowledge_point_id?: number
  category?: string
  status?: string
}

export interface KnowledgePointPayload {
  name: string
  category?: string
  parent_id?: number
  description?: string
}

export interface KnowledgePointsQuery {
  page?: number
  page_size?: number
  keyword?: string
  category?: string
}

export interface BatchRejectPayload {
  question_ids: number[]
  reason: string
}

/** 题库统计（学员端卡片用） */
export interface QuestionBankStats {
  total?: number
  [key: string]: unknown
}

/** 课程四分类及其题目数 */
export interface QuestionCategory {
  category: string
  count: number
  [key: string]: unknown
}

export const questionBankApi = {
  getQuestions(params: QuestionsQuery) {
    return request.get<{ questions: Question[]; total: number }>('/question-bank/questions', { params })
  },

  createQuestion(data: QuestionPayload) {
    return request.post<Question>('/question-bank/questions', data)
  },

  getQuestion(id: number) {
    return request.get<Question>(`/question-bank/questions/${id}`)
  },

  updateQuestion(id: number, data: Partial<QuestionPayload>) {
    return request.put<Question>(`/question-bank/questions/${id}`, data)
  },

  deleteQuestion(id: number) {
    return request.delete<null>(`/question-bank/questions/${id}`)
  },

  publishQuestion(id: number) {
    return request.post<null>(`/question-bank/questions/${id}/publish`)
  },

  rejectQuestion(id: number, reason: string) {
    return request.post<null>(`/question-bank/questions/${id}/reject`, { reason })
  },

  batchPublish(questionIds: number[]) {
    return request.post<null>('/question-bank/questions/batch-publish', { question_ids: questionIds })
  },

  batchReject(questionIds: number[], reason: string) {
    return request.post<null>('/question-bank/questions/batch-reject', { question_ids: questionIds, reason })
  },

  batchImport(questions: QuestionPayload[]) {
    return request.post<{ success_count?: number; failed_count?: number }>('/question-bank/questions/batch-import', { questions })
  },

  /** 题目打标（管理端/导师，LH-28） */
  setQuestionTags(questionId: number, tagIds: number[]) {
    return request.put<null>(`/question-bank/questions/${questionId}/tags`, { tag_ids: tagIds })
  },

  getStats() {
    return request.get<QuestionBankStats>('/question-bank/stats')
  },

  // 课程四分类及其题目数（章节练习用）
  getCategories() {
    return request.get<QuestionCategory[]>('/question-bank/categories')
  },

  getKnowledgePoints(params?: KnowledgePointsQuery) {
    return request.get<{ id: number; name: string; category?: string; parent_id?: number | null; description?: string }[]>(
      '/question-bank/knowledge-points',
      { params }
    )
  },

  createKnowledgePoint(data: KnowledgePointPayload) {
    return request.post<{ id: number }>('/question-bank/knowledge-points', data)
  },

  updateKnowledgePoint(id: number, data: KnowledgePointPayload) {
    return request.put<null>(`/question-bank/knowledge-points/${id}`, data)
  },

  deleteKnowledgePoint(id: number) {
    return request.delete<null>(`/question-bank/knowledge-points/${id}`)
  },

  uploadImage(formData: FormData) {
    return request.post<{ url: string }>('/question-bank/upload-image', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 30000
    })
  }
}
