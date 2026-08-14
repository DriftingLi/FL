// 已迁移模块：走 unwrappedRequest（拦截器解包信封，成功直接返回业务数据 Promise<T>，
// 业务失败抛错并统一 toast，调用方不再自检 res.code）
import { unwrappedRequest } from './request'
import type { CourseSummary } from './course'

// ===== 培训目录体系：专业方向(specialty) → 等级(level) → 课程 → 章节 =====
// 契约与后端 LH-27 真实路由/字段对齐：
//   学员端公开  /catalog/tree、/specialties、/levels、/tags
//   管理端      /admin/specialty*、/admin/level*、/admin/certificate-template*、
//              /admin/question-tag*、/admin/question/:id/tags、/admin/catalog/tree（后端补齐）

/** 专业方向 */
export interface CatalogDirection {
  specialty_id: number
  name: string
  code?: string
  description?: string
  sort_order?: number
  status?: number
  created_at?: string
  [key: string]: unknown
}

/** 课程等级（全局共享，不归属方向） */
export interface CatalogLevel {
  level_id: number
  name: string
  code?: string
  description?: string
  sort_order?: number
  status?: number
  created_at?: string
  [key: string]: unknown
}

/** 证书模板（有效期单位为天 validity_days） */
export interface CertificateTemplate {
  id: number
  name: string
  code?: string
  description?: string
  validity_days?: number
  template_url?: string
  status?: number
  created_at?: string
  updated_at?: string
  [key: string]: unknown
}

/** 目录树中的课程节点 = CourseSummary + sort_order（sort_order 由后端补齐），单一事实源派生 */
export type CatalogCourseNode = CourseSummary & { sort_order?: number }

/** 等级节点（含课程） */
export interface CatalogLevelNode extends CatalogLevel {
  courses?: CatalogCourseNode[]
}

/** 方向节点（含等级） */
export interface CatalogDirectionNode extends CatalogDirection {
  levels?: CatalogLevelNode[]
}

/** 完整目录树（公开/管理端均为 {specialties}） */
export interface CatalogTree {
  specialties: CatalogDirectionNode[]
}

/** 题库标签（管理端含停用项，question_count 由后端补齐） */
export interface QuestionTag {
  id: number
  name: string
  code?: string
  description?: string
  sort_order?: number
  status?: number
  question_count?: number
  created_at?: string
  updated_at?: string
  [key: string]: unknown
}

export interface TagPayload {
  name: string
  code?: string
  description?: string
  sort_order?: number
  status?: number
}

export interface CertificateTemplatePayload {
  name: string
  code?: string
  description?: string
  validity_days?: number
  template_url?: string
  status?: number
}

export const trainingApi = {
  // ===== 目录树 =====
  /** 公开目录树（学员端筛选用）：GET /api/catalog/tree → {specialties} */
  getCatalogTree() {
    return unwrappedRequest.get<CatalogTree>('/catalog/tree')
  },
  /** 全局课程等级列表（仅启用项）：GET /api/levels */
  getLevels() {
    return unwrappedRequest.get<{ levels: CatalogLevel[] }>('/levels')
  },
  /** 管理端目录树（含停用项/章节）：GET /api/admin/catalog/tree → {specialties}（后端补齐） */
  getAdminCatalogTree() {
    return unwrappedRequest.get<CatalogTree>('/admin/catalog/tree')
  },

  // ===== 专业方向（后端路由 /admin/specialty*） =====
  createDirection(data: { name: string; code?: string; description?: string; sort_order?: number; status?: number }) {
    return unwrappedRequest.post<{ specialty_id: number }>('/admin/specialty', data)
  },
  updateDirection(id: number, data: { name?: string; code?: string; description?: string; sort_order?: number; status?: number }) {
    return unwrappedRequest.put<null>(`/admin/specialty/${id}`, data)
  },
  /** 交换专业方向排序：PUT /api/admin/specialty/:id/sort */
  swapDirection(id: number, swapWith: number) {
    return unwrappedRequest.put<null>(`/admin/specialty/${id}/sort`, { swap_with: swapWith })
  },
  deleteDirection(id: number) {
    return unwrappedRequest.delete<null>(`/admin/specialty/${id}`)
  },

  // ===== 课程等级（后端路由 /admin/level*，等级全局共享无方向维度） =====
  createLevel(data: { name: string; code?: string; description?: string; sort_order?: number; status?: number }) {
    return unwrappedRequest.post<{ level_id: number }>('/admin/level', data)
  },
  updateLevel(id: number, data: { name?: string; code?: string; description?: string; sort_order?: number; status?: number }) {
    return unwrappedRequest.put<null>(`/admin/level/${id}`, data)
  },
  /** 交换课程等级排序：PUT /api/admin/level/:id/sort */
  swapLevel(id: number, swapWith: number) {
    return unwrappedRequest.put<null>(`/admin/level/${id}/sort`, { swap_with: swapWith })
  },
  deleteLevel(id: number) {
    return unwrappedRequest.delete<null>(`/admin/level/${id}`)
  },

  // ===== 证书模板（后端单数路由 certificate-template，有效期单位天） =====
  getCertificateTemplates() {
    return unwrappedRequest.get<{ certificate_templates: CertificateTemplate[] }>('/admin/certificate-templates')
  },
  createCertificateTemplate(data: CertificateTemplatePayload) {
    return unwrappedRequest.post<CertificateTemplate>('/admin/certificate-template', data)
  },
  updateCertificateTemplate(id: number, data: Partial<CertificateTemplatePayload>) {
    return unwrappedRequest.put<CertificateTemplate>(`/admin/certificate-template/${id}`, data)
  },
  deleteCertificateTemplate(id: number) {
    return unwrappedRequest.delete<null>(`/admin/certificate-template/${id}`)
  },

  // ===== 题库标签（后端管理端路由 /admin/question-tag*） =====
  /** 学员端标签列表（公开，仅启用项，question_count=已发布题数）：GET /api/tags */
  getTags() {
    return unwrappedRequest.get<{ tags: QuestionTag[] }>('/tags')
  },
  getQuestionTags() {
    return unwrappedRequest.get<{ tags: QuestionTag[] }>('/admin/question-tags')
  },
  createQuestionTag(data: TagPayload) {
    return unwrappedRequest.post<{ id: number }>('/admin/question-tag', data)
  },
  updateQuestionTag(id: number, data: Partial<TagPayload>) {
    return unwrappedRequest.put<null>(`/admin/question-tag/${id}`, data)
  },
  deleteQuestionTag(id: number) {
    return unwrappedRequest.delete<null>(`/admin/question-tag/${id}`)
  },
  /** 题目打标（管理端）：PUT /api/admin/question/:question_id/tags 全量替换 */
  setQuestionTags(questionId: number, tagIds: number[]) {
    return unwrappedRequest.put<null>(`/admin/question/${questionId}/tags`, { tag_ids: tagIds })
  }
}
