// 已迁移模块：走 unwrappedRequest（拦截器解包信封，成功直接返回业务数据 Promise<T>，
// 业务失败抛错并统一 toast，调用方不再自检 res.code）
//
// 响应类型**不再手写**：唯一事实源是后端注解 → backend/docs/swagger.json →
// `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3，issue #964 片六）。
// 入参（body）类型不生成、仍手写（决策 3）。
import { unwrappedRequest } from './request'
import type {
  ChapterDTO,
  ChapterDetailDTO,
  ChapterFileDTO,
  ChapterSlidesDTO,
  CourseDTO,
  CourseDetailDTO,
  CoursePageResult,
  StudyProgressDTO
} from './generated/course'

export type {
  ChapterDTO,
  ChapterDetailDTO,
  ChapterFileDTO,
  ChapterSlidesDTO,
  CourseDTO,
  CourseDetailDTO,
  CoursePageResult,
  StudyProgressDTO
}

/** 进度上报入参（不生成，ADR-0048 决策 3） */
export interface UpdateProgressPayload {
  progress?: number
  study_duration?: number
  chapter_id?: number
  duration?: number
  /** 秒级时长（>0 时后端优先于 duration 分钟） */
  duration_seconds?: number
  /** 章节最后播放位置（秒） */
  video_position?: number
  /** 显式完成该章节（置 progress=100） */
  completed?: boolean
}

// 旧名保留为生成别名（既有 import 路径不破）。旧手写版把大量必填字段写成了可选，
// 生成形状以注解为准 —— 差异进字段级清单第 ② 类。
/** 课程摘要（列表项）= 生成 CourseDTO */
export type CourseSummary = CourseDTO
/** 章节 = 生成 ChapterDTO */
export type CourseChapter = ChapterDTO
/** 课程详情主体 = 生成 CourseDTO（/course/{id} 的 data.course_info） */
export type CourseDetail = CourseDTO
/** 学员端课程详情信封 = 生成 CourseDetailDTO（含 progress/is_enrolled/last_*） */
export type CourseDetailResponse = CourseDetailDTO
/** 章节文件 = 生成 ChapterFileDTO */
export type ChapterFile = ChapterFileDTO
/** 章节详情（含文件与前后章节导航）= 生成 ChapterDetailDTO */
export type ChapterDetail = ChapterDetailDTO

export const courseApi = {
  getCourses(params: { page?: number; page_size?: number; keyword?: string; credential_id?: number; specialty_id?: number; level_id?: number; filter?: 'hot' | 'featured' | 'all' }) {
    // 证件作用域（服务端 CredentialScoped 兜底）：显式 credential_id 优先，其次才是登录学员的当前证件；
    // /courses 是公开端点，匿名/非学员角色不兜底、按不分区处理 ⇒ 要按证件分区时由调用方显式传入
    // （客户端半边 #1106；客户端拦截器不再注入任何证件参数）。
    return unwrappedRequest.get<CoursePageResult>('/courses', { params })
  },

  getCourseDetail(id: number) {
    return unwrappedRequest.get<CourseDetailDTO>(`/course/${id}`)
  },

  /** 上报学习进度：后端返回 StudyProgressDTO（此前被当成无载荷） */
  updateProgress(courseId: number, data: UpdateProgressPayload) {
    return unwrappedRequest.post<StudyProgressDTO>(`/course/${courseId}/progress`, data)
  },

  getChapterDetail(courseId: number, chapterId: number) {
    return unwrappedRequest.get<ChapterDetailDTO>(`/course/${courseId}/chapter/${chapterId}`)
  },

  getChapterSlides(chapterId: number) {
    return unwrappedRequest.get<ChapterSlidesDTO>(`/chapter/${chapterId}/slides`)
  },

  /** 重新生成幻灯片：后端返回 ChapterSlidesDTO（此前被当成无载荷） */
  regenerateSlides(chapterId: number) {
    return unwrappedRequest.post<ChapterSlidesDTO>(`/chapter/${chapterId}/slides/regenerate`)
  }
}
