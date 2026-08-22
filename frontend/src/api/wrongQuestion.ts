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

/** 重做判定结果（对齐后端 SubmitResultDTO：is_correct 简答 AI 判定前为 null） */
export interface RedoResult {
  is_correct?: boolean | null
  correct_answer?: string
  explanation?: string
  question_id?: number
  user_answer?: unknown
  reference_answer?: string
  scoring_criteria?: string
  max_score?: number
  ai_score?: number
  ai_comment?: string
  ai_fallback?: boolean
  accuracy_rate?: number
  common_wrong?: string
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

  batchRemoveWrongQuestions(questionIds: number[]) {
    return unwrappedRequest.post<null>('/wrong-questions/batch-remove', { question_ids: questionIds })
  },

  getWrongQuestionStats() {
    return unwrappedRequest.get<Record<string, unknown>>('/wrong-questions/stats')
  },

  exportWrongQuestions() {
    return unwrappedRequest.get<Blob>('/wrong-questions/export', { responseType: 'blob' })
  }
}
