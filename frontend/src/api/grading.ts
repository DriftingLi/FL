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
}

/** 待批改列表响应（后端兼容数组或分页对象两种形态） */
export type GradingParticipantList =
  | GradingParticipant[]
  | { participants?: GradingParticipant[]; items?: GradingParticipant[] }

/** 答题明细（含后端评分字段 + 前端批改交互阶段字段） */
export interface GradingAnswer {
  id: number
  question_id?: number
  user_answer?: string
  score?: number | null
  ai_score?: number | null
  ai_comment?: string
  grading_comment?: string
  is_correct?: boolean | null
  grader_id?: number | null
  question?: { type?: string; score?: number }
  // 前端批改交互阶段字段（内存态，不入库）
  _score?: number
  _comment?: string
  _confirming?: boolean
  _aiLoading?: boolean
  _regrading?: boolean
  _regradeScore?: number
  _regradeComment?: string
}

/** 参与者详情（含答题明细） */
export interface GradingParticipantDetail {
  answers?: GradingAnswer[]
}

/** AI 评分结果 */
export interface AiGradeResult {
  ai_score?: number
  ai_comment?: string
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
