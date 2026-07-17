// 题目、练习、考试相关类型定义（已取消等级制度）

export type QuestionType = 'single_choice' | 'multi_choice' | 'true_false' | 'fault_image' | 'short_answer'

export type QuestionStatus = 'draft' | 'pending' | 'published'

// 课程四分类
export type CourseCategory = 'CATEGORY_01' | 'CATEGORY_02' | 'CATEGORY_03' | 'CATEGORY_04'

// 练习模式
export type PracticeMode = 'sequential' | 'free' | 'category' | 'knowledge_point'

export interface Question {
  id: number
  type: QuestionType
  content: string
  options?: Record<string, string>
  image_url?: string
  knowledge_point_id?: number | null
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

export interface CategoryStat {
  category: CourseCategory
  count: number
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
