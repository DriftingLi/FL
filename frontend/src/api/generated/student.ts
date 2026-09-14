// 生成文件，勿手改（ADR-0019 契约 codegen 专项 / ADR-0048 按域解冻；spec #940 片五③、#952 片一）。
// 域：学员学习中心（/api/student/*：档案 / 学习记录 / 按天统计 / 我的课程）
// 唯一事实源：后端注解 → backend/docs/swagger.json（CI 有新鲜度锁：backend-lint 的 swagger 步骤）。
// 再生成：cd backend && go run ./cmd/gen-apitypes
// 同步契约：backend/internal/apitypes/codegen_test.go 把本文件与注解渲染结果全等比对。
//
// 覆盖端点：
//   GET  /student/profile
//   GET  /student/records
//   GET  /student/study-stats
//   GET  /student/courses
//   GET  /student/courses/{course_id}
//
// 覆盖的 Go 类型：CourseProgressDTO / StudentCourseChapterDTO / StudentCourseDTO / StudentCourseDetailDTO / StudentCoursesDTO / StudentDTO / StudentProfileDTO / StudyDailyStatsDTO / StudyRecordDTO / StudyRecordPageResult / StudyStatsDTO
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

export interface CourseProgressDTO {
  course_id: number
  course_name: string
  progress: number
  study_date: string
  study_duration: number
  total_chapters: number
}

export interface StudentCourseChapterDTO {
  chapter_id: number
  completed: boolean
  progress: number
  title: string
  video_position: number
}

export interface StudentCourseDTO {
  completed_chapters: number
  course_id: number
  course_name: string
  cover: string
  last_chapter_id: number | null
  last_chapter_title: string
  last_position: number
  last_studied_at: string
  level_id: number | null
  progress: number
  specialty_id: number | null
  study_duration: number
  total_chapters: number
}

export interface StudentCourseDetailDTO {
  chapters: StudentCourseChapterDTO[]
  completed_chapters: number
  course_id: number
  course_name: string
  cover: string
  last_chapter_id: number | null
  last_chapter_title: string
  last_position: number
  last_studied_at: string
  level_id: number | null
  progress: number
  specialty_id: number | null
  study_duration: number
  total_chapters: number
}

export interface StudentCoursesDTO {
  continue_learning: StudentCourseDTO | null
  courses: StudentCourseDTO[]
}

export interface StudentDTO {
  account: string
  avatar_url: string
  created_at: string
  status: number
  student_id: number
  uid: string
  username: string
}

export interface StudentProfileDTO {
  course_progress: CourseProgressDTO[]
  student_info: StudentDTO
  study_stats: StudyStatsDTO
}

export interface StudyDailyStatsDTO {
  active_days: number
  data: number[]
  days: number
  labels: string[]
  total_minutes: number
}

export interface StudyRecordDTO {
  chapter_id: number | null
  chapter_title: string | null
  course_id: number
  course_name: string
  progress: number
  record_id: number
  student_id: number
  study_date: string
  study_duration: number
}

export interface StudyRecordPageResult {
  page: number
  pages: number
  records: StudyRecordDTO[]
  total: number
}

export interface StudyStatsDTO {
  completed_courses: number
  latest_study_time: string
  learning_courses: number
  total_courses: number
  total_study_duration: number
}
