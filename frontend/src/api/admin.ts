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
}

export interface CoursePayload {
  name: string
  category?: string
  description?: string
  cover_image?: string
  duration?: number
  status?: number
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

export const adminApi = {
  // ===== HRWAI 用户管理(统一) =====
  getHrwaiUsers(params: AdminHrwaiUsersQuery) {
    return request.get('/admin/hrwai-users', { params })
  },

  createHrwaiUser(data: CreateHrwaiUserPayload) {
    return request.post('/admin/hrwai-users', data)
  },

  updateHrwaiUser(id: number, data: UpdateHrwaiUserPayload) {
    return request.put(`/admin/hrwai-users/${id}`, data)
  },

  resetHrwaiUserPassword(id: number, password: string) {
    return request.put(`/admin/hrwai-users/${id}/password`, { password })
  },

  toggleHrwaiUserStatus(id: number) {
    return request.put(`/admin/hrwai-users/${id}/status`)
  },

  deleteHrwaiUser(id: number) {
    return request.delete(`/admin/hrwai-users/${id}`)
  },

  // ===== 导师管理 =====
  getTutors(params: AdminTutorsQuery) {
    return request.get('/admin/tutors', { params })
  },

  addTutor(data: AddTutorPayload) {
    return request.post('/admin/tutor', data)
  },

  deleteTutor(id: number) {
    return request.delete(`/admin/tutor/${id}`)
  },

  resetTutorPassword(id: number, password: string) {
    return request.put(`/admin/tutor/${id}/password`, { password })
  },

  toggleTutorStatus(id: number) {
    return request.put(`/admin/tutor/${id}/status`)
  },

  getStatistics() {
    return request.get('/admin/statistics')
  },

  generateContent(data: GenerateContentPayload) {
    return request.post('/admin/course/generate-content', data)
  },

  getGenerateStatus(taskId: string) {
    return request.get(`/admin/course/generate-content/${taskId}`)
  },

  getCourses(params: AdminCoursesQuery) {
    return request.get('/admin/courses', { params })
  },

  getCourseDetail(id: number) {
    return request.get(`/admin/course/${id}`)
  },

  createCourse(data: CoursePayload) {
    return request.post('/admin/course', data)
  },

  updateCourse(id: number, data: CoursePayload) {
    return request.put(`/admin/course/${id}`, data)
  },

  deleteCourse(id: number) {
    return request.delete(`/admin/course/${id}`)
  },

  createChapter(courseId: number, data: ChapterPayload) {
    return request.post(`/admin/course/${courseId}/chapter`, data)
  },

  updateChapter(chapterId: number, data: ChapterPayload) {
    return request.put(`/admin/chapter/${chapterId}`, data)
  },

  deleteChapter(chapterId: number) {
    return request.delete(`/admin/chapter/${chapterId}`)
  },

  // ===== AI 多配置 =====

  listAIConfigs() {
    return request.get('/admin/ai-configs')
  },

  createAIConfig(data: CreateAIConfigPayload) {
    return request.post('/admin/ai-configs', data)
  },

  updateAIConfig(id: number, data: UpdateAIConfigPayload) {
    return request.put(`/admin/ai-configs/${id}`, data)
  },

  deleteAIConfig(id: number) {
    return request.delete(`/admin/ai-configs/${id}`)
  },

  testAIConfig(id: number) {
    return request.post(`/admin/ai-configs/${id}/test`)
  },

  // ===== 功能绑定 =====

  listFeatureBindings() {
    return request.get('/admin/ai-feature-bindings')
  },

  setFeatureBinding(featureKey: string, configId: number) {
    return request.put(`/admin/ai-feature-bindings/${featureKey}`, { config_id: configId })
  },

  // 解除多绑定功能的单个配置绑定
  unbindFeatureConfig(featureKey: string, configId: number) {
    return request.delete(`/admin/ai-feature-bindings/${featureKey}/configs/${configId}`)
  },

  // ===== 资料审核 =====

  listProfileReviews(params: ProfileReviewsQuery) {
    return request.get('/admin/profile-reviews', { params })
  },

  approveProfileReview(id: number) {
    return request.post(`/admin/profile-reviews/${id}/approve`)
  },

  rejectProfileReview(id: number, reason: string) {
    return request.post(`/admin/profile-reviews/${id}/reject`, { reason })
  }
}
