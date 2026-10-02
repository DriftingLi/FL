// 生成文件，勿手改（ADR-0019 契约 codegen 专项 / ADR-0048 按域解冻；spec #940 片五③、#952 片一）。
// 域：模拟考试（/api/mock-exam/*，含真题卷整卷复用链路）
// 唯一事实源：后端注解 → backend/docs/swagger.json（CI 有新鲜度锁：backend-lint 的 swagger 步骤）。
// 再生成：cd backend && go run ./cmd/gen-apitypes
// 同步契约：backend/internal/apitypes/codegen_test.go 把本文件与注解渲染结果全等比对。
//
// 覆盖端点：
//   POST /mock-exam/start
//   POST /mock-exam/{mock_exam_id}/save
//   GET  /mock-exam/{mock_exam_id}/resume
//   POST /mock-exam/{mock_exam_id}/submit
//   GET  /mock-exam/{mock_exam_id}/result
//   GET  /mock-exam/history
//
// 覆盖的 Go 类型：MockExamAnswerDetailDTO / MockExamHistoryDTO / MockExamHistoryItemDTO / MockExamResultDTO / MockExamResumeDTO / MockExamStartDTO / MockExamSubmitDTO / QuestionDTO
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

export interface MockExamAnswerDetailDTO {
  ai_comment?: string
  ai_fallback?: boolean
  ai_score?: number
  content: string
  correct_answer: string
  explanation: string
  is_correct: boolean | null
  max_score: number
  options: unknown
  question_id: number
  score: number
  type: string
  user_answer: unknown
}

export interface MockExamHistoryDTO {
  exams: MockExamHistoryItemDTO[]
  page: number
  page_size: number
  total: number
}

export interface MockExamHistoryItemDTO {
  answers: unknown
  created_at: string
  duration: number
  id: number
  paper_id?: number
  question_ids: unknown
  remaining_time: number
  result: unknown
  score: number | null
  start_time: string
  status: string
  student_id: number
  submit_time: string
}

export interface MockExamResultDTO {
  accuracy: number
  correct_count: number
  details: MockExamAnswerDetailDTO[] | null
  max_score: number
  mock_exam_id: number
  submit_time: string
  total_questions: number
  total_score: number
}

export interface MockExamResumeDTO {
  answers: unknown
  duration: number
  mock_exam_id: number
  questions: QuestionDTO[]
  remaining_time: number
  start_time: string
}

export interface MockExamStartDTO {
  duration: number
  mock_exam_id: number
  questions: QuestionDTO[]
  remaining_time: number
  total_questions: number
  total_score: number
}

export interface MockExamSubmitDTO {
  accuracy: number
  correct_count: number
  details: MockExamAnswerDetailDTO[] | null
  max_score: number
  total_questions: number
  total_score: number
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
