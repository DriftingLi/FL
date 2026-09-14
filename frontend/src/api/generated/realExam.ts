// 生成文件，勿手改（ADR-0019 契约 codegen 专项 / ADR-0048 按域解冻；spec #940 片五③、#952 片一）。
// 域：真题套卷（/api/real-exam/*：列表 / 按卷练习 / 按卷开考 / 积分兑换）
// 唯一事实源：后端注解 → backend/docs/swagger.json（CI 有新鲜度锁：backend-lint 的 swagger 步骤）。
// 再生成：cd backend && go run ./cmd/gen-apitypes
// 同步契约：backend/internal/apitypes/codegen_test.go 把本文件与注解渲染结果全等比对。
//
// 覆盖端点：
//   GET  /real-exam/papers
//   GET  /real-exam/papers/{paper_id}/practice
//   POST /real-exam/papers/{paper_id}/exam
//   POST /real-exam/papers/{paper_id}/redeem
//
// 覆盖的 Go 类型：MockExamStartDTO / PracticeStartResultDTO / QuestionDTO / RealExamPaperDTO / RedeemResult
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

export interface MockExamStartDTO {
  duration: number
  mock_exam_id: number
  questions: QuestionDTO[]
  remaining_time: number
  total_questions: number
  total_score: number
}

export interface PracticeStartResultDTO {
  completed: number
  current_index: number
  questions: QuestionDTO[]
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

export interface RealExamPaperDTO {
  duration_minutes: number
  entitled: boolean
  paper_id: number
  price: number
  question_count: number
  source?: string
  title: string
  year?: number
}

export interface RedeemResult {
  balance: number
  ref_id: string
  sku: string
  total_earned: number
}
