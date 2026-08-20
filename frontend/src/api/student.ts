import { unwrappedRequest } from './request'

const DASHBOARD_TIMEOUT = 45000

/** 学员学习统计概览（与后端 StudyStatsDTO 对齐） */
export interface StudyStatSummary {
  total_courses?: number
  total_study_duration?: number
  completed_courses?: number
  learning_courses?: number
  latest_study_time?: string
}

/** 课程进度条目（与后端 CourseProgressDTO 对齐） */
export interface CourseProgressItem {
  course_id?: number
  course_name?: string
  progress?: number
  study_duration?: number
  total_chapters?: number
  study_date?: string
}

/** 学员主页响应（student_info + study_stats + course_progress） */
export interface StudentProfile {
  user_id?: number
  uid?: string
  account?: string
  username?: string
  avatar_url?: string
  study_stats?: StudyStatSummary
  course_progress?: CourseProgressItem[]
}

/** 学习记录项（与后端 StudyRecordDTO 对齐） */
export interface StudyRecordItem {
  record_id?: number
  student_id?: number
  course_id?: number
  chapter_id?: number | null
  study_duration?: number
  progress?: number
  study_date?: string
  course_name?: string
  chapter_title?: string | null
}

/** 学习记录分页 */
export interface StudyRecordsData {
  records: StudyRecordItem[]
  page?: number
  pages?: number
  total?: number
}

/** 按天学习统计（与后端 StudyDailyStatsDTO 对齐） */
export interface StudyStats {
  days: number
  labels: string[]
  data: number[]
  total_minutes: number
  active_days: number
}

/** 我的课程条目（ADR-0017：course_progress 超集，含最后学习位置） */
export interface StudentCourseItem {
  course_id: number
  course_name: string
  cover?: string
  specialty_id?: number | null
  level_id?: number | null
  progress?: number
  completed_chapters?: number
  total_chapters?: number
  study_duration?: number
  last_chapter_id?: number | null
  last_chapter_title?: string
  last_position?: number
  last_studied_at?: string
}

/** 我的课程响应（continue_learning 为最后学习时间最新的课程，无记录时 null） */
export interface StudentCoursesData {
  courses: StudentCourseItem[]
  continue_learning: StudentCourseItem | null
}

/** 单课程章节学习状态 */
export interface StudentChapterProgress {
  chapter_id: number
  title: string
  progress?: number
  video_position?: number
  completed?: boolean
}

/** 单课程学习详情（我的课程条目 + 每章状态） */
export interface StudentCourseDetail extends StudentCourseItem {
  chapters: StudentChapterProgress[]
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
  },

  // 我的课程（含继续学习 top1，ADR-0017）
  getStudentCourses() {
    return unwrappedRequest.get<StudentCoursesData>('/student/courses', { timeout: DASHBOARD_TIMEOUT })
  },

  // 单课程学习详情（每章进度/播放位置/完成状态，ADR-0017）
  getStudentCourseDetail(courseId: number) {
    return unwrappedRequest.get<StudentCourseDetail>(`/student/courses/${courseId}`, { timeout: DASHBOARD_TIMEOUT })
  }
}
