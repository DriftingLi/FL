// 题目、练习、考试相关类型定义

export type QuestionType = 'single_choice' | 'multi_choice' | 'true_false' | 'fault_image' | 'short_answer'

export type QuestionStatus = 'draft' | 'pending' | 'published'

export interface Question {
  id: number
  type: QuestionType
  content: string
  options?: Record<string, string>
  image_url?: string
  status?: QuestionStatus
  score?: number
  // 学员侧不返回以下字段
  answer?: string
  explanation?: string
  reference_answer?: string
  scoring_criteria?: string
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
}
