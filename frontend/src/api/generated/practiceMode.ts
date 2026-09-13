// 生成文件，勿手改（ADR-0019 契约 codegen 专项 / ADR-0048 按域解冻；spec #940 片五③、#952 片一）。
// 域：题库练习模式（/api/practice-mode/*：随机 / 标签 / 顺序 / 进度 / 判定 / 统计 / 历史）
// 唯一事实源：后端注解 → backend/docs/swagger.json（CI 有新鲜度锁：backend-lint 的 swagger 步骤）。
// 再生成：cd backend && go run ./cmd/gen-apitypes
// 同步契约：backend/internal/apitypes/codegen_test.go 把本文件与注解渲染结果全等比对。
//
// 覆盖端点：
//   GET  /practice-mode/free
//   GET  /practice-mode/tag
//   GET  /practice-mode/sequential
//   GET  /practice-mode/sequential-progress
//   POST /practice-mode/progress
//   GET  /practice-mode/progress
//   POST /practice-mode/submit
//   GET  /practice-mode/practice-stats
//   GET  /practice-mode/stats
//   GET  /practice-mode/history
//
// 覆盖的 Go 类型：HistoryItemDTO / HistoryResultDTO / PracticePracticeStatsDTO / PracticeStartResultDTO / PracticeStatsDTO / PracticeTypeStat / ProgressResultDTO / QuestionDTO / SubmitResultDTO
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

export interface HistoryItemDTO {
  created_at: string
  id: number
  is_correct: boolean
  practice_type: string
  question?: QuestionDTO
  question_id: number
  student_id: number
  user_answer: string
}

export interface HistoryResultDTO {
  page: number
  page_size: number
  records: HistoryItemDTO[]
  total: number
}

export interface PracticePracticeStatsDTO {
  today_count: number
  total_count: number
  total_days: number
}

export interface PracticeStartResultDTO {
  completed: number
  current_index: number
  questions: QuestionDTO[]
  total: number
}

export interface PracticeStatsDTO {
  accuracy: number
  by_type: Record<string, PracticeTypeStat>
  correct: number
  total: number
  wrong: number
}

export interface PracticeTypeStat {
  accuracy: number
  correct: number
  total: number
}

export interface ProgressResultDTO {
  answers_state: Record<string, unknown> | null
  completed: number
  current_index: number
  pool_total: number
  total: number
}

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
