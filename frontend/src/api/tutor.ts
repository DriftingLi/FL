import { unwrappedRequest } from './request'
import type { AxiosProgressEvent } from 'axios'

export interface TutorCoursesQuery {
  page?: number
  page_size?: number
  keyword?: string
}

export interface UpdateChapterPayload {
  title?: string
  content?: string
  content_type?: string
  content_url?: string
  file_url?: string
  description?: string
  duration?: number
  order_num?: number
}

export interface BatchDeleteFilesPayload {
  file_ids: number[]
}

/** 课程摘要（导师端） */
export interface TutorCourse {
  course_id: number
  name: string
  cover_image?: string
  description?: string
  chapter_count?: number
  specialty_id?: number | null
  level_id?: number | null
  [key: string]: unknown
}

/** 导师章节 */
export interface TutorChapter {
  chapter_id: number
  title: string
  content?: string
  order_num?: number
  duration?: number
  [key: string]: unknown
}

/** 导师章节详情（含课程与章节列表） */
export interface TutorChapterDetail {
  chapter_id: number
  title: string
  content?: string
  course?: { name?: string; [key: string]: unknown }
  chapters?: TutorChapter[]
  [key: string]: unknown
}

/** 阅卷统计（按天分组） */
export interface GradingStatsData {
  days: number
  labels: string[]
  data: number[]
  total_count: number
  active_days: number
  [key: string]: unknown
}

export const tutorApi = {
  getCourses(params: TutorCoursesQuery) {
    return unwrappedRequest.get<{ courses: TutorCourse[]; total: number }>('/tutor/courses', { params })
  },

  // 阅卷统计（按天分组），days=7|30
  getGradingStats(params?: { days?: number }) {
    return unwrappedRequest.get<GradingStatsData>('/tutor/grading-stats', { params })
  },

  getCourseChapters(courseId: number) {
    return unwrappedRequest.get<{ course?: TutorCourse; chapters?: TutorChapter[] }>(`/tutor/course/${courseId}/chapters`)
  },

  getChapterDetail(chapterId: number) {
    return unwrappedRequest.get<TutorChapterDetail>(`/tutor/chapter/${chapterId}`)
  },

  uploadChapterFile(chapterId: number, formData: FormData, onProgress: (event: AxiosProgressEvent) => void) {
    return unwrappedRequest.post<{ url?: string; file_id?: number }>(`/tutor/chapter/${chapterId}/upload`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 300000,
      onUploadProgress: onProgress
    })
  },

  updateChapter(chapterId: number, data: UpdateChapterPayload) {
    return unwrappedRequest.put<TutorChapter>(`/tutor/chapter/${chapterId}`, data)
  },

  deleteFile(fileId: number) {
    return unwrappedRequest.delete<null>(`/tutor/file/${fileId}`)
  },

  batchDeleteFiles(data: BatchDeleteFilesPayload) {
    return unwrappedRequest.post<null>('/tutor/files/batch-delete', data)
  }
}
