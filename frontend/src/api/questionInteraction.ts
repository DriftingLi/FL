import { unwrappedRequest } from './request'
import type {
  QuestionCommentDTO,
  QuestionCommentPageResult,
  QuestionNote,
  QuestionTag
} from './generated/questionInteraction'

export const questionInteractionApi = {
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
