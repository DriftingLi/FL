import request from './request'

export interface UpdateProgressPayload {
  progress?: number
  study_duration?: number
  chapter_id?: number
  duration?: number
  [key: string]: unknown
}

/** 课程摘要（列表项，courseToDict 字段） */
export interface CourseSummary {
  course_id: number
  name: string
  category?: string
  cover_image?: string
  description?: string
  duration?: number
  chapter_count?: number
  status?: number
  // ===== 培训目录扩展（LH-27/28）=====
  specialty_id?: number | null
  level_id?: number | null
  theory_hours?: number
  practice_hours?: number
  certificate_template_id?: number | null
  created_at?: string
  [key: string]: unknown
}

/** 章节（课程详情内嵌） */
export interface CourseChapter {
  chapter_id: number
  title: string
  content?: string
  content_type?: string
  order_num?: number
  duration?: number
  [key: string]: unknown
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
  study_progress?: number
  [key: string]: unknown
}

/** 学员端课程详情响应（后端包一层 course_info） */
export interface CourseDetailResponse {
  course_info?: CourseDetail
  chapters?: CourseChapter[]
  progress?: number
  [key: string]: unknown
}

/** 章节详情（含文件与前后章节导航） */
export interface ChapterDetail {
  chapter_id: number
  title: string
  content?: string
  study_status?: string
  previous_chapter_id?: number | null
  next_chapter_id?: number | null
  files?: {
    content_type?: string
    file_url?: string
    url?: string
    [key: string]: unknown
  }[]
  [key: string]: unknown
}

export const courseApi = {
  getCourses(params: { page?: number; page_size?: number; category?: string; keyword?: string; specialty_id?: number; level_id?: number }) {
    return request.get<{ courses: CourseSummary[]; total: number }>('/courses', { params })
  },

  getCourseDetail(id: number) {
    return request.get<CourseDetailResponse>(`/course/${id}`)
  },

  updateProgress(courseId: number, data: UpdateProgressPayload) {
    return request.post<null>(`/course/${courseId}/progress`, data)
  },

  getChapterDetail(courseId: number, chapterId: number) {
    return request.get<ChapterDetail>(`/course/${courseId}/chapter/${chapterId}`)
  },

  getChapterSlides(chapterId: number) {
    return request.get<{ slides?: string[] }>(`/chapter/${chapterId}/slides`)
  },

  regenerateSlides(chapterId: number) {
    return request.post<null>(`/chapter/${chapterId}/slides/regenerate`)
  }
}
