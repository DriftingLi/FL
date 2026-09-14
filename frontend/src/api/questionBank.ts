// 已迁移模块：走 unwrappedRequest（拦截器解包信封，成功直接返回业务数据 Promise<T>）。
//
// 响应类型**不再手写**：唯一事实源是后端注解 → backend/docs/swagger.json →
// `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3，issue #964 片六）。
// 本文件只留请求壳、端点装配与名称适配，入参（query / body）类型不生成、仍手写（决策 3）。
//
// **边界**：题目元素在页面层仍是跨域共享 UI 模型 `Question`（@/types/question），
// 其枚举窄化（QuestionType / QuestionStatus）与 options 结构尚无法由注解表达
// （片一已知限制：注解层没有枚举词汇）；故列表响应用 `WithUIQuestions` 显式标注该替换点。
// 题目 UI 模型的收口需要先补生成器的 enum 能力，另立片（ADR-0048 片一实施修订）。
import { unwrappedRequest } from './request'
import type { WithUIQuestions } from '@/types/question'
import type {
  QuestionBankStatsDTO,
  QuestionDTO,
  QuestionImageUploadDTO,
  QuestionImportResultDTO,
  QuestionPageDTO,
  QuestionPublishResultDTO,
  QuestionRejectResultDTO
} from './generated/questionBank'

export type {
  QuestionBankStatsDTO,
  QuestionDTO,
  QuestionImageUploadDTO,
  QuestionImportResultDTO,
  QuestionPageDTO,
  QuestionPublishResultDTO,
  QuestionRejectResultDTO
}

/** 查询入参（不生成，ADR-0048 决策 3） */
export interface QuestionsQuery {
  page?: number
  page_size?: number
  keyword?: string
  type?: string
  status?: string
  created_by?: number
  /** 按题库标签筛选（LH-28） */
  tag_id?: number
  credential_id?: number
  /** 排序口径（#412）：id_asc 按 ID 升序；缺省为服务端默认（最新提交优先） */
  sort?: string
}

/** 创建/更新入参（不生成，ADR-0048 决策 3） */
export interface QuestionPayload {
  type: string
  content: string
  options?: Record<string, unknown>
  answer: string
  explanation?: string
  image_url?: string
  reference_answer?: string
  scoring_criteria?: string
  score?: number
  status?: string
  credential_id?: number | null
  /** 题库标签（LH-28，创建/更新时全量替换） */
  tag_ids?: number[]
}

/** 批量驳回入参（不生成，ADR-0048 决策 3） */
export interface BatchRejectPayload {
  question_ids: number[]
  reason: string
}

export const questionBankApi = {
  getQuestions(params: QuestionsQuery) {
    // credential_id 由主 client 请求拦截器默认注入（#387）
    return unwrappedRequest.get<WithUIQuestions<QuestionPageDTO>>('/question-bank/questions', { params })
  },

  createQuestion(data: QuestionPayload) {
    return unwrappedRequest.post<QuestionDTO>('/question-bank/questions', data)
  },

  getQuestion(id: number) {
    return unwrappedRequest.get<QuestionDTO>(`/question-bank/questions/${id}`)
  },

  updateQuestion(id: number, data: Partial<QuestionPayload>) {
    return unwrappedRequest.put<QuestionDTO>(`/question-bank/questions/${id}`, data)
  },

  deleteQuestion(id: number) {
    return unwrappedRequest.delete<null>(`/question-bank/questions/${id}`)
  },

  /** 单题发布：后端返回发布后的题目（此前被当成无载荷） */
  publishQuestion(id: number) {
    return unwrappedRequest.post<QuestionDTO>(`/question-bank/questions/${id}/publish`)
  },

  /** 单题驳回：后端返回驳回后的题目（此前被当成无载荷） */
  rejectQuestion(id: number, reason: string) {
    return unwrappedRequest.post<QuestionDTO>(`/question-bank/questions/${id}/reject`, { reason })
  },

  /** 批量发布：后端返回 {published_count}（此前被当成无载荷） */
  batchPublish(questionIds: number[]) {
    return unwrappedRequest.post<QuestionPublishResultDTO>('/question-bank/questions/batch-publish', { question_ids: questionIds })
  },

  /** 批量驳回：后端返回 {rejected_count}（此前被当成无载荷） */
  batchReject(questionIds: number[], reason: string) {
    return unwrappedRequest.post<QuestionRejectResultDTO>('/question-bank/questions/batch-reject', { question_ids: questionIds, reason })
  },

  /** 批量导入：后端返回 {success_count, error_count, errors}（此前手写为 success_count/failed_count，failed_count 不存在） */
  batchImport(questions: QuestionPayload[]) {
    return unwrappedRequest.post<QuestionImportResultDTO>('/question-bank/questions/batch-import', { questions })
  },

  /** 题库统计：后端返回 {total, by_type, by_status}（此前手写为 total/published/pending/total_count，后三者不存在） */
  getStats() {
    return unwrappedRequest.get<QuestionBankStatsDTO>('/question-bank/stats')
  },

  uploadImage(formData: FormData) {
    return unwrappedRequest.post<QuestionImageUploadDTO>('/question-bank/upload-image', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 30000
    })
  }
}
