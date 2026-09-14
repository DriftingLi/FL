import { unwrappedRequest } from './request'
import type {
  QuestionCommentDTO,
  QuestionCommentPageResult,
  QuestionNote,
  QuestionTag
} from './generated/questionInteraction'
import type { QuestionDTO } from './generated/questionBank'

export const questionInteractionApi = {
  /**
   * 学员侧按 id 取题（ADR-0049 决策 4 的落点）。
   *
   * 端点与导师编辑面共用，但**读路径按能力分流**：学员走题库池口径
   * （published + 排源标记真题题 + 当前证件），不满足返回 404。
   * 响应形状是生成物 QuestionDTO；页面消费的 UI 模型 Question 由调用方显式映射
   * （元素类型收口归 questionBank 片，ADR-0048 决策 7）。
   */
  getQuestion(questionId: number) {
    return unwrappedRequest.get<QuestionDTO>('/question-bank/questions/' + questionId)
  },
  listComments(questionId: number, params?: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<QuestionCommentPageResult>(`/questions/${questionId}/comments`, { params })
  },
  createComment(questionId: number, data: { content: string }) {
    return unwrappedRequest.post<QuestionCommentDTO>(`/questions/${questionId}/comments`, data)
  },
  deleteComment(commentId: number) {
    return unwrappedRequest.delete(`/questions/comments/${commentId}`)
  },
  /**
   * 未写笔记时后端 data 为 null（注解层只能指认 $ref，表达不了顶层 data 可空），
   * 故这里显式加 `| null`，调用方必须判空。
   */
  getNote(questionId: number) {
    return unwrappedRequest.get<QuestionNote | null>(`/questions/${questionId}/note`)
  },
  upsertNote(questionId: number, data: { content: string }) {
    return unwrappedRequest.put<QuestionNote>(`/questions/${questionId}/note`, data)
  },
  deleteNote(questionId: number) {
    return unwrappedRequest.delete(`/questions/${questionId}/note`)
  },
  listKnowledge(questionId: number) {
    return unwrappedRequest.get<QuestionTag[]>(`/questions/${questionId}/knowledge`)
  }
}
