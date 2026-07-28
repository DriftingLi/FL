import request from './request'

export interface AdminStudentsQuery {
  page?: number
  page_size?: number
  keyword?: string
}

export interface AddStudentPayload {
  phone: string
  password: string
  name: string
  email?: string
  company?: string
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

export interface FeatureBinding {
  feature_key: string
  feature_label: string
  config_id: number | null
  config_name: string
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
  getStudents(params: AdminStudentsQuery) {
    return request.get('/admin/students', { params })
  },

  addStudent(data: AddStudentPayload) {
    return request.post('/admin/student', data)
  },

  deleteStudent(id: number) {
    return request.delete(`/admin/student/${id}`)
  },

  resetStudentPassword(id: number, password: string) {
    return request.put(`/admin/student/${id}/password`, { password })
  },

  toggleStudentStatus(id: number) {
    return request.put(`/admin/student/${id}/status`)
  },

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
  }
}
