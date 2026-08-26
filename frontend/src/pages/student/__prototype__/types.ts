// PROTOTYPE — throwaway types for 真题练习占位
export type Difficulty = '入门' | '进阶' | '专项' | '认证'

export interface Paper {
  id: number
  year: number
  title: string
  question_count: number
  duration_minutes: number
  credential_id: number
  credential_name: string
  source: string
  difficulty: Difficulty
}
