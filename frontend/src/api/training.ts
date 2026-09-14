// 已迁移模块：走 unwrappedRequest（拦截器解包信封，成功直接返回业务数据 Promise<T>，
// 业务失败抛错并统一 toast，调用方不再自检 res.code）
//
// 响应类型**不再手写**：唯一事实源是后端注解 → backend/docs/swagger.json →
// `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3，issue #964 片六）。
// 入参（body）类型不生成、仍手写（决策 3）。
import { unwrappedRequest } from './request'
import type {
  CatalogLevelNode,
  CatalogSpecialtyNode,
  CatalogTreeDTO,
  CertificateTemplateDict,
  CertificateTemplateListDTO,
  CourseDTO,
  LevelDict,
  LevelListDTO,
  QuestionTagDict,
  QuestionTagListDTO,
  QuestionTagsResultDTO,
  SpecialtyDict
} from './generated/training'

export type {
  CatalogLevelNode,
  CatalogSpecialtyNode,
  CatalogTreeDTO,
  CertificateTemplateDict,
  CertificateTemplateListDTO,
  CourseDTO,
  LevelDict,
  LevelListDTO,
  QuestionTagDict,
  QuestionTagListDTO,
  QuestionTagsResultDTO,
  SpecialtyDict
}

// ===== 培训目录体系：专业方向(specialty) → 等级(level) → 课程 → 章节 =====
// 契约与后端 LH-27 真实路由/字段对齐：
//   学员端公开  /catalog/tree、/levels、/tags
//   管理端      /admin/specialty*、/admin/level*、/admin/certificate-template*、
//              /admin/question-tag*、/admin/question/:id/tags、/admin/catalog/tree

// 旧名保留为生成别名（既有 import 路径不破）；旧手写版把必填字段写成可选，形状以注解为准（清单第 ② 类）。
/** 专业方向 = 生成 SpecialtyDict */
export type CatalogDirection = SpecialtyDict
/** 课程等级（全局共享，不归属方向）= 生成 LevelDict */
export type CatalogLevel = LevelDict
/** 证书模板（有效期单位为天 validity_days）= 生成 CertificateTemplateDict */
export type CertificateTemplate = CertificateTemplateDict
/** 目录树中的课程节点 = 生成 CourseDTO（sort_order 为 CourseDTO 既有字段） */
export type CatalogCourseNode = CourseDTO
/** 方向节点（含等级）= 生成 CatalogSpecialtyNode */
export type CatalogDirectionNode = CatalogSpecialtyNode
/** 完整目录树（公开/管理端均为 {specialties}）= 生成 CatalogTreeDTO */
export type CatalogTree = CatalogTreeDTO
/** 题库标签 = 生成 QuestionTagDict */
export type QuestionTag = QuestionTagDict

/** 标签创建/更新入参（不生成，ADR-0048 决策 3） */
export interface TagPayload {
  name: string
  code?: string
  description?: string
  sort_order?: number
  status?: number
}

/** 证书模板创建/更新入参（不生成，ADR-0048 决策 3） */
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
  /**
   * 公开目录树（学员端筛选用）：GET /api/catalog/tree → {specialties}。
   * **公开路由**（无 JWTAuth）拿不到登录上下文，服务端无法兜底当前证件，故由调用方显式传入
   * （ADR-0047 §4 / #931；与课程列表同口径 #702）。
   */
  getCatalogTree(credentialId?: number | null) {
    return unwrappedRequest.get<CatalogTreeDTO>('/catalog/tree', {
      params: credentialId ? { credential_id: credentialId } : {}
    })
  },
  /** 全局课程等级列表（仅启用项）：GET /api/levels */
  getLevels() {
    return unwrappedRequest.get<LevelListDTO>('/levels')
  },
  /** 管理端目录树（含停用项/章节）：GET /api/admin/catalog/tree → {specialties} */
  getAdminCatalogTree() {
    return unwrappedRequest.get<CatalogTreeDTO>('/admin/catalog/tree')
  },

  // ===== 专业方向（后端路由 /admin/specialty*） =====
  /** 创建返回落库后的字典项（201 Created） */
  createDirection(data: { name: string; code?: string; description?: string; sort_order?: number; status?: number }) {
    return unwrappedRequest.post<SpecialtyDict>('/admin/specialty', data)
  },
  /** 更新返回落库后的字典项（此前被当成无载荷） */
  updateDirection(id: number, data: { name?: string; code?: string; description?: string; sort_order?: number; status?: number }) {
    return unwrappedRequest.put<SpecialtyDict>(`/admin/specialty/${id}`, data)
  },
  /** 交换专业方向排序：PUT /api/admin/specialty/:id/sort */
  swapDirection(id: number, swapWith: number) {
    return unwrappedRequest.put<null>(`/admin/specialty/${id}/sort`, { swap_with: swapWith })
  },
  deleteDirection(id: number) {
    return unwrappedRequest.delete<null>(`/admin/specialty/${id}`)
  },

  // ===== 课程等级（后端路由 /admin/level*，等级全局共享无方向维度） =====
  /** 创建返回落库后的字典项（201 Created） */
  createLevel(data: { name: string; code?: string; description?: string; sort_order?: number; status?: number }) {
    return unwrappedRequest.post<LevelDict>('/admin/level', data)
  },
  /** 更新返回落库后的字典项（此前被当成无载荷） */
  updateLevel(id: number, data: { name?: string; code?: string; description?: string; sort_order?: number; status?: number }) {
    return unwrappedRequest.put<LevelDict>(`/admin/level/${id}`, data)
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
    return unwrappedRequest.get<CertificateTemplateListDTO>('/admin/certificate-templates')
  },
  createCertificateTemplate(data: CertificateTemplatePayload) {
    return unwrappedRequest.post<CertificateTemplateDict>('/admin/certificate-template', data)
  },
  updateCertificateTemplate(id: number, data: Partial<CertificateTemplatePayload>) {
    return unwrappedRequest.put<CertificateTemplateDict>(`/admin/certificate-template/${id}`, data)
  },
  deleteCertificateTemplate(id: number) {
    return unwrappedRequest.delete<null>(`/admin/certificate-template/${id}`)
  },

  // ===== 题库标签（后端管理端路由 /admin/question-tag*） =====
  /**
   * 学员端标签列表（公开，仅启用项，question_count=已发布题数）：GET /api/tags。
   * 同上：公开路由由调用方显式传证件（与抽题池同口径 #702）。
   */
  getTags(credentialId?: number | null) {
    return unwrappedRequest.get<QuestionTagListDTO>('/tags', {
      params: credentialId ? { credential_id: credentialId } : {}
    })
  },
  getQuestionTags() {
    return unwrappedRequest.get<QuestionTagListDTO>('/admin/question-tags')
  },
  createQuestionTag(data: TagPayload) {
    return unwrappedRequest.post<QuestionTagDict>('/admin/question-tag', data)
  },
  /** 更新返回落库后的标签（此前被当成无载荷） */
  updateQuestionTag(id: number, data: Partial<TagPayload>) {
    return unwrappedRequest.put<QuestionTagDict>(`/admin/question-tag/${id}`, data)
  },
  deleteQuestionTag(id: number) {
    return unwrappedRequest.delete<null>(`/admin/question-tag/${id}`)
  },
  /** 题目打标（管理端）：PUT /api/admin/question/:question_id/tags 全量替换，返回写入后的标签ID集合 */
  setQuestionTags(questionId: number, tagIds: number[]) {
    return unwrappedRequest.put<QuestionTagsResultDTO>(`/admin/question/${questionId}/tags`, { tag_ids: tagIds })
  }
}
