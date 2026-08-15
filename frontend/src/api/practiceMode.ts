import { unwrappedRequest } from './request'
import type { PracticeProgress, Question, SubmitResult } from '@/types/question'

/** 练习进度（断点续练用，含答题状态；与后端 ProgressResultDTO 对齐） */
export interface PracticeProgressData extends PracticeProgress {
  answers_state?: Record<string, unknown>
  practice_mode?: string
}

/** 练习统计 */
export interface PracticeStats {
  total?: number
  completed?: number
  in_progress?: number
  total_count?: number
  correct_count?: number
}

/** 练习历史分页结果（与后端 HistoryResultDTO 对齐） */
export interface PracticeHistory {
  total: number
  page: number
  page_size: number
  records: PracticeHistoryItem[]
}

/** 练习历史项（与后端 HistoryItemDTO 对齐） */
export interface PracticeHistoryItem {
  id: number
  student_id?: number
  question_id?: number
  is_correct?: boolean
  practice_type?: string
  user_answer?: string
  created_at?: string
  question?: Question
}

// 题库练习模式接口，对应后端 /api/practice-mode
export const practiceModeApi = {
  // 随机练习：随机抽 count 题（可按题型筛选）
  getFreeQuestions(params?: { count?: number; type?: string }) {
    return unwrappedRequest.get<Question[]>('/practice-mode/free', { params })
  },
  // 标签练习：开始/续练（返回当前批次题目 + 进度，mode 为 tag:<tagID>）
  startTagPractice(params: { tag_id: number; count?: number }) {
    return unwrappedRequest.get<{ questions?: Question[]; current_index?: number; total?: number }>(
      '/practice-mode/tag',
      { params }
    )
  },
  // 顺序练习：开始/续练，返回当前批次题目 + 进度
  startSequential() {
    return unwrappedRequest.get<{ questions?: Question[]; progress?: PracticeProgressData }>('/practice-mode/sequential')
  },
  // 顺序练习进度（卡片展示用）
  getSequentialProgress() {
    return unwrappedRequest.get<PracticeProgress>('/practice-mode/sequential-progress')
  },
  // 保存练习游标和答题状态（顺序/专项/标签练习）
  saveProgress(index: number, mode: string = 'sequential', total: number = 0, answersState: Record<string, unknown> = {}) {
    return unwrappedRequest.post<null>('/practice-mode/progress', { index, practice_mode: mode, total, answers_state: answersState })
  },
  // 查询任意模式的练习进度和答题状态（断点续练用）
  getProgress(mode: string = 'sequential') {
    return unwrappedRequest.get<PracticeProgressData>('/practice-mode/progress', { params: { mode } })
  },
  // 提交单题答案并判定
  submitAnswer(data: { question_id: number; user_answer: string; practice_type?: string }) {
    return unwrappedRequest.post<SubmitResult>('/practice-mode/submit', data)
  },
  // 练习统计
  getStats() {
    return unwrappedRequest.get<PracticeStats>('/practice-mode/stats')
  },
  // 练习历史
  getHistory(params: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<PracticeHistory>('/practice-mode/history', { params })
  }
}
