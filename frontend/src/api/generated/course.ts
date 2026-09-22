// 生成文件，勿手改（ADR-0019 契约 codegen 专项 / ADR-0048 按域解冻；spec #940 片五③、#952 片一）。
// 域：课程与章节学习（/api/courses、/api/course/*、/api/chapter/*）
// 唯一事实源：后端注解 → backend/docs/swagger.json（CI 有新鲜度锁：backend-lint 的 swagger 步骤）。
// 再生成：cd backend && go run ./cmd/gen-apitypes
// 同步契约：backend/internal/apitypes/codegen_test.go 把本文件与注解渲染结果全等比对。
//
// 覆盖端点：
//   GET  /courses
//   GET  /course/{course_id}
//   POST /course/{course_id}/progress
//   GET  /course/{course_id}/chapter/{chapter_id}
//   GET  /chapter/{chapter_id}/slides
//   POST /chapter/{chapter_id}/slides/regenerate
//
// 覆盖的 Go 类型：CertificateTemplateDTO / ChapterDTO / ChapterDetailDTO / ChapterFileDTO / ChapterSlidesDTO / CourseBriefDTO / CourseDTO / CourseDetailDTO / CoursePageResult / CredentialBriefDTO / LevelBriefDTO / SpecialtyBriefDTO / StudyProgressDTO
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

export interface ChapterDetailDTO {
  chapter_id: number
  content: string
  content_type: string
  course_id: number
  created_at: string
  description: string
  duration: number
  file_url: string
  files: ChapterFileDTO[]
  next_chapter_id: number | null
  order_num: number
  previous_chapter_id: number | null
  resume_position: number
  study_status?: string
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

export interface ChapterSlidesDTO {
  chapter_id: number
  slides: string[]
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
  entitled?: boolean
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

export interface CourseDetailDTO {
  chapters: ChapterDTO[]
  completed_chapters: number
  course_info: CourseDTO
  is_enrolled: boolean
  last_chapter_id: number | null
  last_position: number
  last_studied_at: string
  progress: number
}

export interface CoursePageResult {
  courses: CourseDTO[]
  page: number
  pages: number
  total: number
}

export interface CredentialBriefDTO {
  category: string
  code: string
  id: number
  level: number | null
  name: string
}

export interface LevelBriefDTO {
  code: string
  level_id: number
  name: string
}

export interface SpecialtyBriefDTO {
  code: string
  name: string
  specialty_id: number
}

export interface StudyProgressDTO {
  completed_chapters: number
  progress: number
  record_id: number
  study_duration: number
}
