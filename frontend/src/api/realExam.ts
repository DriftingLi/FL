import { unwrappedRequest } from './request'
import { useCredentialStore } from '@/stores/credential'
import type { Question } from '@/types/question'

/** 真题套卷列表项（与后端 RealExamPaperDTO 对齐） */
export interface RealExamPaper {
  paper_id: number
  title: string
  year?: number
  source?: string
  question_count: number
  duration_minutes: number
  entitled: boolean
  price: number
}

/** 按卷练习开始/续练（与后端 PracticeStartResultDTO 对齐） */
export interface RealExamPracticeStart {
  questions: Question[]
  current_index: number
  total: number
  completed: number
}

/** 按卷开考（与后端 MockExamStartDTO 对齐，后续复用 mock-exam 端点） */
export interface RealExamStartResult {
  mock_exam_id: number
  duration: number
  total_score: number
  total_questions: number
  remaining_time: number
  questions: Question[]
}

/** 兑换结果（与后端 RedeemResult 对齐） */
export interface RealExamRedeemResult {
  balance: number
  total_earned: number
  sku: string
  ref_id: string
}

// 真题套卷接口，对应后端 /api/real-exam
export const realExamApi = {
  // 套卷列表：按当前证件分区，附兑换状态与单价
  listPapers() {
    const params: Record<string, unknown> = {}
    try { const cred = useCredentialStore().current?.id; if (cred) params.credential_id = cred } catch {}
    return unwrappedRequest.get<RealExamPaper[]>('/real-exam/papers', { params })
  },
  // 按卷练习开始/续练（未兑换时后端拒绝）
  startPractice(paperId: number) {
    return unwrappedRequest.get<RealExamPracticeStart>(`/real-exam/papers/${paperId}/practice`)
  },
  // 按卷开考：返回 mock_exam_id，之后复用 mock-exam 的 save/submit/result 端点
  startExam(paperId: number) {
    return unwrappedRequest.post<RealExamStartResult>(`/real-exam/papers/${paperId}/exam`)
  },
  // 积分兑换单套卷（重复兑换后端报"已兑换"）
  redeemPaper(paperId: number) {
    return unwrappedRequest.post<RealExamRedeemResult>(`/real-exam/papers/${paperId}/redeem`)
  }
}
