import request from './request'

export interface QuestionsQuery {
  page?: number
  page_size?: number
  keyword?: string
  type?: string
  status?: string
  category?: string
  knowledge_point_id?: number
  created_by?: number
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

export const questionBankApi = {
  getQuestions(params: QuestionsQuery) {
    return request.get('/question-bank/questions', { params })
  },

  createQuestion(data: QuestionPayload) {
    return request.post('/question-bank/questions', data)
  },

  getQuestion(id: number) {
    return request.get(`/question-bank/questions/${id}`)
  },

  updateQuestion(id: number, data: Partial<QuestionPayload>) {
    return request.put(`/question-bank/questions/${id}`, data)
  },

  deleteQuestion(id: number) {
    return request.delete(`/question-bank/questions/${id}`)
  },

  publishQuestion(id: number) {
    return request.post(`/question-bank/questions/${id}/publish`)
  },

  rejectQuestion(id: number, reason: string) {
    return request.post(`/question-bank/questions/${id}/reject`, { reason })
  },

  batchPublish(questionIds: number[]) {
    return request.post('/question-bank/questions/batch-publish', { question_ids: questionIds })
  },

  batchReject(questionIds: number[], reason: string) {
    return request.post('/question-bank/questions/batch-reject', { question_ids: questionIds, reason })
  },

  batchImport(questions: QuestionPayload[]) {
    return request.post('/question-bank/questions/batch-import', { questions })
  },

  getStats() {
    return request.get('/question-bank/stats')
  },

  // 课程四分类及其题目数（章节练习用）
  getCategories() {
    return request.get('/question-bank/categories')
  },

  getKnowledgePoints(params?: KnowledgePointsQuery) {
    return request.get('/question-bank/knowledge-points', { params })
  },

  createKnowledgePoint(data: KnowledgePointPayload) {
    return request.post('/question-bank/knowledge-points', data)
  },

  updateKnowledgePoint(id: number, data: KnowledgePointPayload) {
    return request.put(`/question-bank/knowledge-points/${id}`, data)
  },

  deleteKnowledgePoint(id: number) {
    return request.delete(`/question-bank/knowledge-points/${id}`)
  },

  uploadImage(formData: FormData) {
    return request.post('/question-bank/upload-image', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 30000
    })
  }
}
