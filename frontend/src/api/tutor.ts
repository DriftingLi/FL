import request from './request'
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

export const tutorApi = {
  getCourses(params: TutorCoursesQuery) {
    return request.get('/tutor/courses', { params })
  },

  // 阅卷统计（按天分组），days=7|30
  getGradingStats(params?: { days?: number }) {
    return request.get('/tutor/grading-stats', { params })
  },

  getCourseChapters(courseId: number) {
    return request.get(`/tutor/course/${courseId}/chapters`)
  },

  getChapterDetail(chapterId: number) {
    return request.get(`/tutor/chapter/${chapterId}`)
  },

  uploadChapterFile(chapterId: number, formData: FormData, onProgress: (event: AxiosProgressEvent) => void) {
    return request.post(`/tutor/chapter/${chapterId}/upload`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 300000,
      onUploadProgress: onProgress
    })
  },

  updateChapter(chapterId: number, data: UpdateChapterPayload) {
    return request.put(`/tutor/chapter/${chapterId}`, data)
  },

  deleteFile(fileId: number) {
    return request.delete(`/tutor/file/${fileId}`)
  },

  batchDeleteFiles(data: BatchDeleteFilesPayload) {
    return request.post('/tutor/files/batch-delete', data)
  }
}
