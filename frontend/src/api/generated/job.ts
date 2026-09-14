// 生成文件，勿手改（ADR-0019 契约 codegen 专项 / ADR-0048 按域解冻；spec #940 片五③、#952 片一）。
// 域：职位与投递（/api/recruit/jobs*、/api/jobs*、/api/resume/applications*、/api/recruit/applications*）
// 唯一事实源：后端注解 → backend/docs/swagger.json（CI 有新鲜度锁：backend-lint 的 swagger 步骤）。
// 再生成：cd backend && go run ./cmd/gen-apitypes
// 同步契约：backend/internal/apitypes/codegen_test.go 把本文件与注解渲染结果全等比对。
//
// 覆盖端点：
//   POST /recruit/jobs
//   PUT  /recruit/jobs/{id}
//   POST /recruit/jobs/{id}/toggle-status
//   GET  /recruit/jobs
//   GET  /recruit/jobs/{id}
//   GET  /jobs
//   GET  /jobs/{id}
//   POST /jobs/{id}/apply
//   POST /jobs/{id}/report
//   GET  /resume/applications
//   POST /resume/applications/{id}/withdraw
//   GET  /recruit/jobs/{id}/applications
//   GET  /recruit/applications/{id}
//   POST /recruit/applications/{id}/reject
//
// 覆盖的 Go 类型：ApplicationDTO / ApplicationListResult / JobListResult / JobPostingDTO / RecruiterApplicationListResult / ReportDTO
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

export interface ApplicationDTO {
  company_name?: string
  created_at: string
  employer_viewed_at?: string
  id: number
  job_posting_id: number
  job_title?: string
  recruiter_id: number
  resume_updated_at: string
  resume_updated_at_snapshot?: string
  status: string
  student_real_name_masked?: string
  student_resume_updated_at?: string
  student_user_id: number
  updated_at: string
}

export interface ApplicationListResult {
  items: ApplicationDTO[]
  page: number
  page_size: number
  total: number
}

export interface JobListResult {
  items: JobPostingDTO[]
  total: number
}

export interface JobPostingDTO {
  apply_state?: string
  business_scope?: string
  company_name?: string
  contact_name?: string
  cooldown_days?: number
  created_at: string
  description: string
  experience_req: string
  forced_offline: boolean
  id: number
  offline_reason?: string
  position_id?: number
  position_name?: string
  published_at: string
  recruiter_id: number
  region: string
  salary_max?: number
  salary_min?: number
  salary_text: string
  status: string
  title: string
  updated_at: string
}

export interface RecruiterApplicationListResult {
  items: ApplicationDTO[]
  job_title: string
  page: number
  page_size: number
  total: number
  unread_count: number
}

export interface ReportDTO {
  created_at: string
  handled_at?: string
  id: number
  job_posting_id: number
  job_title?: string
  reason: string
  status: string
  student_user_id: number
}
