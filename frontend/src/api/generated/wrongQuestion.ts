// 生成文件，勿手改（ADR-0019 契约 codegen 专项 / ADR-0048 按域解冻；spec #940 片五③、#952 片一）。
// 域：错题本（/api/wrong-questions/*：列表 / 重做 / 移出 / 统计 / 导出）
// 唯一事实源：后端注解 → backend/docs/swagger.json（CI 有新鲜度锁：backend-lint 的 swagger 步骤）。
// 再生成：cd backend && go run ./cmd/gen-apitypes
// 同步契约：backend/internal/apitypes/codegen_test.go 把本文件与注解渲染结果全等比对。
//
// 覆盖端点：
//   GET  /wrong-questions
//   POST /wrong-questions/{question_id}/redo
//   POST /wrong-questions/{question_id}/remove
//   POST /wrong-questions/batch-remove
//   GET  /wrong-questions/stats
//   GET  /wrong-questions/export
//
// 覆盖的 Go 类型：QuestionDTO / SubmitResultDTO / WrongQuestionBatchRemoveResultDTO / WrongQuestionDTO / WrongQuestionPageDTO / WrongQuestionRemoveResultDTO / WrongQuestionStatsDTO
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

export interface QuestionDTO {
  answer?: string
  content: string
  created_at: string
  created_by: number | null
  created_by_type: string
  credential_id?: number
  explanation?: string
  id: number
  image_url: string
  options: unknown
  reference_answer?: string
  reject_reason: string
  score: number
  scoring_criteria?: string
  status: string
  tags?: unknown
  type: string
  updated_at: string
}

export interface SubmitResultDTO {
  accuracy_rate?: number
  ai_comment?: string
  ai_explanation?: string
  ai_fallback?: boolean
  ai_score?: number
  common_wrong?: string
  correct_answer: string
  explanation: string
  is_correct: boolean | null
  max_score?: number
  question_id: number
  reference_answer?: string
  scoring_criteria?: string
  total_attempts?: number
  user_answer: unknown
}

export interface WrongQuestionBatchRemoveResultDTO {
  removed: number
}

export interface WrongQuestionDTO {
  created_at: string
  favorite_id: number
  favorited: boolean
  id: number
  is_redone: boolean
  is_removed: boolean
  last_wrong_at: string
  question?: QuestionDTO
  question_id: number
  student_id: number
  wrong_count: number
}

export interface WrongQuestionPageDTO {
  items: WrongQuestionDTO[]
  page: number
  page_size: number
  total: number
}

export interface WrongQuestionRemoveResultDTO {
  removed: boolean
}

export interface WrongQuestionStatsDTO {
  by_type: Record<string, number>
  total: number
}
