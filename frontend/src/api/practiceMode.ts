import { unwrappedRequest } from './request'
import { useCredentialStore } from '@/stores/credential'
import type { Question, WithUIQuestions } from '@/types/question'
import type {
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

/** 惰性读取当前证件 id（无 Pinia 环境/未选证件返回 undefined；容错不阻断保存） */
function currentCredentialId(): number | undefined {
  try {
    return useCredentialStore().current?.id
  } catch {
    return undefined
  }
}

// 题库练习模式接口，对应后端 /api/practice-mode（JWT + CapQuestionPractice + CredentialScoped）。
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
  // #413：传参对象让 params 占位存在（缺省不携带 credential_id，证件分区由服务端兜底）。
  startSequential(params?: { credential_id?: number }) {
    return unwrappedRequest.get<WithUIQuestions<PracticeStartResultDTO>>('/practice-mode/sequential', { params: params || {} })
  },
  // 顺序练习进度（卡片展示用；#413 返回体含实时池总数 pool_total）
  getSequentialProgress(params?: { credential_id?: number }) {
    return unwrappedRequest.get<ProgressResultDTO>('/practice-mode/sequential-progress', { params: params || {} })
  },
  // 保存练习游标和答题状态（顺序/标签/按卷练习）
  // #505：顺序练习进度按证件分桶（#414），保存 body 必须携带当前证件 id——服务端对 body 没有
  // 兜底路径（客户端拦截器也不注入证件），漏带会把游标写进 NULL 孤儿行（断点回跳根因）。
  // 非 sequential 模式（标签/按卷）不分桶，不携带。
  saveProgress(index: number, mode: PracticeModeKey = 'sequential', total: number = 0, answersState: Record<string, unknown> = {}) {
    // body 里的 credential_id 只可能是「保存时的当前证件」（用户没有指定证件的入口）：与 query 侧
    // 的显式浏览参数同名不同义，此处由客户端下发、服务端无兜底路径（拦截器不碰 POST body）。
    const credentialId = mode === 'sequential' ? currentCredentialId() : undefined
    return unwrappedRequest.post<ProgressSaveResultDTO>('/practice-mode/progress', { index, practice_mode: mode, total, answers_state: answersState, credential_id: credentialId })
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
  // 刷题数据展示（顶部 3 宫格，与 /stats 独立）。证件分区走服务端 CredentialScoped 兜底：不传即按
  // 登录学员当前证件，显式 credential_id 优先。
  getPracticeStats(params?: { credential_id?: number }) {
    return unwrappedRequest.get<PracticePracticeStatsDTO>('/practice-mode/practice-stats', { params: params || {} })
  },
  // 练习历史
  getHistory(params: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<HistoryResultDTO>('/practice-mode/history', { params })
  }
}
