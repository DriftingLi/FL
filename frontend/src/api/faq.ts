import { unwrappedRequest } from './request'
import type {
  AdminFaqCategoriesResult,
  AdminFaqCategoryDTO,
  AdminFaqEntriesResult,
  AdminFaqEntryDTO,
  FaqResult
} from './generated/faq'

/** 学员端帮助中心整页（分类 + 已发布条目）—— 形状来自生成物（ADR-0048） */
export type HelpCenter = FaqResult
export type AdminFaqCategory = AdminFaqCategoryDTO
export type AdminFaqEntry = AdminFaqEntryDTO

export interface AdminFaqCategoryPayload {
  code: string
  title: string
  sort_order: number
  enabled: boolean
}

export interface AdminFaqEntryPayload {
  category_id: number
  question: string
  answer: string
  sort_order: number
  published: boolean
}

export const faqApi = {
  /** 学员端帮助中心：一次取回分类 + 全部已发布条目（搜索走端上过滤，无搜索接口） */
  getHelpCenter() {
    return unwrappedRequest.get<FaqResult>('/faq')
  },

  adminListCategories() {
    return unwrappedRequest.get<AdminFaqCategoriesResult>('/admin/faq/categories')
  },
  adminCreateCategory(data: AdminFaqCategoryPayload) {
    return unwrappedRequest.post<AdminFaqCategoryDTO>('/admin/faq/categories', data)
  },
  adminUpdateCategory(id: number, data: AdminFaqCategoryPayload) {
    return unwrappedRequest.put<AdminFaqCategoryDTO>(`/admin/faq/categories/${id}`, data)
  },
  adminDeleteCategory(id: number) {
    return unwrappedRequest.delete(`/admin/faq/categories/${id}`)
  },

  adminListEntries(params?: { category_id?: number }) {
    return unwrappedRequest.get<AdminFaqEntriesResult>('/admin/faq/entries', { params })
  },
  adminCreateEntry(data: AdminFaqEntryPayload) {
    return unwrappedRequest.post<AdminFaqEntryDTO>('/admin/faq/entries', data)
  },
  adminUpdateEntry(id: number, data: AdminFaqEntryPayload) {
    return unwrappedRequest.put<AdminFaqEntryDTO>(`/admin/faq/entries/${id}`, data)
  },
  adminDeleteEntry(id: number) {
    return unwrappedRequest.delete(`/admin/faq/entries/${id}`)
  }
}
