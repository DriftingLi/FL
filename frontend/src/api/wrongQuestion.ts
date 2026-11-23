import { unwrappedRequest } from './request'
import type {
  SubmitResultDTO,
  WrongQuestionBatchRemoveResultDTO,
  WrongQuestionDTO,
  WrongQuestionPageDTO,
  WrongQuestionRemoveResultDTO,
  WrongQuestionStatsDTO
} from './generated/wrongQuestion'

/** 错题列表查询入参（保留手写：ADR-0048 决策 3 不生成入参类型） */
export type WrongQuestionCategory = number | 'all' | 'unclassified'

export interface WrongQuestionsQuery {
  page?: number
  page_size?: number
  practice_type?: string
  type?: string
  sort?: string
  favorited?: boolean
  min_wrong_count?: number
  category_id?: WrongQuestionCategory
}

/** 错题项 —— 响应形状来自生成物（ADR-0048） */
export type WrongQuestionItem = WrongQuestionDTO

/** 重做判定结果（生成物 SubmitResultDTO：is_correct 简答 AI 判定前为 null） */
export type RedoResult = SubmitResultDTO

export const wrongQuestionApi = {
  getWrongQuestions(params: WrongQuestionsQuery) {
    return unwrappedRequest.get<WrongQuestionPageDTO>('/wrong-questions', { params: { category_id: 'all', ...params } })
  },

  redoWrongQuestion(questionId: number, userAnswer: string, categoryId: WrongQuestionCategory = 'all') {
    return unwrappedRequest.post<RedoResult>(`/wrong-questions/${questionId}/redo`, { user_answer: userAnswer }, { params: { category_id: categoryId } })
  },

  // 移出返回 {removed}（单条 bool / 批量 int，两个 DTO 分开定型）——此前手写类型写成 null，属过时宽容
  removeWrongQuestion(questionId: number) {
    return unwrappedRequest.post<WrongQuestionRemoveResultDTO>(`/wrong-questions/${questionId}/remove`)
  },

  batchRemoveWrongQuestions(questionIds: number[]) {
    return unwrappedRequest.post<WrongQuestionBatchRemoveResultDTO>('/wrong-questions/batch-remove', { question_ids: questionIds })
  },

  getWrongQuestionStats(categoryId: WrongQuestionCategory = 'all') {
    return unwrappedRequest.get<WrongQuestionStatsDTO>('/wrong-questions/stats', { params: { category_id: categoryId } })
  },

  /** 纯文本附件导出（非信封面：响应体是 text/plain，不指认 data，域声明登记 NoData） */
  exportWrongQuestions(categoryId: WrongQuestionCategory = 'all') {
    return unwrappedRequest.get<Blob>('/wrong-questions/export', { responseType: 'blob', params: { category_id: categoryId } })
  }
}
