import { unwrappedRequest } from './request'
import type { WithUIQuestions } from '@/types/question'
import type {
  CategoryPracticeStartDTO,
  HistoryItemDTO,
  HistoryResultDTO,
  PracticePracticeStatsDTO,
  PracticeStartResultDTO,
  PracticeStatsDTO,
  ProgressResultDTO,
  ProgressSaveResultDTO,
  SubmitResultDTO
} from './generated/practiceMode'

// 响应类型**不再手写**：唯一事实源是后端注解 → backend/docs/swagger.json →
// `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3，spec #952 片一）。
// 本文件只留请求壳与端点装配，生成类型经下面一行按既有 import 路径透出。
//
// **本片不接线的唯一边界**：题目元素仍是跨域共享 UI 模型 Question（@/types/question），
// 不是生成的 QuestionDTO —— 该模型被题库 / 错题 / 考试多个域共用，收口归 questionBank 片
// （ADR-0048 决策 7）。强行接线要么加一层运行时映射、要么把 UI 模型的 options / status
// 降级成 unknown / string，两者都越过本片「响应字节不变 + 行为不变」的边界。
// 除 questions 元素外，本域响应字段全部走生成类型。
//
// 别名规则（片一统一口径）：**只在旧名与生成形状确实对应时**保留旧名（如 mockExam 的
// MockExamHistoryItem）；旧名对应的是错形状（如本模块的 PracticeStats / PracticeHistoryItem /
// PracticeProgressData —— 字段名或可空性与后端不符）则直接删除旧名，改出生成名，
// 差异在片一的字段级清单里逐条归档。
export type {
  HistoryItemDTO,
  HistoryResultDTO,
  PracticePracticeStatsDTO,
  PracticeStartResultDTO,
  PracticeStatsDTO,
  ProgressResultDTO,
  ProgressSaveResultDTO,
  SubmitResultDTO
}

type PaperPracticeMode = `paper:${number}`
type PracticeCategoryFilter = number | 'all' | 'unclassified'

export const practiceModeApi = {
  startCategoryPractice(data: { category_id: number; mode: string; count?: number; restart?: boolean }) {
    return unwrappedRequest.post<WithUIQuestions<CategoryPracticeStartDTO>>('/practice-mode/attempts', data, {
      headers: { 'X-Silent': 'true' }
    })
  },
  submitCategoryAnswer(data: { attempt_key: string; question_id: number; user_answer: unknown }) {
    return unwrappedRequest.post<SubmitResultDTO>('/practice-mode/attempts/answer', data)
  },
  saveProgress(index: number, mode: PaperPracticeMode, total: number, answersState: Record<string, unknown>) {
    return unwrappedRequest.post<ProgressSaveResultDTO>('/practice-mode/progress', { index, practice_mode: mode, total, answers_state: answersState })
  },
  getProgress(mode: PaperPracticeMode) {
    return unwrappedRequest.get<ProgressResultDTO>('/practice-mode/progress', { params: { mode, category_id: 'all' } })
  },
  // 提交单题答案并判定
  submitAnswer(data: { question_id: number; user_answer: string; practice_type?: string }) {
    return unwrappedRequest.post<SubmitResultDTO>('/practice-mode/submit', data, { params: { category_id: 'all' } })
  },
  // 练习统计
  getStats(categoryId: PracticeCategoryFilter = 'all') {
    return unwrappedRequest.get<PracticeStatsDTO>('/practice-mode/stats', { params: { category_id: categoryId } })
  },
  getPracticeStats(categoryId: PracticeCategoryFilter = 'all') {
    return unwrappedRequest.get<PracticePracticeStatsDTO>('/practice-mode/practice-stats', { params: { category_id: categoryId } })
  },
  // 练习历史
  getHistory(params: { page?: number; page_size?: number; category_id?: PracticeCategoryFilter }) {
    return unwrappedRequest.get<HistoryResultDTO>('/practice-mode/history', { params: { category_id: 'all', ...params } })
  }
}
