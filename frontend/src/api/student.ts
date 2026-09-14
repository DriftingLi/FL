// 已迁移模块：响应类型**不再手写**，唯一事实源是后端注解 → backend/docs/swagger.json →
// `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3，issue #964 片六）。
// 入参（query）类型不生成、仍手写（决策 3）。
import { unwrappedRequest } from './request'
import type {
  CourseProgressDTO,
  StudentCourseChapterDTO,
  StudentCourseDTO,
  StudentCourseDetailDTO,
  StudentCoursesDTO,
  StudentProfileDTO,
  StudyDailyStatsDTO,
  StudyRecordDTO,
  StudyRecordPageResult,
  StudyStatsDTO
} from './generated/student'

export type {
  CourseProgressDTO,
  StudentCourseChapterDTO,
  StudentCourseDTO,
  StudentCourseDetailDTO,
  StudentCoursesDTO,
  StudentProfileDTO,
  StudyDailyStatsDTO,
  StudyRecordDTO,
  StudyRecordPageResult,
  StudyStatsDTO
}

// 旧名保留为生成类型别名（既有 import 路径不破）。旧手写版把「可空/必填」写反
// （字段几乎全可选），生成形状以注解为准，差异进字段级清单第 ② 类。
export type StudyStatSummary = StudyStatsDTO
export type CourseProgressItem = CourseProgressDTO
export type StudyRecordItem = StudyRecordDTO
export type StudyRecordsData = StudyRecordPageResult
export type StudyStats = StudyDailyStatsDTO
export type StudentCourseItem = StudentCourseDTO
export type StudentCoursesData = StudentCoursesDTO
export type StudentChapterProgress = StudentCourseChapterDTO
export type StudentCourseDetail = StudentCourseDetailDTO

// 旧名 StudentProfile **不保留**：手写形状是「扁平 user 字段 + study_stats」，后端实际是
// {student_info, study_stats, course_progress} 信封 —— 形状不同，按 ADR-0048 决策 4 删旧名，
// 直接用生成名 StudentProfileDTO（无外部消费点）。

const DASHBOARD_TIMEOUT = 45000

export const studentApi = {
  getProfile() {
    return unwrappedRequest.get<StudentProfileDTO>('/student/profile', { timeout: DASHBOARD_TIMEOUT })
  },

  getRecords(params: { page?: number; page_size?: number; start_date?: string; end_date?: string }) {
    return unwrappedRequest.get<StudyRecordPageResult>('/student/records', { params, timeout: DASHBOARD_TIMEOUT })
  },

  // 学习统计（按天分组），days=7|30
  getStudyStats(params?: { days?: number }) {
    return unwrappedRequest.get<StudyDailyStatsDTO>('/student/study-stats', { params, timeout: DASHBOARD_TIMEOUT })
  },

  // 我的课程（含继续学习 top1，ADR-0017）
  getStudentCourses() {
    return unwrappedRequest.get<StudentCoursesDTO>('/student/courses', { timeout: DASHBOARD_TIMEOUT })
  },

  // 单课程学习详情（每章进度/播放位置/完成状态，ADR-0017）
  getStudentCourseDetail(courseId: number) {
    return unwrappedRequest.get<StudentCourseDetailDTO>(`/student/courses/${courseId}`, { timeout: DASHBOARD_TIMEOUT })
  }
}
