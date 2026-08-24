// 已迁移模块：走 unwrappedRequest（拦截器解包信封，成功直接返回业务数据 Promise<T>，
// 业务失败抛错并统一 toast，调用方不再自检 res.code）
import { unwrappedRequest } from './request'
import type { CourseSummary, CourseChapter, CourseDetail } from './course'

export interface AdminHrwaiUsersQuery {
  page?: number
  page_size?: number
  keyword?: string
}

export interface HrwaiUser {
  id: number
  uid?: string
  account: string
  username: string
  phone: string
  email?: string
  company?: string
  status: number
  created_at: string
}

export interface CreateHrwaiUserPayload {
  phone: string
  password: string
  account?: string
  username?: string
  email?: string
  company?: string
}

export interface UpdateHrwaiUserPayload {
  username: string
  email?: string
  company?: string
  status: number
}

export interface AdminTutorsQuery {
  page?: number
  page_size?: number
  keyword?: string
}

export interface AddTutorPayload {
  username: string
  password: string
  name: string
}

export interface GenerateContentPayload {
  course_id?: number
  chapter_ids?: number[]
}

// ===== 资料审核（昵称/头像） =====

export interface ProfileChangeRequest {
  id: number
  user_id: number
  username: string
  avatar_url: string
  field_type: 'nickname' | 'avatar'
  old_value: string
  new_value: string
  status: 'pending' | 'approved' | 'rejected'
  reject_reason?: string
  reviewed_by?: number
  reviewed_at?: string | null
  created_at: string
}

export interface ProfileReviewsQuery {
  status?: string
  page?: number
  page_size?: number
}

// ===== 审计日志 =====

export interface AuditLogItem {
  id: number
  actor_id: number
  actor_role: string
  actor_name: string
  action: string
  path: string
  method: string
  request_id: string
  ip: string
  status: number
  detail?: unknown
  created_at: string
}

export interface AuditLogsQuery {
  page?: number
  page_size?: number
  actor_id?: number
  role?: string
  keyword?: string
}

// ===== AI 多配置 =====

export interface AIConfig {
  id: number
  name: string
  api_key: string // 脱敏后
  base_url: string
  model: string
  description: string
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface CreateAIConfigPayload {
  name: string
  api_key: string
  base_url: string
  model: string
  description?: string
}

export interface UpdateAIConfigPayload {
  name: string
  api_key?: string // 留空表示不修改
  base_url: string
  model: string
  description?: string
  is_active?: boolean
}

export interface BoundConfig {
  config_id: number
  config_name: string
  model: string
}

export interface FeatureBinding {
  feature_key: string
  feature_label: string
  is_multi: boolean // 是否多绑定功能
  // 单绑定字段
  config_id?: number | null
  config_name?: string
  // 多绑定字段
  bound_configs?: BoundConfig[]
  // 前端临时字段（多绑定功能"待添加"下拉框的选中值）
  _pending_config_id?: number
}

export interface AdminCoursesQuery {
  page?: number
  page_size?: number
  keyword?: string
  credential_id?: number
  specialty_id?: number
  level_id?: number
}

export interface CoursePayload {
  name?: string
  description?: string
  cover_image?: string
  duration?: number
  status?: number
  // ===== 培训目录扩展（LH-27/28，字段与后端 applyCourseTrainingFields 对齐）=====
  credential_id?: number | null
  specialty_id?: number | null
  level_id?: number | null
  theory_hours?: number
  practice_hours?: number
  prerequisite_course_ids?: number[]
  certificate_template_id?: number | null
  sort_order?: number
}

export interface ChapterPayload {
  title: string
  content?: string
  content_type?: string
  file_url?: string
  description?: string
  duration?: number
  order_num?: number
}

// ===== 响应类型 =====

/** 管理员课程列表项（复用 course 模块类型：字段随 CourseSummary 收敛，不再重复声明） */
export type AdminCourseItem = CourseSummary & { sort_order?: number }

/** 管理员课程详情（后端扁平 dict：课程字段 + chapters + 嵌套 specialty/level/certificate_template/prerequisites） */
export interface AdminCourseDetail extends CourseDetail {
  chapters?: CourseChapter[]
}

/** 内容生成任务（轮询状态用） */
export interface GenerateTask {
  task_id: string
  status: string
  total?: number
  completed?: number
  results?: {
    chapter_id: number
    title: string
    status: string
    content?: string
    error?: string
  }[]
}

/** 统计看板概览（与后端 AdminOverviewDTO 对齐） */
export interface AdminStatisticsOverview {
  total_students?: number
  active_today?: number
  total_courses?: number
  total_study_duration?: number
}

/** 课程统计条目（与后端 CourseStatDTO 对齐） */
export interface AdminCourseStat {
  course_id?: number
  name: string
  study_count: number
  total_duration: number
  avg_progress: number
}

/** 管理员统计 */
export interface AdminStatistics {
  overview?: AdminStatisticsOverview
  course_stats?: AdminCourseStat[]
}

/** 导师管理列表项 */
export interface AdminTutor {
  tutor_id: number
  username: string
  name: string
  status: number
  created_at?: string
}

export const adminApi = {
  // ===== HRWAI 用户管理(统一) =====
  getHrwaiUsers(params: AdminHrwaiUsersQuery) {
    return unwrappedRequest.get<{ list: HrwaiUser[]; total: number }>('/admin/hrwai-users', { params })
  },

  createHrwaiUser(data: CreateHrwaiUserPayload) {
    return unwrappedRequest.post<HrwaiUser>('/admin/hrwai-users', data)
  },

  updateHrwaiUser(id: number, data: UpdateHrwaiUserPayload) {
    return unwrappedRequest.put<HrwaiUser>(`/admin/hrwai-users/${id}`, data)
  },

  resetHrwaiUserPassword(id: number, password: string) {
    return unwrappedRequest.put<null>(`/admin/hrwai-users/${id}/password`, { password })
  },

  toggleHrwaiUserStatus(id: number) {
    return unwrappedRequest.put<HrwaiUser>(`/admin/hrwai-users/${id}/status`)
  },

  deleteHrwaiUser(id: number) {
    return unwrappedRequest.delete<null>(`/admin/hrwai-users/${id}`)
  },

  // ===== 导师管理 =====
  getTutors(params: AdminTutorsQuery) {
    return unwrappedRequest.get<{ tutors: AdminTutor[]; total: number }>('/admin/tutors', { params })
  },

  addTutor(data: AddTutorPayload) {
    return unwrappedRequest.post<AdminTutor>('/admin/tutor', data)
  },

  deleteTutor(id: number) {
    return unwrappedRequest.delete<null>(`/admin/tutor/${id}`)
  },

  resetTutorPassword(id: number, password: string) {
    return unwrappedRequest.put<null>(`/admin/tutor/${id}/password`, { password })
  },

  toggleTutorStatus(id: number) {
    return unwrappedRequest.put<AdminTutor>(`/admin/tutor/${id}/status`)
  },

  getStatistics() {
    return unwrappedRequest.get<AdminStatistics>('/admin/statistics')
  },

  generateContent(data: GenerateContentPayload) {
    return unwrappedRequest.post<GenerateTask>('/admin/course/generate-content', data)
  },

  getGenerateStatus(taskId: string) {
    return unwrappedRequest.get<GenerateTask>(`/admin/course/generate-content/${taskId}`)
  },

  getCourses(params: AdminCoursesQuery) {
    return unwrappedRequest.get<{ courses: AdminCourseItem[]; total: number }>('/admin/courses', { params })
  },

  getCourseDetail(id: number) {
    return unwrappedRequest.get<AdminCourseDetail>(`/admin/course/${id}`)
  },

  createCourse(data: CoursePayload) {
    return unwrappedRequest.post<AdminCourseItem>('/admin/course', data)
  },

  updateCourse(id: number, data: CoursePayload) {
    return unwrappedRequest.put<AdminCourseItem>(`/admin/course/${id}`, data)
  },
  /** 交换课程排序（同一方向+等级组内）：PUT /api/admin/course/:id/sort */
  swapCourse(id: number, swapWith: number) {
    return unwrappedRequest.put<null>(`/admin/course/${id}/sort`, { swap_with: swapWith })
  },

  deleteCourse(id: number) {
    return unwrappedRequest.delete<null>(`/admin/course/${id}`)
  },

  createChapter(courseId: number, data: ChapterPayload) {
    return unwrappedRequest.post<{ chapter_id: number }>(`/admin/course/${courseId}/chapter`, data)
  },

  updateChapter(chapterId: number, data: ChapterPayload) {
    return unwrappedRequest.put<{ chapter_id: number }>(`/admin/chapter/${chapterId}`, data)
  },

  deleteChapter(chapterId: number) {
    return unwrappedRequest.delete<null>(`/admin/chapter/${chapterId}`)
  },

  // ===== AI 多配置 =====

  listAIConfigs() {
    return unwrappedRequest.get<AIConfig[]>('/admin/ai-configs')
  },

  createAIConfig(data: CreateAIConfigPayload) {
    return unwrappedRequest.post<AIConfig>('/admin/ai-configs', data)
  },

  updateAIConfig(id: number, data: UpdateAIConfigPayload) {
    return unwrappedRequest.put<AIConfig>(`/admin/ai-configs/${id}`, data)
  },

  deleteAIConfig(id: number) {
    return unwrappedRequest.delete<null>(`/admin/ai-configs/${id}`)
  },

  testAIConfig(id: number) {
    return unwrappedRequest.post<{ ok?: boolean; message?: string }>(`/admin/ai-configs/${id}/test`)
  },

  // ===== 功能绑定 =====

  listFeatureBindings() {
    return unwrappedRequest.get<FeatureBinding[]>('/admin/ai-feature-bindings')
  },

  setFeatureBinding(featureKey: string, configId: number) {
    return unwrappedRequest.put<null>(`/admin/ai-feature-bindings/${featureKey}`, { config_id: configId })
  },

  // 解除多绑定功能的单个配置绑定
  unbindFeatureConfig(featureKey: string, configId: number) {
    return unwrappedRequest.delete<null>(`/admin/ai-feature-bindings/${featureKey}/configs/${configId}`)
  },

  // ===== 资料审核 =====
  // 该组端点仅资料审核页使用

  listProfileReviews(params: ProfileReviewsQuery) {
    return unwrappedRequest.get<{ requests: ProfileChangeRequest[]; total: number }>('/admin/profile-reviews', { params })
  },

  approveProfileReview(id: number) {
    return unwrappedRequest.post<null>(`/admin/profile-reviews/${id}/approve`)
  },

  rejectProfileReview(id: number, reason: string) {
    return unwrappedRequest.post<null>(`/admin/profile-reviews/${id}/reject`, { reason })
  },

  // ===== 审计日志 =====

  listAuditLogs(params: AuditLogsQuery) {
    return unwrappedRequest.get<{ items: AuditLogItem[]; total: number }>('/admin/audit-logs', { params })
  }
}
