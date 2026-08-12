import { unwrappedRequest } from './request'
import type { Question } from '@/types/question'

export interface LevelExamSessionQuery {
  page?: number
  page_size?: number
  status?: string
}

export interface LevelExamSessionPayload {
  name: string
  start_time: string
  end_time: string
  duration: number
  question_config?: Record<string, unknown>
  total_score?: number
  pass_score?: number
}

export interface SaveAnswerPayload {
  question_id?: number
  user_answer?: string
  time_used?: number
  answers?: unknown
  remaining_time?: number
  [key: string]: unknown
}

export interface SubmitExamPayload {
  /** 作答记录：{题目ID: 答案} */
  answers: Record<string | number, unknown>
  submit?: boolean
  [key: string]: unknown
}

export interface ExamHistoryQuery {
  page?: number
  page_size?: number
}

/** 考试场次（学员端列表项） */
export interface LevelExamSession {
  id: number
  name: string
  status?: string
  start_time?: string
  end_time?: string
  participant_id?: number
  [key: string]: unknown
}

/** 进入考试返回 */
export interface LevelExamEnterData {
  participant_id: number
  questions: Question[]
  answers?: Record<string, unknown>
  remaining_time: number
}

/** 考试结果 */
export interface LevelExamResult {
  participant: {
    score?: number | null
    is_passed?: boolean
    [key: string]: unknown
  }
  [key: string]: unknown
}

export const levelExamApi = {
  getSessions(params: LevelExamSessionQuery) {
    return unwrappedRequest.get<{ sessions: LevelExamSession[] }>('/level-exam/sessions', { params })
  },

  createSession(data: LevelExamSessionPayload) {
    return unwrappedRequest.post<{ id: number }>('/level-exam/sessions', data)
  },

  getSessionDetail(sessionId: number) {
    return unwrappedRequest.get<LevelExamSession>(`/level-exam/sessions/${sessionId}`)
  },

  updateSession(sessionId: number, data: LevelExamSessionPayload) {
    return unwrappedRequest.put<null>(`/level-exam/sessions/${sessionId}`, data)
  },

  deleteSession(sessionId: number) {
    return unwrappedRequest.delete<null>(`/level-exam/sessions/${sessionId}`)
  },

  updateSessionStatus(sessionId: number, status: string) {
    return unwrappedRequest.put<null>(`/level-exam/sessions/${sessionId}/status`, { status })
  },

  enterExam(sessionId: number) {
    return unwrappedRequest.post<LevelExamEnterData>(`/level-exam/sessions/${sessionId}/enter`)
  },

  saveAnswer(participantId: number, data: SaveAnswerPayload) {
    return unwrappedRequest.post<null>(`/level-exam/participants/${participantId}/save`, data)
  },

  submitExam(participantId: number, data: SubmitExamPayload) {
    return unwrappedRequest.post<{ result?: LevelExamResult }>(`/level-exam/participants/${participantId}/submit`, data)
  },

  getExamResult(participantId: number) {
    return unwrappedRequest.get<LevelExamResult>(`/level-exam/participants/${participantId}/result`)
  },

  getAvailableExams() {
    return unwrappedRequest.get<LevelExamSession[]>('/level-exam/available')
  },

  getExamHistory(params: ExamHistoryQuery) {
    return unwrappedRequest.get<{ exams: LevelExamSession[] }>('/level-exam/history', { params })
  }
}
