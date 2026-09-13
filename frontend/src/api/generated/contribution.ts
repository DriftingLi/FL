// 生成文件，勿手改（ADR-0019 契约 codegen 专项 / ADR-0048 按域解冻；spec #940 片五③、#952 片一）。
// 域：投稿与审核（/api/contributions/*、/api/admin/contributions/*，含举报队列）
// 唯一事实源：后端注解 → backend/docs/swagger.json（CI 有新鲜度锁：backend-lint 的 swagger 步骤）。
// 再生成：cd backend && go run ./cmd/gen-apitypes
// 同步契约：backend/internal/apitypes/codegen_test.go 把本文件与注解渲染结果全等比对。
//
// 覆盖端点：
//   POST /contributions/upload-file
//   POST /contributions
//   GET  /contributions
//   GET  /contributions/mine
//   GET  /contributions/{id}
//   POST /contributions/{id}/download
//   DELETE /contributions/{id}
//   POST /contributions/{id}/report
//   GET  /admin/contributions/pending
//   POST /admin/contributions/{id}/approve
//   POST /admin/contributions/{id}/reject
//   POST /admin/contributions/{id}/archive
//   GET  /admin/contributions/reports
//   POST /admin/contributions/reports/{id}/handle
//
// 覆盖的 Go 类型：ContributionAuthor / ContributionFileDTO / ContributionItemDTO / ContributionPageResult / ContributionReportItemDTO / ContributionReportPageResult / DownloadResult
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

export interface ContributionAuthor {
  anonymous: boolean
  user_id: number
  username: string
}

export interface ContributionFileDTO {
  content_type: string
  file_id?: number
  file_name: string
  file_size: number
  file_url: string
}

export interface ContributionItemDTO {
  author: ContributionAuthor
  created_at: string
  credential_id: number
  downloads_count: number
  files?: ContributionFileDTO[]
  id: number
  intro: string
  is_anonymous: boolean
  reject_reason?: string
  status: string
  title: string
}

export interface ContributionPageResult {
  items: ContributionItemDTO[]
  page: number
  page_size: number
  total: number
}

export interface ContributionReportItemDTO {
  contribution_id: number
  contribution_title: string
  created_at: string
  id: number
  reason: string
  reporter_id: number
  status: number
}

export interface ContributionReportPageResult {
  items: ContributionReportItemDTO[]
  page: number
  page_size: number
  total: number
}

export interface DownloadResult {
  is_new: boolean
  tier_awarded: number
}
