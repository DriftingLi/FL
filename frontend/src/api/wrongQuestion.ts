import request from './request'

export interface WrongQuestionsQuery {
  page?: number
  page_size?: number
  practice_type?: string
  type?: string
}

/** 错题项 */
export interface WrongQuestionItem {
  id: number
  question_id: number
  wrong_count?: number
  question?: {
    type?: string
    content?: string
    options?: Record<string, string>
    [key: string]: unknown
  }
  [key: string]: unknown
}

/** 重做判定结果 */
export interface RedoResult {
  is_correct?: boolean
  [key: string]: unknown
}

export const wrongQuestionApi = {
  getWrongQuestions(params: WrongQuestionsQuery) {
    return request.get<{ items: WrongQuestionItem[]; total: number }>('/wrong-questions', { params })
  },

  redoWrongQuestion(questionId: number, userAnswer: string) {
    return request.post<RedoResult>(`/wrong-questions/${questionId}/redo`, { user_answer: userAnswer })
  },

  removeWrongQuestion(questionId: number) {
    return request.post<null>(`/wrong-questions/${questionId}/remove`)
  },

  getWrongQuestionStats() {
    return request.get<Record<string, unknown>>('/wrong-questions/stats')
  },

  exportWrongQuestions() {
    return request.get<Blob>('/wrong-questions/export', { responseType: 'blob' })
  }
}
