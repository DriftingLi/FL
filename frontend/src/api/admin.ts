// 已迁移模块：走 unwrappedRequest（拦截器解包信封，成功直接返回业务数据 Promise<T>，
// 业务失败抛错并统一 toast，调用方不再自检 res.code）
//
// 响应类型**不再手写**：唯一事实源是后端注解 → backend/docs/swagger.json →
// `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3，issue #965 片七）。
// 本文件只留请求壳、端点装配与**名称适配**。
//
// 入参（query / body）类型**保留手写**并在此集中声明：ADR-0048 决策 3 明确入参不生成
// （swag 对 body 描述弱，本仓多处 body 是 map[string]any）。带 `Query` / `Payload` 后缀的
// interface 都是入参，不是响应形状，勿与下面的生成别名混用。
//
// 别名规则（片一统一口径）：旧名与生成形状确实对应时保留旧名（响应形状一律取生成类型）；
// 旧名对应错形状时删旧名、改出生成名（本片差异逐条记于片七字段级差异清单）。
import { unwrappedRequest } from './request'
// 票 6（ADR-0060 决策 6）：本模块的分页端点在**出口处**把后端的键名（list / tutors / requests /
// items）消解成中立容器 Page<T>——页面因此不再需要知道「导师列表的键叫 tutors」。
import { toPage, type Page } from './page'
import type {
  AIConfigDTO,
  AdminCourseDetailDTO,
  AdminOverviewDTO,
  AdminStatisticsDTO,
  AuditLog,
  AuditLogPageResult,
  ChapterDTO,
  CourseDTO,
  CoursePageResult,
  CourseStatDTO,
  DeleteChapterResult,
  DeleteCourseResult,
  FeatureBindingDTO,
  GenerateContentResultDTO,
  GenTaskStatus,
  HrwaiUserCreatedDTO,
  HrwaiUserPageResult,
  HrwaiUserSummary,
  ProfileChangeRequestDTO,
  ProfileChangeRequestPageResult,
  RecruiterCreatedDTO,
  RecruiterListItem,
  RecruiterListResult,
  RecruiterPasswordResetResult,
  RecruiterUpdatedDTO,
  StatusResultDTO,
  TutorDTO,
  TutorDeletedDTO,
  TutorListDTO,
  TutorRegisterResultDTO
} from './generated/admin'

export type {
  AIConfigDTO as AIConfig,
  AdminCourseDetailDTO as AdminCourseDetail,
  AdminOverviewDTO as AdminStatisticsOverview,
  AdminStatisticsDTO as AdminStatistics,
  AuditLog as AuditLogItem,
  ChapterDTO as AdminChapter,
  CourseDTO as AdminCourseItem,
  CoursePageResult as AdminCoursePageData,
  CourseStatDTO as AdminCourseStat,
  DeleteChapterResult as AdminChapterDeleteResult,
  DeleteCourseResult as AdminCourseDeleteResult,
  FeatureBindingDTO as FeatureBinding,
  GenTaskStatus as GenerateTask,
  HrwaiUserCreatedDTO as HrwaiUserCreated,
  HrwaiUserPageResult as HrwaiUserPageData,
  HrwaiUserSummary as HrwaiUser,
  ProfileChangeRequestDTO as ProfileChangeRequest,
  ProfileChangeRequestPageResult as ProfileReviewPageData,
  RecruiterCreatedDTO as AdminRecruiterCreated,
  RecruiterListItem as AdminRecruiter,
  RecruiterListResult as AdminRecruiterPageData,
  RecruiterPasswordResetResult as AdminRecruiterPasswordResetResult,
  RecruiterUpdatedDTO as AdminRecruiterUpdated,
  StatusResultDTO as AdminStatusResult,
  TutorDTO as AdminTutor,
  TutorDeletedDTO as AdminTutorDeleteResult,
  TutorListDTO as AdminTutorPageData,
  TutorRegisterResultDTO as AdminTutorCreated
}

// ===== 入参（query / body）—— 不生成，ADR-0048 决策 3 =====

export interface AdminHrwaiUsersQuery {
  page?: number
  page_size?: number
  keyword?: string
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

export interface ProfileReviewsQuery {
  status?: string
  page?: number
  page_size?: number
}

export interface AuditLogsQuery {
  page?: number
  page_size?: number
  actor_id?: number
  role?: string
  keyword?: string
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

export interface AdminCoursesQuery {
  page?: number
  page_size?: number
  keyword?: string
  credential_id?: number
  specialty_id?: number
  level_id?: number
  filter?: 'hot' | 'featured' | 'all'
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
  is_hot?: boolean
  is_featured?: boolean
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

export interface AdminRecruitersQuery {
  page?: number
  page_size?: number
  keyword?: string
}

export interface AddRecruiterPayload {
  username: string
  password: string
  company_name: string
  credit_code: string
  business_scope: string
  contact_name: string
  contact_phone: string
  contact_email: string
  wechat?: string
}

export const adminApi = {
  // ===== HRWAI 用户管理(统一) =====
  /** 列表（后端行键 = `list`）。 */
  async getHrwaiUsers(params: AdminHrwaiUsersQuery): Promise<Page<HrwaiUserSummary>> {
    const res = await unwrappedRequest.get<HrwaiUserPageResult>('/admin/hrwai-users', { params })
    return toPage(res?.list, res?.total)
  },

  createHrwaiUser(data: CreateHrwaiUserPayload) {
    return unwrappedRequest.post<HrwaiUserCreatedDTO>('/admin/hrwai-users', data)
  },

  updateHrwaiUser(id: number, data: UpdateHrwaiUserPayload) {
    return unwrappedRequest.put<null>(`/admin/hrwai-users/${id}`, data)
  },

  resetHrwaiUserPassword(id: number, password: string) {
    return unwrappedRequest.put<null>(`/admin/hrwai-users/${id}/password`, { password })
  },

  toggleHrwaiUserStatus(id: number) {
    return unwrappedRequest.put<StatusResultDTO>(`/admin/hrwai-users/${id}/status`)
  },

  deleteHrwaiUser(id: number) {
    return unwrappedRequest.delete<null>(`/admin/hrwai-users/${id}`)
  },

  // ===== 导师管理 =====
  /** 列表（后端行键 = `tutors`）。 */
  async getTutors(params: AdminTutorsQuery): Promise<Page<TutorDTO>> {
    const res = await unwrappedRequest.get<TutorListDTO>('/admin/tutors', { params })
    return toPage(res?.tutors, res?.total)
  },

  addTutor(data: AddTutorPayload) {
    return unwrappedRequest.post<TutorRegisterResultDTO>('/admin/tutor', data)
  },

  deleteTutor(id: number) {
    return unwrappedRequest.delete<TutorDeletedDTO>(`/admin/tutor/${id}`)
  },

  resetTutorPassword(id: number, password: string) {
    return unwrappedRequest.put<null>(`/admin/tutor/${id}/password`, { password })
  },

  toggleTutorStatus(id: number) {
    return unwrappedRequest.put<StatusResultDTO>(`/admin/tutor/${id}/status`)
  },

  // ===== 企业招聘者管理（#416） =====
  /** 列表（后端行键 = `items`）。 */
  async getRecruiters(params: AdminRecruitersQuery): Promise<Page<RecruiterListItem>> {
    const res = await unwrappedRequest.get<RecruiterListResult>('/admin/recruiters', { params })
    return toPage(res?.items, res?.total)
  },

  addRecruiter(data: AddRecruiterPayload) {
    return unwrappedRequest.post<RecruiterCreatedDTO>('/admin/recruiters', data)
  },

  toggleRecruiterStatus(id: number) {
    return unwrappedRequest.put<StatusResultDTO>(`/admin/recruiters/${id}/status`)
  },

  // #417：编辑企业信息与重置密码（响应与错误信息不回显口令字段）
  editRecruiter(id: number, data: Partial<AddRecruiterPayload>) {
    return unwrappedRequest.put<RecruiterUpdatedDTO>(`/admin/recruiters/${id}`, data)
  },

  resetRecruiterPassword(id: number, password: string) {
    return unwrappedRequest.put<RecruiterPasswordResetResult>(`/admin/recruiters/${id}/password`, { password })
  },

  getStatistics() {
    return unwrappedRequest.get<AdminStatisticsDTO>('/admin/statistics')
  },

  generateContent(data: GenerateContentPayload) {
    return unwrappedRequest.post<GenerateContentResultDTO>('/admin/course/generate-content', data)
  },

  getGenerateStatus(taskId: string) {
    return unwrappedRequest.get<GenTaskStatus>(`/admin/course/generate-content/${taskId}`)
  },

  getCourses(params: AdminCoursesQuery) {
    return unwrappedRequest.get<CoursePageResult>('/admin/courses', { params })
  },

  getCourseDetail(id: number) {
    return unwrappedRequest.get<AdminCourseDetailDTO>(`/admin/course/${id}`)
  },

  createCourse(data: CoursePayload) {
    return unwrappedRequest.post<CourseDTO>('/admin/course', data)
  },

  updateCourse(id: number, data: CoursePayload) {
    return unwrappedRequest.put<CourseDTO>(`/admin/course/${id}`, data)
  },
  /** 交换课程排序（同一方向+等级组内）：PUT /api/admin/course/:id/sort */
  swapCourse(id: number, swapWith: number) {
    return unwrappedRequest.put<null>(`/admin/course/${id}/sort`, { swap_with: swapWith })
  },

  deleteCourse(id: number) {
    return unwrappedRequest.delete<DeleteCourseResult>(`/admin/course/${id}`)
  },

  createChapter(courseId: number, data: ChapterPayload) {
    return unwrappedRequest.post<ChapterDTO>(`/admin/course/${courseId}/chapter`, data)
  },

  updateChapter(chapterId: number, data: ChapterPayload) {
    return unwrappedRequest.put<ChapterDTO>(`/admin/chapter/${chapterId}`, data)
  },

  deleteChapter(chapterId: number) {
    return unwrappedRequest.delete<DeleteChapterResult>(`/admin/chapter/${chapterId}`)
  },

  // ===== AI 多配置 =====

  listAIConfigs() {
    return unwrappedRequest.get<AIConfigDTO[]>('/admin/ai-configs')
  },

  createAIConfig(data: CreateAIConfigPayload) {
    return unwrappedRequest.post<null>('/admin/ai-configs', data)
  },

  updateAIConfig(id: number, data: UpdateAIConfigPayload) {
    return unwrappedRequest.put<null>(`/admin/ai-configs/${id}`, data)
  },

  deleteAIConfig(id: number) {
    return unwrappedRequest.delete<null>(`/admin/ai-configs/${id}`)
  },

  testAIConfig(id: number) {
    return unwrappedRequest.post<null>(`/admin/ai-configs/${id}/test`)
  },

  // ===== 功能绑定 =====

  listFeatureBindings() {
    return unwrappedRequest.get<FeatureBindingDTO[]>('/admin/ai-feature-bindings')
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

  /** 列表（后端行键 = `requests`）。 */
  async listProfileReviews(params: ProfileReviewsQuery): Promise<Page<ProfileChangeRequestDTO>> {
    const res = await unwrappedRequest.get<ProfileChangeRequestPageResult>('/admin/profile-reviews', { params })
    return toPage(res?.requests, res?.total)
  },

  approveProfileReview(id: number) {
    return unwrappedRequest.post<ProfileChangeRequestDTO>(`/admin/profile-reviews/${id}/approve`)
  },

  rejectProfileReview(id: number, reason: string) {
    return unwrappedRequest.post<ProfileChangeRequestDTO>(`/admin/profile-reviews/${id}/reject`, { reason })
  },

  // ===== 审计日志 =====

  /** 列表（后端行键 = `items`，生成物标注可为 null）。 */
  async listAuditLogs(params: AuditLogsQuery): Promise<Page<AuditLog>> {
    const res = await unwrappedRequest.get<AuditLogPageResult>('/admin/audit-logs', { params })
    return toPage(res?.items, res?.total)
  }
}
