import { unwrappedRequest } from './request'

export interface GradeAnswerPayload {
  score: number
  comment?: string
}

export interface GradingStatsQuery {
  days?: number
}

export interface GradingParticipantsQuery {
  page?: number
  page_size?: number
  status?: string
  course_id?: number
}

/** 批改列表项 */
export interface GradingParticipant {
  id: number
  grading_status?: string
  student_name?: string
  exam_name?: string
  session_name?: string
  participant_id?: number
  ungraded_count?: number
  [key: string]: unknown
}

/** 待批改列表响应（后端兼容数组或分页对象两种形态） */
export type GradingParticipantList =
  | GradingParticipant[]
  | { participants?: GradingParticipant[]; items?: GradingParticipant[] }

/** 参与者详情（含答题明细） */
export interface GradingParticipantDetail {
  answers?: {
    id: number
    ai_score?: number | null
    ai_comment?: string
    score?: number | null
    _score?: number
    _comment?: string
    [key: string]: unknown
  }[]
  [key: string]: unknown
}

/** AI 评分结果 */
export interface AiGradeResult {
  ai_score?: number
  ai_comment?: string
  [key: string]: unknown
}

export const gradingApi = {
  getSubmittedParticipants(params?: GradingParticipantsQuery) {
    return unwrappedRequest.get<GradingParticipantList>('/grading/participants', { params })
  },

  getParticipantDetail(participantId: number) {
    return unwrappedRequest.get<GradingParticipantDetail>(`/grading/participants/${participantId}`)
  },

  gradeAnswer(answerId: number, data: GradeAnswerPayload) {
    return unwrappedRequest.post<null>(`/grading/${answerId}/grade`, data)
  },

  regradeAnswer(answerId: number, data: GradeAnswerPayload) {
    return unwrappedRequest.post<null>(`/grading/${answerId}/regrade`, data)
  },

  getGradingStats(params: GradingStatsQuery) {
    return unwrappedRequest.get<Record<string, unknown>>('/grading/stats', { params })
  },

  confirmAiGrading(answerId: number) {
    return unwrappedRequest.post<null>(`/grading/${answerId}/confirm-ai`)
  },

  aiGradeAnswer(answerId: number) {
    return unwrappedRequest.post<AiGradeResult>(`/grading/${answerId}/ai-grade`)
  },

  confirmObjectiveAnswers(participantId: number) {
    return unwrappedRequest.post<null>(`/grading/participants/${participantId}/confirm-objective`)
  }
}
