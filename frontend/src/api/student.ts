import { unwrappedRequest } from './request'

const DASHBOARD_TIMEOUT = 45000

/** 学员主页 / 学习记录 */
export interface StudentProfile {
  user_id?: number
  nickname?: string
  avatar_url?: string
  study_stats?: Record<string, unknown>
  course_progress?: Array<Record<string, unknown>>
  records?: Array<Record<string, unknown>>
  [key: string]: unknown
}

/** 学习记录项 */
export interface StudyRecordItem {
  course_id?: number
  course_name?: string
  chapter_title?: string
  study_duration?: number
  study_date?: string
  [key: string]: unknown
}

/** 学习记录分页 */
export interface StudyRecordsData {
  records: StudyRecordItem[]
  total?: number
  [key: string]: unknown
}

/** 按天学习统计 */
export interface StudyStats {
  days: number
  labels: string[]
  data: number[]
  total_minutes: number
  active_days: number
  [key: string]: unknown
}

export const studentApi = {
  getProfile() {
    return unwrappedRequest.get<StudentProfile>('/student/profile', { timeout: DASHBOARD_TIMEOUT })
  },

  getRecords(params: { page?: number; page_size?: number; start_date?: string; end_date?: string }) {
    return unwrappedRequest.get<StudyRecordsData>('/student/records', { params, timeout: DASHBOARD_TIMEOUT })
  },

  // 学习统计（按天分组），days=7|30
  getStudyStats(params?: { days?: number }) {
    return unwrappedRequest.get<StudyStats>('/student/study-stats', { params, timeout: DASHBOARD_TIMEOUT })
  }
}
