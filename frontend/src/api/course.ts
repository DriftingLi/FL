import request from './request'

export interface UpdateProgressPayload {
  progress?: number
  study_duration?: number
  chapter_id?: number
  duration?: number
  [key: string]: unknown
}

/** 课程摘要（列表项） */
export interface CourseSummary {
  course_id: number
  name: string
  category?: string
  cover_image?: string
  description?: string
  duration?: number
  chapter_count?: number
  status?: number
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
  getCourses(params: { page?: number; page_size?: number; category?: string; keyword?: string }) {
    return request.get<{ courses: CourseSummary[]; total: number }>('/courses', { params })
  },

  getCourseDetail(id: number) {
    return request.get<{ course_info?: CourseSummary; chapters?: CourseChapter[] }>(`/course/${id}`)
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
