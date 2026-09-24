// 生成文件，勿手改（ADR-0019 契约 codegen 专项 / ADR-0048 按域解冻；spec #940 片五③、#952 片一）。
// 域：学员简历卡（/api/resume/*：简历 CRUD / 可见性 / PDF 与工作照附件 / 查看留痕 / 收到的联系方式申请）
// 唯一事实源：后端注解 → backend/docs/swagger.json（CI 有新鲜度锁：backend-lint 的 swagger 步骤）。
// 再生成：cd backend && go run ./cmd/gen-apitypes
// 同步契约：backend/internal/apitypes/codegen_test.go 把本文件与注解渲染结果全等比对。
//
// 覆盖端点：
//   GET  /resume
//   PUT  /resume
//   PUT  /resume/visibility
//   POST /resume/pdf
//   DELETE /resume/pdf
//   POST /resume/image
//   GET  /resume/view-stats
//   GET  /resume/contact-requests
//   POST /resume/contact-requests/{id}/approve
//   POST /resume/contact-requests/{id}/reject
//   POST /resume/contact-requests/{id}/revoke
//   GET  /resume/pdf
//
// 覆盖的 Go 类型：ContactRequestDTO / ContactRequestListResult / JobCardDTO
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

export interface ContactRequestDTO {
  company_disabled?: boolean
  company_name?: string
  contact_email?: string
  contact_name?: string
  contact_phone?: string
  created_at: string
  decided_at?: string
  expires_at?: string
  id: number
  message: string
  recruiter_id: number
  source?: string
  status: string
  student_user_id: number
  updated_at: string
  wechat?: string
}

export interface ContactRequestListResult {
  items: ContactRequestDTO[]
  page: number
  page_size: number
  total: number
}

export interface JobCardDTO {
  available_in: string
  contact_phone: string
  created_at: string
  expected_position_extra: string
  expected_position_id?: number
  expected_regions: string[] | null
  experience_years: number
  job_nature: string
  photos: string[] | null
  real_name: string
  region: string
  resume_certifications: Record<string, unknown>[] | null
  resume_experiences: Record<string, unknown>[] | null
  resume_file_url: string
  salary_max?: number
  salary_min?: number
  salary_negotiable: boolean
  self_intro: string
  updated_at: string
  user_id: number
  visibility: string
  wechat: string
}
