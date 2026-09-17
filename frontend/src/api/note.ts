import { unwrappedRequest } from './request'
import type { NoteDTO, NotePageDTO } from './generated/note'

/** 「我的笔记」列表的筛选口径（ADR-0055）：全部 / 题目笔记 / 独立笔记 */
export type NoteScope = 'all' | 'question' | 'standalone'

/** 笔记条目 —— 响应形状来自生成物（ADR-0048） */
export type NoteItem = NoteDTO

export const noteApi = {
  /** 我的笔记（分页，按更新时间倒序） */
  list(params: { scope?: NoteScope; page?: number; page_size?: number }) {
    return unwrappedRequest.get<NotePageDTO>('/notes', { params })
  },

  /** 新建独立笔记（与题目无关） */
  create(data: { content: string }) {
    return unwrappedRequest.post<NoteDTO>('/notes', data)
  },

  /** 改笔记正文（题目笔记与独立笔记同一条路径） */
  update(id: number, data: { content: string }) {
    return unwrappedRequest.put<NoteDTO>(`/notes/${id}`, data)
  },

  /** 删笔记 */
  remove(id: number) {
    return unwrappedRequest.delete(`/notes/${id}`)
  }
}
