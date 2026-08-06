import request from './request'

// ===== 培训目录体系：专业方向 → 等级 → 课程 → 章节 =====
// 契约与后端 LH-27 对齐：接口不一致时以评论反馈为准

/** 专业方向 */
export interface CatalogDirection {
  direction_id: number
  name: string
  code?: string
  sort_order?: number
  [key: string]: unknown
}

/** 课程等级 */
export interface CatalogLevel {
  level_id: number
  name: string
  code?: string
  sort_order?: number
  [key: string]: unknown
}

/** 证书模板 */
export interface CertificateTemplate {
  template_id: number
  name: string
  code?: string
  description?: string
  valid_months?: number
  [key: string]: unknown
}

/** 目录树中的课程节点 */
export interface CatalogCourseNode {
  course_id: number
  name: string
  status?: number
  chapter_count?: number
  sort_order?: number
  theory_hours?: number
  practice_hours?: number
  prerequisite_course_ids?: number[]
  certificate_template_id?: number | null
  certificate_valid_months?: number | null
  description?: string
  chapters?: CatalogChapterNode[]
  [key: string]: unknown
}

/** 目录树中的章节节点 */
export interface CatalogChapterNode {
  chapter_id: number
  title: string
  order_num?: number
  duration?: number
  [key: string]: unknown
}

/** 等级节点（含课程） */
export interface CatalogLevelNode extends CatalogLevel {
  courses?: CatalogCourseNode[]
}

/** 方向节点（含等级） */
export interface CatalogDirectionNode extends CatalogDirection {
  levels?: CatalogLevelNode[]
}

/** 完整目录树（公开/管理端通用，管理端多带章节） */
export interface CatalogTree {
  directions: CatalogDirectionNode[]
}

/** 课程扩展信息（等级/学时/前置/证书，叠加在 CourseSummary 上） */
export interface CourseTrainingInfo {
  direction_id?: number
  direction_name?: string
  level_id?: number
  level_name?: string
  theory_hours?: number
  practice_hours?: number
  prerequisite_course_ids?: number[]
  prerequisite_courses?: { course_id: number; name: string }[]
  certificate_template_id?: number
  certificate_template_name?: string
  certificate_valid_months?: number
  sort_order?: number
  [key: string]: unknown
}

/** 题库标签 */
export interface QuestionTag {
  tag_id: number
  name: string
  question_count?: number
  [key: string]: unknown
}

export interface TagPayload {
  name: string
}

export interface CertificateTemplatePayload {
  name: string
  code?: string
  description?: string
  valid_months?: number | null
}

export const trainingApi = {
  // ===== 目录树 =====
  /** 公开目录树（学员端筛选用） */
  getCatalogTree() {
    return request.get<CatalogTree>('/catalog/tree')
  },
  /** 管理端目录树（含章节） */
  getAdminCatalogTree() {
    return request.get<CatalogTree>('/admin/catalog/tree')
  },

  // ===== 专业方向 =====
  createDirection(data: { name: string; code?: string; sort_order?: number }) {
    return request.post<{ direction_id: number }>('/admin/catalog/directions', data)
  },
  updateDirection(id: number, data: { name?: string; code?: string; sort_order?: number }) {
    return request.put<null>(`/admin/catalog/directions/${id}`, data)
  },
  deleteDirection(id: number) {
    return request.delete<null>(`/admin/catalog/directions/${id}`)
  },

  // ===== 课程等级 =====
  createLevel(data: { name: string; code?: string; sort_order?: number; direction_id?: number }) {
    return request.post<{ level_id: number }>('/admin/catalog/levels', data)
  },
  updateLevel(id: number, data: { name?: string; code?: string; sort_order?: number }) {
    return request.put<null>(`/admin/catalog/levels/${id}`, data)
  },
  deleteLevel(id: number) {
    return request.delete<null>(`/admin/catalog/levels/${id}`)
  },

  // ===== 证书模板 =====
  getCertificateTemplates() {
    return request.get<{ templates: CertificateTemplate[] }>('/admin/certificate-templates')
  },
  createCertificateTemplate(data: CertificateTemplatePayload) {
    return request.post<CertificateTemplate>('/admin/certificate-templates', data)
  },
  updateCertificateTemplate(id: number, data: Partial<CertificateTemplatePayload>) {
    return request.put<CertificateTemplate>(`/admin/certificate-templates/${id}`, data)
  },
  deleteCertificateTemplate(id: number) {
    return request.delete<null>(`/admin/certificate-templates/${id}`)
  },

  // ===== 题库标签 =====
  getQuestionTags() {
    return request.get<QuestionTag[]>('/question-bank/tags')
  },
  createQuestionTag(data: TagPayload) {
    return request.post<{ tag_id: number }>('/question-bank/tags', data)
  },
  updateQuestionTag(id: number, data: Partial<TagPayload>) {
    return request.put<null>(`/question-bank/tags/${id}`, data)
  },
  deleteQuestionTag(id: number) {
    return request.delete<null>(`/question-bank/tags/${id}`)
  },
  /** 题目打标（管理端/导师） */
  setQuestionTags(questionId: number, tagIds: number[]) {
    return request.put<null>(`/question-bank/questions/${questionId}/tags`, { tag_ids: tagIds })
  }
}
