import { unwrappedRequest } from './request'

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
  }
}

/** 重做判定结果 */
export interface RedoResult {
  is_correct?: boolean
  correct_answer?: string
  explanation?: string
  accuracy_rate?: number | null
  common_wrong?: string | null
  total_attempts?: number
  ai_explanation?: string
}

export const wrongQuestionApi = {
  getWrongQuestions(params: WrongQuestionsQuery) {
    return unwrappedRequest.get<{ items: WrongQuestionItem[]; total: number }>('/wrong-questions', { params })
  },

  redoWrongQuestion(questionId: number, userAnswer: string) {
    return unwrappedRequest.post<RedoResult>(`/wrong-questions/${questionId}/redo`, { user_answer: userAnswer })
  },

  removeWrongQuestion(questionId: number) {
    return unwrappedRequest.post<null>(`/wrong-questions/${questionId}/remove`)
  },

  getWrongQuestionStats() {
    return unwrappedRequest.get<Record<string, unknown>>('/wrong-questions/stats')
  },

  exportWrongQuestions() {
    return unwrappedRequest.get<Blob>('/wrong-questions/export', { responseType: 'blob' })
  }
}
