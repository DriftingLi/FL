import request from './request'
import type { PracticeProgress, Question, SubmitResult } from '@/types/question'

/** 练习进度（断点续练用，含答题状态） */
export interface PracticeProgressData extends PracticeProgress {
  answers_state?: Record<string, unknown>
  practice_mode?: string
  [key: string]: unknown
}

/** 练习统计 */
export interface PracticeStats {
  [key: string]: unknown
}

/** 练习历史项 */
export interface PracticeHistoryItem {
  id: number
  practice_mode?: string
  correct_count?: number
  total_count?: number
  finished_at?: string
  [key: string]: unknown
}

// 题库练习模式接口，对应后端 /api/practice-mode
export const practiceModeApi = {
  // 随机练习：随机抽 count 题（可按题型筛选）
  getFreeQuestions(params?: { count?: number; type?: string }) {
    return request.get<Question[]>('/practice-mode/free', { params })
  },
  // 标签练习：按题库标签抽题（返回结构与 /free 一致、不含答案）
  getTagQuestions(params: { tag_id: number; count?: number }) {
    return request.get<Question[]>('/practice-mode/tag', { params })
  },
  // 顺序练习：开始/续练，返回当前批次题目 + 进度
  startSequential() {
    return request.get<{ questions?: Question[]; progress?: PracticeProgressData }>('/practice-mode/sequential')
  },
  // 顺序练习进度（卡片展示用）
  getSequentialProgress() {
    return request.get<PracticeProgress>('/practice-mode/sequential-progress')
  },
  // 保存练习游标和答题状态（顺序/专项/标签练习）
  saveProgress(index: number, mode: string = 'sequential', total: number = 0, answersState: Record<string, unknown> = {}) {
    return request.post<null>('/practice-mode/progress', { index, practice_mode: mode, total, answers_state: answersState })
  },
  // 查询任意模式的练习进度和答题状态（断点续练用）
  getProgress(mode: string = 'sequential') {
    return request.get<PracticeProgressData>('/practice-mode/progress', { params: { mode } })
  },
  // 提交单题答案并判定
  submitAnswer(data: { question_id: number; user_answer: string; practice_type?: string }) {
    return request.post<SubmitResult>('/practice-mode/submit', data)
  },
  // 练习统计
  getStats() {
    return request.get<PracticeStats>('/practice-mode/stats')
  },
  // 练习历史
  getHistory(params: { page?: number; page_size?: number }) {
    return request.get<{ items: PracticeHistoryItem[]; total: number }>('/practice-mode/history', { params })
  }
}
