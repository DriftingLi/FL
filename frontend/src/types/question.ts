// 题目、练习、考试相关类型定义
// 题目字段与后端 typed QuestionDTO 契约逐字对齐（见 backend/internal/service/question_dto.go）。

export type QuestionType = 'single_choice' | 'multi_choice' | 'true_false' | 'fault_image' | 'short_answer'

export type QuestionStatus = 'draft' | 'pending' | 'published'

export interface QuestionTagItem {
  id: number
  code: string
  name: string
  sort_order: number
  status: number
}

export interface Question {
  id: number
  type: QuestionType
  content: string
  options: Record<string, string> | null
  image_url: string
  status: QuestionStatus
  reject_reason: string
  score: number
  created_by: number | null
  created_by_type: string
  created_at: string
  updated_at: string
  // 学员侧（练习/考试/错题重做）不返回以下字段
  answer?: string
  explanation?: string
  reference_answer?: string
  scoring_criteria?: string
  // 题库管理面附加
  tags?: QuestionTagItem[] | null
}

export interface PracticeProgress {
  completed: number
  total: number
  current_index: number
}

export interface SubmitResult {
  is_correct: boolean | null
  correct_answer: string
  explanation: string
  question_id: number
  user_answer: unknown
  ai_score?: number
  ai_comment?: string
  ai_fallback?: boolean
  reference_answer?: string
  scoring_criteria?: string
  max_score?: number
  accuracy_rate?: number | null
  common_wrong?: string | null
  total_attempts?: number
  ai_explanation?: string
}
