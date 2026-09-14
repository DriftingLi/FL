// 生成文件，勿手改（ADR-0019 契约 codegen 专项 / ADR-0048 按域解冻；spec #940 片五③、#952 片一）。
// 域：论坛（/api/forum/*、/api/admin/forum/*：帖子 / 回复 / 互动 / 举报 / 管理端）
// 唯一事实源：后端注解 → backend/docs/swagger.json（CI 有新鲜度锁：backend-lint 的 swagger 步骤）。
// 再生成：cd backend && go run ./cmd/gen-apitypes
// 同步契约：backend/internal/apitypes/codegen_test.go 把本文件与注解渲染结果全等比对。
//
// 覆盖端点：
//   POST /forum/upload-image
//   GET  /forum/topics
//   POST /forum/topics
//   GET  /forum/topics/{id}
//   PUT  /forum/topics/{id}
//   POST /forum/topics/{id}/replies
//   DELETE /forum/topics/{id}
//   DELETE /forum/replies/{id}
//   POST /forum/topics/{id}/like
//   DELETE /forum/topics/{id}/like
//   POST /forum/topics/{id}/report
//   POST /forum/replies/{id}/report
//   GET  /forum/my-topics
//   GET  /forum/my-replies
//   GET  /forum/my-liked-topics
//   GET  /forum/my-observed
//   GET  /forum/my-view-history
//   POST /forum/replies/{id}/like
//   DELETE /forum/replies/{id}/like
//   POST /forum/topics/{id}/accept
//   DELETE /forum/topics/{id}/accept
//   GET  /admin/forum/topics
//   GET  /admin/forum/topics/{id}
//   DELETE /admin/forum/topics/{id}
//   DELETE /admin/forum/replies/{id}
//   POST /admin/forum/topics/{id}/featured
//   DELETE /admin/forum/topics/{id}/featured
//   POST /admin/forum/topics/{id}/experience
//   DELETE /admin/forum/topics/{id}/experience
//   GET  /admin/forum/reports
//   PUT  /admin/forum/reports/{id}
//
// 覆盖的 Go 类型：ForumAuthor / ForumImageUploadResultDTO / ForumLikeResultDTO / ForumReplyDTO / ForumReportDTO / ForumReportPageResult / ForumTopicDTO / ForumTopicDetailDTO / ForumTopicPageResult / MyReplyDTO / MyReplyPageResult
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

export interface ForumAuthor {
  avatar_url: string
  user_id: number
  username: string
}

export interface ForumImageUploadResultDTO {
  url: string
}

export interface ForumLikeResultDTO {
  liked: boolean
  likes_count: number
}

export interface ForumReplyDTO {
  author: ForumAuthor
  can_delete: boolean
  content: string
  content_format: string
  created_at: string
  id: number
  images: string[]
  ip_city: string
  ip_province: string
  is_accepted: boolean
  liked_by_me: boolean
  likes_count: number
  parent_avatar_url?: string
  parent_id?: number
  parent_name?: string
  topic_id: number
}

export interface ForumReportDTO {
  created_at: string
  id: number
  reason: string
  reply_id?: number
  reporter: string
  reporter_id: number
  status: number
  topic_id?: number
  topic_title: string
}

export interface ForumReportPageResult {
  page: number
  pages: number
  reports: ForumReportDTO[]
  total: number
}

export interface ForumTopicDTO {
  accepted_reply_id?: number
  author: ForumAuthor
  can_delete: boolean
  category: string
  chapter_id: number | null
  chapter_title: string
  content: string
  content_format: string
  created_at: string
  id: number
  images: string[]
  ip_city: string
  ip_province: string
  is_experience: boolean
  is_featured: boolean
  last_reply_at: string | null
  liked_by_me: boolean
  likes_count: number
  reply_count: number
  reward_issued: boolean
  solved_at?: string
  title: string
  view_count: number
}

export interface ForumTopicDetailDTO {
  page: number
  pages: number
  replies: ForumReplyDTO[]
  topic: ForumTopicDTO
  total: number
}

export interface ForumTopicPageResult {
  page: number
  pages: number
  topics: ForumTopicDTO[]
  total: number
}

export interface MyReplyDTO {
  author: ForumAuthor
  content: string
  content_format: string
  created_at: string
  id: number
  images: string[]
  parent_id?: number
  topic_id: number
  topic_title: string
}

export interface MyReplyPageResult {
  page: number
  pages: number
  replies: MyReplyDTO[]
  total: number
}
