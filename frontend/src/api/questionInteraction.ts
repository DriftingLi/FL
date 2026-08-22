import { unwrappedRequest } from './request'

export const questionInteractionApi = {
  listComments(questionId: number, params?: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<{ items: any[]; total: number }>(`/questions/${questionId}/comments`, { params })
  },
  createComment(questionId: number, data: { content: string }) {
    return unwrappedRequest.post<any>(`/questions/${questionId}/comments`, data)
  },
  deleteComment(commentId: number) {
    return unwrappedRequest.delete(`/questions/comments/${commentId}`)
  },
  getNote(questionId: number) {
    return unwrappedRequest.get<any>(`/questions/${questionId}/note`)
  },
  upsertNote(questionId: number, data: { content: string }) {
    return unwrappedRequest.put<any>(`/questions/${questionId}/note`, data)
  },
  deleteNote(questionId: number) {
    return unwrappedRequest.delete(`/questions/${questionId}/note`)
  },
  listKnowledge(questionId: number) {
    return unwrappedRequest.get<any[]>(`/questions/${questionId}/knowledge`)
  }
}
