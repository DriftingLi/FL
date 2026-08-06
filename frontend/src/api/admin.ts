import request from './request'

export interface AdminHrwaiUsersQuery {
  page?: number
  page_size?: number
  keyword?: string
}

export interface HrwaiUser {
  id: number
  username: string
  name: string
  nickname?: string
  phone: string
  email?: string
  company?: string
  status: number
  created_at: string
}

export interface CreateHrwaiUserPayload {
  phone: string
  password: string
  name: string
  email?: string
  company?: string
}

export interface UpdateHrwaiUserPayload {
  name: string
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
  name: string
  nickname: string
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

// ===== 论坛管理 =====

export interface AdminForumTopic {
  id: number
  chapter_id?: number | null
  chapter_title?: string
  title: string
  content: string
  images?: string[]
  view_count: number
  reply_count: number
  last_reply_at?: string | null
  created_at: string
  author: {
    user_id: number
    username: string
    name: string
    nickname: string
    avatar_url: string
  }
}

export interface AdminForumReply {
  id: number
  topic_id: number
  parent_id?: number | null
  parent_name?: string
  content: string
  images?: string[]
  created_at: string
  author: {
    user_id: number
    username: string
    name: string
    nickname: string
    avatar_url: string
  }
}

export interface AdminForumListParams {
  scope?: 'all' | 'general' | 'chapter'
  chapter_id?: number
  page?: number
  page_size?: number
  keyword?: string
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
  detail?: any
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
  category?: string
  specialty_id?: number
  level_id?: number
}

export interface CoursePayload {
  name: string
  category?: string
  description?: string
  cover_image?: string
  duration?: number
  status?: number
  // ===== 培训目录扩展（LH-27/28，字段与后端 applyCourseTrainingFields 对齐）=====
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
  content_url?: string
  file_url?: string
  description?: string
  duration?: number
  order_num?: number
}

// ===== 响应类型 =====

/** 管理员课程列表项 */
export interface AdminCourseItem {
  course_id: number
  name: string
  category?: string
  cover_image?: string
  description?: string
  duration?: number
  status?: number
  chapter_count?: number
  created_at?: string
  // ===== 培训目录扩展（LH-27/28）=====
  specialty_id?: number | null
  level_id?: number | null
  theory_hours?: number
  practice_hours?: number
  prerequisite_course_ids?: number[]
  certificate_template_id?: number | null
  sort_order?: number
  [key: string]: unknown
}

/** 管理员课程详情（含章节） */
export interface AdminChapter {
  chapter_id: number
  title: string
  content?: string
  content_type?: string
  content_url?: string
  file_url?: string
  description?: string
  duration?: number
  order_num?: number
  [key: string]: unknown
}

/** 管理员课程详情（后端扁平 dict：课程字段 + chapters + 嵌套 specialty/level/certificate_template/prerequisites） */
export interface AdminCourseDetail {
  course_id?: number
  name?: string
  category?: string
  description?: string
  cover_image?: string
  duration?: number
  status?: number
  specialty_id?: number | null
  level_id?: number | null
  theory_hours?: number
  practice_hours?: number
  certificate_template_id?: number | null
  chapters?: AdminChapter[]
  specialty?: { specialty_id: number; code?: string; name: string }
  level?: { level_id: number; code?: string; name: string }
  certificate_template?: {
    id: number
    code?: string
    name: string
    description?: string
    validity_days?: number
    template_url?: string
  }
  prerequisites?: { course_id: number; name: string }[]
  [key: string]: unknown
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
  [key: string]: unknown
}

/** 管理员统计 */
export interface AdminStatistics {
  overview?: Record<string, unknown>
  course_stats?: { name: string; study_count: number; total_duration: number; avg_progress: number }[]
  [key: string]: unknown
}

/** 导师管理列表项 */
export interface AdminTutor {
  tutor_id: number
  username: string
  name: string
  status: number
  created_at?: string
  [key: string]: unknown
}

export const adminApi = {
  // ===== HRWAI 用户管理(统一) =====
  getHrwaiUsers(params: AdminHrwaiUsersQuery) {
    return request.get<{ list: HrwaiUser[]; total: number }>('/admin/hrwai-users', { params })
  },

  createHrwaiUser(data: CreateHrwaiUserPayload) {
    return request.post<HrwaiUser>('/admin/hrwai-users', data)
  },

  updateHrwaiUser(id: number, data: UpdateHrwaiUserPayload) {
    return request.put<HrwaiUser>(`/admin/hrwai-users/${id}`, data)
  },

  resetHrwaiUserPassword(id: number, password: string) {
    return request.put<null>(`/admin/hrwai-users/${id}/password`, { password })
  },

  toggleHrwaiUserStatus(id: number) {
    return request.put<HrwaiUser>(`/admin/hrwai-users/${id}/status`)
  },

  deleteHrwaiUser(id: number) {
    return request.delete<null>(`/admin/hrwai-users/${id}`)
  },

  // ===== 导师管理 =====
  getTutors(params: AdminTutorsQuery) {
    return request.get<{ tutors: AdminTutor[]; total: number }>('/admin/tutors', { params })
  },

  addTutor(data: AddTutorPayload) {
    return request.post<AdminTutor>('/admin/tutor', data)
  },

  deleteTutor(id: number) {
    return request.delete<null>(`/admin/tutor/${id}`)
  },

  resetTutorPassword(id: number, password: string) {
    return request.put<null>(`/admin/tutor/${id}/password`, { password })
  },

  toggleTutorStatus(id: number) {
    return request.put<AdminTutor>(`/admin/tutor/${id}/status`)
  },

  getStatistics() {
    return request.get<AdminStatistics>('/admin/statistics')
  },

  generateContent(data: GenerateContentPayload) {
    return request.post<GenerateTask>('/admin/course/generate-content', data)
  },

  getGenerateStatus(taskId: string) {
    return request.get<GenerateTask>(`/admin/course/generate-content/${taskId}`)
  },

  getCourses(params: AdminCoursesQuery) {
    return request.get<{ courses: AdminCourseItem[]; total: number }>('/admin/courses', { params })
  },

  getCourseDetail(id: number) {
    return request.get<AdminCourseDetail>(`/admin/course/${id}`)
  },

  createCourse(data: CoursePayload) {
    return request.post<AdminCourseItem>('/admin/course', data)
  },

  updateCourse(id: number, data: CoursePayload) {
    return request.put<AdminCourseItem>(`/admin/course/${id}`, data)
  },

  deleteCourse(id: number) {
    return request.delete<null>(`/admin/course/${id}`)
  },

  createChapter(courseId: number, data: ChapterPayload) {
    return request.post<{ chapter_id: number }>(`/admin/course/${courseId}/chapter`, data)
  },

  updateChapter(chapterId: number, data: ChapterPayload) {
    return request.put<{ chapter_id: number }>(`/admin/chapter/${chapterId}`, data)
  },

  deleteChapter(chapterId: number) {
    return request.delete<null>(`/admin/chapter/${chapterId}`)
  },

  // ===== AI 多配置 =====

  listAIConfigs() {
    return request.get<AIConfig[]>('/admin/ai-configs')
  },

  createAIConfig(data: CreateAIConfigPayload) {
    return request.post<AIConfig>('/admin/ai-configs', data)
  },

  updateAIConfig(id: number, data: UpdateAIConfigPayload) {
    return request.put<AIConfig>(`/admin/ai-configs/${id}`, data)
  },

  deleteAIConfig(id: number) {
    return request.delete<null>(`/admin/ai-configs/${id}`)
  },

  testAIConfig(id: number) {
    return request.post<{ ok?: boolean; message?: string }>(`/admin/ai-configs/${id}/test`)
  },

  // ===== 功能绑定 =====

  listFeatureBindings() {
    return request.get<FeatureBinding[]>('/admin/ai-feature-bindings')
  },

  setFeatureBinding(featureKey: string, configId: number) {
    return request.put<null>(`/admin/ai-feature-bindings/${featureKey}`, { config_id: configId })
  },

  // 解除多绑定功能的单个配置绑定
  unbindFeatureConfig(featureKey: string, configId: number) {
    return request.delete<null>(`/admin/ai-feature-bindings/${featureKey}/configs/${configId}`)
  },

  // ===== 资料审核 =====

  listProfileReviews(params: ProfileReviewsQuery) {
    return request.get<{ requests: ProfileChangeRequest[]; total: number }>('/admin/profile-reviews', { params })
  },

  approveProfileReview(id: number) {
    return request.post<null>(`/admin/profile-reviews/${id}/approve`)
  },

  rejectProfileReview(id: number, reason: string) {
    return request.post<null>(`/admin/profile-reviews/${id}/reject`, { reason })
  },

  // ===== 论坛管理 =====

  listAdminForumTopics(params: AdminForumListParams) {
    return request.get<{ topics: AdminForumTopic[]; total: number }>('/admin/forum/topics', { params })
  },

  getAdminForumTopic(id: number) {
    return request.get<{ topic?: AdminForumTopic; replies?: AdminForumReply[] }>(`/admin/forum/topics/${id}`)
  },

  deleteAdminForumTopic(id: number) {
    return request.delete<null>(`/admin/forum/topics/${id}`)
  },

  deleteAdminForumReply(id: number) {
    return request.delete<null>(`/admin/forum/replies/${id}`)
  },

  // ===== 审计日志 =====

  listAuditLogs(params: AuditLogsQuery) {
    return request.get<{ items: AuditLogItem[]; total: number }>('/admin/audit-logs', { params })
  }
}
