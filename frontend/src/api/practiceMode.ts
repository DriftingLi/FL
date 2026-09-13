import { unwrappedRequest } from './request'
import { useCredentialStore } from '@/stores/credential'
import type { Question } from '@/types/question'
import type {
  HistoryItemDTO,
  HistoryResultDTO,
  PracticePracticeStatsDTO,
  PracticeStartResultDTO,
  PracticeStatsDTO,
  ProgressResultDTO,
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
export type {
  HistoryItemDTO,
  HistoryResultDTO,
  PracticePracticeStatsDTO,
  PracticeStartResultDTO,
  PracticeStatsDTO,
  ProgressResultDTO,
  SubmitResultDTO
}

/** 用 UI 模型的题目元素替换生成 DTO 的 questions（见文件头边界说明）。 */
type WithUIQuestions<T extends { questions: unknown }> = Omit<T, 'questions'> & { questions: Question[] }

/** 惰性读取当前证件 id（无 Pinia 环境/未选证件返回 undefined；容错不阻断保存） */
function currentCredentialId(): number | undefined {
  try {
    return useCredentialStore().current?.id
  } catch {
    return undefined
  }
}

// 题库练习模式接口，对应后端 /api/practice-mode（credential_id 由主 client 拦截器默认注入，#387）
//
// 练习模式标识白名单（#390，与后端 ParsePracticeMode 封闭校验对齐 #386）：
// 顺序 'sequential' / 标签 'tag:<tagID>' / 按卷 'paper:<paperID>'；未知 mode 后端 400。
// 专项（free:<type>）与随机不在封闭集，不落进度（见 QuestionBank getPracticeModeKey）。
export type PracticeModeKey = 'sequential' | `tag:${number}` | `paper:${number}`

export const practiceModeApi = {
  // 随机练习：随机抽 count 题（可按题型筛选）
  getFreeQuestions(params?: { count?: number; type?: string }) {
    return unwrappedRequest.get<Question[]>('/practice-mode/free', { params })
  },
  // 标签练习：开始/续练（返回当前批次题目 + 进度，mode 为 tag:<tagID>）
  startTagPractice(params: { tag_id: number; count?: number }) {
    return unwrappedRequest.get<WithUIQuestions<PracticeStartResultDTO>>('/practice-mode/tag', { params })
  },
  // 顺序练习：开始/续练，返回当前批次题目 + 进度
  // #413：传参对象让「证件过滤默认注入」拦截器真正生效（此前不传 params 被跳过）。
  startSequential(params?: { credential_id?: number }) {
    return unwrappedRequest.get<WithUIQuestions<PracticeStartResultDTO>>('/practice-mode/sequential', { params: params || {} })
  },
  // 顺序练习进度（卡片展示用；#413 返回体含实时池总数 pool_total）
  getSequentialProgress(params?: { credential_id?: number }) {
    return unwrappedRequest.get<ProgressResultDTO>('/practice-mode/sequential-progress', { params: params || {} })
  },
  // 保存练习游标和答题状态（顺序/标签/按卷练习）
  // #505：顺序练习进度按证件分桶（#414），保存 body 必须携带当前证件 id——拦截器只注入
  // GET query 不触碰 POST body，漏带会把游标写进 NULL 孤儿行（断点回跳根因）。非 sequential
  // 模式（标签/按卷）不分桶，不携带。
  saveProgress(index: number, mode: PracticeModeKey = 'sequential', total: number = 0, answersState: Record<string, unknown> = {}) {
    const credentialId = mode === 'sequential' ? currentCredentialId() : undefined
    return unwrappedRequest.post<null>('/practice-mode/progress', { index, practice_mode: mode, total, answers_state: answersState, credential_id: credentialId })
  },
  // 查询任意模式的练习进度和答题状态（断点续练用）
  getProgress(mode: PracticeModeKey = 'sequential') {
    return unwrappedRequest.get<ProgressResultDTO>('/practice-mode/progress', { params: { mode } })
  },
  // 提交单题答案并判定
  submitAnswer(data: { question_id: number; user_answer: string; practice_type?: string }) {
    return unwrappedRequest.post<SubmitResultDTO>('/practice-mode/submit', data)
  },
  // 练习统计
  getStats() {
    return unwrappedRequest.get<PracticeStatsDTO>('/practice-mode/stats')
  },
  // 刷题数据展示（顶部 3 宫格，credential_id 由主 client 拦截器默认注入，与 /stats 独立）
  getPracticeStats(params?: { credential_id?: number }) {
    return unwrappedRequest.get<PracticePracticeStatsDTO>('/practice-mode/practice-stats', { params: params || {} })
  },
  // 练习历史
  getHistory(params: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<HistoryResultDTO>('/practice-mode/history', { params })
  }
}
