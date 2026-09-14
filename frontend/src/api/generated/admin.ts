// 生成文件，勿手改（ADR-0019 契约 codegen 专项 / ADR-0048 按域解冻；spec #940 片五③、#952 片一）。
// 域：管理端（/api/admin/*：HRWAI 用户 / 导师 / 招聘者 / 课程 / 章节 / 统计 / AI 配置 / 资料审核 / 审计 / 数据导出）
// 唯一事实源：后端注解 → backend/docs/swagger.json（CI 有新鲜度锁：backend-lint 的 swagger 步骤）。
// 再生成：cd backend && go run ./cmd/gen-apitypes
// 同步契约：backend/internal/apitypes/codegen_test.go 把本文件与注解渲染结果全等比对。
//
// 覆盖端点：
//   GET  /admin/hrwai-users
//   POST /admin/hrwai-users
//   PUT  /admin/hrwai-users/{id}
//   PUT  /admin/hrwai-users/{id}/password
//   PUT  /admin/hrwai-users/{id}/status
//   DELETE /admin/hrwai-users/{id}
//   GET  /admin/tutors
//   POST /admin/tutor
//   DELETE /admin/tutor/{tutor_id}
//   PUT  /admin/tutor/{tutor_id}/password
//   PUT  /admin/tutor/{tutor_id}/status
//   GET  /admin/recruiters
//   POST /admin/recruiters
//   PUT  /admin/recruiters/{id}/status
//   PUT  /admin/recruiters/{id}
//   PUT  /admin/recruiters/{id}/password
//   GET  /admin/statistics
//   POST /admin/course/generate-content
//   GET  /admin/course/generate-content/{task_id}
//   GET  /admin/courses
//   GET  /admin/course/{course_id}
//   POST /admin/course
//   PUT  /admin/course/{course_id}
//   PUT  /admin/course/{course_id}/sort
//   DELETE /admin/course/{course_id}
//   POST /admin/course/{course_id}/chapter
//   PUT  /admin/chapter/{chapter_id}
//   DELETE /admin/chapter/{chapter_id}
//   GET  /admin/ai-configs
//   POST /admin/ai-configs
//   PUT  /admin/ai-configs/{id}
//   DELETE /admin/ai-configs/{id}
//   POST /admin/ai-configs/{id}/test
//   GET  /admin/ai-feature-bindings
//   PUT  /admin/ai-feature-bindings/{feature_key}
//   DELETE /admin/ai-feature-bindings/{feature_key}/configs/{config_id}
//   GET  /admin/profile-reviews
//   POST /admin/profile-reviews/{id}/approve
//   POST /admin/profile-reviews/{id}/reject
//   GET  /admin/audit-logs
//   GET  /admin/export/students
//   GET  /admin/export/questions
//   GET  /admin/export/evaluations
//
// 覆盖的 Go 类型：AuditLogPageResult / AuditLog / AIConfigDTO / AdminCourseDetailDTO / AdminOverviewDTO / AdminStatisticsDTO / CertificateTemplateDTO / ChapterDTO / ChapterFileDTO / ChapterGenResult / CourseBriefDTO / CourseDTO / CoursePageResult / CourseStatDTO / CredentialBriefDTO / DeleteChapterResult / DeleteCourseResult / FeatureBindingDTO / GenTaskStatus / GenerateContentResultDTO / HrwaiUserCreatedDTO / HrwaiUserPageResult / HrwaiUserSummary / LevelBriefDTO / ProfileChangeRequestDTO / ProfileChangeRequestPageResult / RecruiterCreatedDTO / RecruiterListItem / RecruiterListResult / RecruiterPasswordResetResult / RecruiterUpdatedDTO / SpecialtyBriefDTO / StatusResultDTO / TutorDTO / TutorDeletedDTO / TutorListDTO / TutorRegisterResultDTO
//
// 可空性 / 缺省态由**注解层**表达，生成器只如实转写（Go 结构体 tag）：
//   - extensions:"x-nullable" → 字段渲染 'T | null'：键一定在，值为 null（Go 指针且无 omitempty）；
//   - extensions:"x-optional" → 字段渲染 'T?'：键**可能整个不存在**（Go omitempty）；
//   - 两者可同时标注（'T?' 且 '| null'）；未标注的一律按「键一定在、非 null」渲染 ——
//     swag 看不到 Go 的 omitempty，漏标即契约撒谎。
// 其余已知限制：
//   - Go 侧 any 字段在 swagger 里是空 schema，渲染 'unknown'（不猜结构）；
//   - 不生成 query / body 的入参类型（只生成响应形状）。
// 需要更精确的形状时先在注解层补齐（先例见 spec #940 片五②的差集清单）。

export interface AuditLogPageResult {
  items: AuditLog[] | null
  page: number
  pages: number
  total: number
}

export interface AuditLog {
  action: string
  actor_id: number
  actor_name: string
  actor_role: string
  created_at: string
  detail?: Record<string, unknown>
  id: number
  ip: string
  method: string
  path: string
  request_id: string
  status: number
}

export interface AIConfigDTO {
  api_key: string
  base_url: string
  created_at: string
  description: string
  id: number
  is_active: boolean
  model: string
  name: string
  updated_at: string
}

export interface AdminCourseDetailDTO {
  certificate_name?: string
  certificate_template?: CertificateTemplateDTO
  certificate_template_id: number | null
  chapter_count?: number
  chapters: ChapterDTO[]
  course_id: number
  cover_image: string
  created_at: string
  credential?: CredentialBriefDTO
  credential_id: number | null
  description: string
  duration: number
  is_featured: boolean
  is_hot: boolean
  level?: LevelBriefDTO
  level_id: number | null
  name: string
  points_price?: number
  practice_hours: number
  prerequisite_course_ids?: number[]
  prerequisites?: CourseBriefDTO[]
  sort_order: number
  specialty?: SpecialtyBriefDTO
  specialty_id: number | null
  status: number
  student_count?: number
  theory_hours: number
}

export interface AdminOverviewDTO {
  active_today: number
  total_courses: number
  total_students: number
  total_study_duration: number
}

export interface AdminStatisticsDTO {
  course_stats: CourseStatDTO[]
  overview: AdminOverviewDTO
}

export interface CertificateTemplateDTO {
  code: string
  description: string
  id: number
  name: string
  template_url: string
  validity_days: number
}

export interface ChapterDTO {
  chapter_id: number
  content: string
  content_type: string
  course_id: number
  created_at: string
  description: string
  duration: number
  file_url: string
  files?: ChapterFileDTO[]
  order_num: number
  title: string
}

export interface ChapterFileDTO {
  chapter_id: number | null
  content_type: string
  created_at: string
  file_id: number
  file_name: string
  file_size: number
  file_url: string
}

export interface ChapterGenResult {
  chapter_id: number
  content?: string
  error?: string
  status: string
  title: string
}

export interface CourseBriefDTO {
  course_id: number
  name: string
}

export interface CourseDTO {
  certificate_name?: string
  certificate_template?: CertificateTemplateDTO
  certificate_template_id: number | null
  chapter_count?: number
  chapters?: ChapterDTO[]
  course_id: number
  cover_image: string
  created_at: string
  credential?: CredentialBriefDTO
  credential_id: number | null
  description: string
  duration: number
  is_featured: boolean
  is_hot: boolean
  level?: LevelBriefDTO
  level_id: number | null
  name: string
  points_price?: number
  practice_hours: number
  prerequisite_course_ids?: number[]
  prerequisites?: CourseBriefDTO[]
  sort_order: number
  specialty?: SpecialtyBriefDTO
  specialty_id: number | null
  status: number
  student_count?: number
  theory_hours: number
}

export interface CoursePageResult {
  courses: CourseDTO[]
  page: number
  pages: number
  total: number
}

export interface CourseStatDTO {
  avg_progress: number
  course_id: number
  name: string
  study_count: number
  total_duration: number
}

export interface CredentialBriefDTO {
  category: string
  code: string
  id: number
  level: number | null
  name: string
}

export interface DeleteChapterResult {
  chapter_id: number
}

export interface DeleteCourseResult {
  course_id: number
}

export interface FeatureBindingDTO {
  config_id?: number
  config_name?: string
  feature_key: string
  feature_label: string
}

export interface GenTaskStatus {
  completed: number
  results: ChapterGenResult[] | null
  status: string
  task_id: string
  total: number
}

export interface GenerateContentResultDTO {
  task_id: string
}

export interface HrwaiUserCreatedDTO {
  account: string
  id: number
  phone: string
  uid: string
  username: string
}

export interface HrwaiUserPageResult {
  list: HrwaiUserSummary[]
  page: number
  page_size: number
  total: number
}

export interface HrwaiUserSummary {
  account: string
  company: string
  created_at: string
  email: string
  id: number
  phone: string
  status: number
  uid: string
  username: string
}

export interface LevelBriefDTO {
  code: string
  level_id: number
  name: string
}

export interface ProfileChangeRequestDTO {
  avatar_url: string
  created_at: string
  field_type: string
  id: number
  new_value: string
  old_value: string
  reject_reason: string
  reviewed_at?: string
  reviewed_by?: number
  status: string
  user_id: number
  username: string
}

export interface ProfileChangeRequestPageResult {
  page: number
  pages: number
  requests: ProfileChangeRequestDTO[]
  total: number
}

export interface RecruiterCreatedDTO {
  business_scope: string
  company_name: string
  contact_email: string
  contact_name: string
  contact_phone: string
  credit_code: string
  id: number
  status: number
  username: string
  wechat: string
}

export interface RecruiterListItem {
  business_scope: string
  company_name: string
  contact_email: string
  contact_name: string
  contact_phone: string
  created_at: string
  credit_code: string
  id: number
  status: number
  username: string
  wechat: string
}

export interface RecruiterListResult {
  items: RecruiterListItem[]
  page: number
  total: number
}

export interface RecruiterPasswordResetResult {
}

export interface RecruiterUpdatedDTO {
  business_scope: string
  company_name: string
  contact_email: string
  contact_name: string
  contact_phone: string
  credit_code: string
  id: number
  username: string
  wechat: string
}

export interface SpecialtyBriefDTO {
  code: string
  name: string
  specialty_id: number
}

export interface StatusResultDTO {
  status: number
}

export interface TutorDTO {
  created_at: string
  name: string
  status: number
  tutor_id: number
  username: string
}

export interface TutorDeletedDTO {
  tutor_id: number
}

export interface TutorListDTO {
  page: number
  total: number
  tutors: TutorDTO[]
}

export interface TutorRegisterResultDTO {
  name: string
  tutor_id: number
  username: string
}
