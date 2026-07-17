import type { QuestionType, CourseCategory } from '@/types/question'

// 题型中文映射
export const typeMap: Record<QuestionType, string> = {
  single_choice: '单选题',
  multi_choice: '多选题',
  true_false: '判断题',
  fault_image: '故障识图',
  short_answer: '简答题'
}

export const questionTypeOptions = Object.entries(typeMap).map(([value, label]) => ({ value, label }))

// 课程四分类
export const categoryMap: Record<CourseCategory, string> = {
  CATEGORY_01: '基础理论',
  CATEGORY_02: '安全规范',
  CATEGORY_03: '实操技能',
  CATEGORY_04: '进阶提升'
}

export const categoryOptions = Object.entries(categoryMap).map(([value, label]) => ({ value, label }))

// 考试场次状态
export const sessionStatusMap: Record<string, string> = {
  upcoming: '未开始',
  ongoing: '进行中',
  finished: '已结束'
}

// 随机练习可选题量
export const randomCountOptions = [
  { value: 10, label: '10 题' },
  { value: 20, label: '20 题' },
  { value: 30, label: '30 题' },
  { value: 50, label: '50 题' }
]

// 模拟考试可选题量
export const mockCountOptions = [
  { value: 20, label: '20 题' },
  { value: 40, label: '40 题' },
  { value: 60, label: '60 题' }
]
