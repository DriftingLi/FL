// 已迁移模块：走 unwrappedRequest（拦截器解包信封，成功直接返回业务数据 Promise<T>，
// 业务失败抛错并统一 toast，调用方不再自检 res.code）
import { unwrappedRequest } from './request'
import { useCredentialStore } from '@/stores/credential'

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

/** 课程摘要（列表项，courseToDict 字段，与后端 CourseDTO 契约对齐） */
export interface CourseSummary {
  course_id: number
  name: string
  cover_image?: string
  description?: string
  duration?: number
  chapter_count?: number
  status?: number
  // ===== 培训目录扩展（LH-27/28）=====
  credential_id?: number | null
  credential?: { id: number; code: string; name: string; category: string; level: number | null }
  specialty_id?: number | null
  level_id?: number | null
  theory_hours?: number
  practice_hours?: number
  certificate_template_id?: number | null
  certificate_name?: string
  prerequisite_course_ids?: number[]
  sort_order?: number
  created_at?: string
  is_hot?: boolean
  is_featured?: boolean
  /** 学习人数（详情元数据，导师端列表展示用） */
  student_count?: number
}

/** 章节（课程详情内嵌，与后端 ChapterDTO 对齐） */
export interface CourseChapter {
  chapter_id: number
  title: string
  content?: string
  content_type?: string
  order_num?: number
  duration?: number
  course_id?: number
  file_url?: string
  description?: string
}

/** 课程详情主体（课程字段 + 嵌套 specialty/level/certificate_template/prerequisites） */
export interface CourseDetail extends CourseSummary {
  specialty?: { specialty_id: number; code?: string; name: string }
  level?: { level_id: number; code?: string; name: string }
  certificate_template?: {
    id: number
    code?: string
    name: string
    description?: string
    validity_days?: number
    template_url?: string
  }
  prerequisites?: { course_id: number; name: string }[]
  chapter_count?: number
  student_count?: number
  study_progress?: number
  chapters?: CourseChapter[]
}

/** 学员端课程详情响应（后端包一层 course_info；学习位置字段 ADR-0017） */
export interface CourseDetailResponse {
  course_info?: CourseDetail
  chapters?: CourseChapter[]
  progress?: number
  is_enrolled?: boolean
  completed_chapters?: number
  last_chapter_id?: number | null
  last_position?: number
  last_studied_at?: string
}

/** 章节文件（与后端 ChapterFileDTO 对齐） */
export interface ChapterFile {
  chapter_id?: number | null
  content_type?: string
  created_at?: string
  file_id?: number
  file_name?: string
  file_size?: number
  file_url?: string
}

/** 章节详情（含文件与前后章节导航） */
export interface ChapterDetail {
  chapter_id: number
  title: string
  content?: string
  course_id?: number
  content_type?: string
  file_url?: string
  description?: string
  duration?: number
  order_num?: number
  study_status?: string
  previous_chapter_id?: number | null
  next_chapter_id?: number | null
  files?: ChapterFile[]
}

export const courseApi = {
  getCourses(params: { page?: number; page_size?: number; keyword?: string; credential_id?: number; specialty_id?: number; level_id?: number; filter?: 'hot' | 'featured' | 'all' }) {
    try { const cred = useCredentialStore().current?.id; if (cred && !params.credential_id) (params as any).credential_id = cred } catch {}
    return unwrappedRequest.get<{ courses: CourseSummary[]; total: number }>('/courses', { params })
  },

  getCourseDetail(id: number) {
    return unwrappedRequest.get<CourseDetailResponse>(`/course/${id}`)
  },

  updateProgress(courseId: number, data: UpdateProgressPayload) {
    return unwrappedRequest.post<null>(`/course/${courseId}/progress`, data)
  },

  getChapterDetail(courseId: number, chapterId: number) {
    return unwrappedRequest.get<ChapterDetail>(`/course/${courseId}/chapter/${chapterId}`)
  },

  getChapterSlides(chapterId: number) {
    return unwrappedRequest.get<{ slides?: string[] }>(`/chapter/${chapterId}/slides`)
  },

  regenerateSlides(chapterId: number) {
    return unwrappedRequest.post<null>(`/chapter/${chapterId}/slides/regenerate`)
  }
}
