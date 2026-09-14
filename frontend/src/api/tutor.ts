// 已迁移模块：走 unwrappedRequest（拦截器解包信封，成功直接返回业务数据 Promise<T>）。
//
// 响应类型**不再手写**：唯一事实源是后端注解 → backend/docs/swagger.json →
// `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3，issue #964 片六）。
// 本文件只留请求壳、端点装配与名称适配，入参（query / body）类型不生成、仍手写（决策 3）。
import { unwrappedRequest } from './request'
import type { AxiosProgressEvent } from 'axios'
import type {
  BatchDeleteFilesResult,
  ChapterDTO,
  ChapterDetailDTO,
  ChapterFileDTO,
  CourseDTO,
  CoursePageResult,
  DeleteFileResult,
  TutorCourseChaptersDTO
} from './generated/tutor'

export type {
  BatchDeleteFilesResult,
  ChapterDTO,
  ChapterDetailDTO,
  ChapterFileDTO,
  CourseDTO,
  CoursePageResult,
  DeleteFileResult,
  TutorCourseChaptersDTO
}

/** 查询入参（不生成，ADR-0048 决策 3） */
export interface TutorCoursesQuery {
  page?: number
  page_size?: number
  keyword?: string
}

/** 更新章节入参（不生成，ADR-0048 决策 3） */
export interface UpdateChapterPayload {
  title?: string
  content?: string
  content_type?: string
  file_url?: string
  description?: string
  duration?: number
  order_num?: number
}

/** 批量删除入参（不生成，ADR-0048 决策 3） */
export interface BatchDeleteFilesPayload {
  file_ids: number[]
}

/** 课程摘要（导师端）= 生成 CourseDTO 的子集，单一事实源派生 */
export type TutorCourse = Pick<
  CourseDTO,
  'course_id' | 'name' | 'cover_image' | 'description' | 'chapter_count' | 'specialty_id' | 'level_id'
>

/** 导师章节 = 生成 ChapterDTO 的子集，单一事实源派生 */
export type TutorChapter = Pick<ChapterDTO, 'chapter_id' | 'title' | 'content' | 'order_num' | 'duration'>

/**
 * 导师章节详情 = 生成 ChapterDetailDTO（章节字段平铺 + files + 上下章ID）。
 * 旧手写版额外声明了 `course` / `chapters` 两个**后端从不返回**的字段（页面也未消费）——
 * 字段级差异清单第 ② 类「手写类型过时」，此处收敛为生成形状（保留旧名不破 import 路径）。
 */
export type TutorChapterDetail = ChapterDetailDTO

export const tutorApi = {
  getCourses(params: TutorCoursesQuery) {
    return unwrappedRequest.get<CoursePageResult>('/tutor/courses', { params })
  },

  getCourseChapters(courseId: number) {
    return unwrappedRequest.get<TutorCourseChaptersDTO>(`/tutor/course/${courseId}/chapters`)
  },

  getChapterDetail(chapterId: number) {
    return unwrappedRequest.get<ChapterDetailDTO>(`/tutor/chapter/${chapterId}`)
  },

  /** 上传章节文件：后端返回落库后的 ChapterFileDTO（此前手写为 {url,file_id}，与后端不符） */
  uploadChapterFile(chapterId: number, formData: FormData, onProgress: (event: AxiosProgressEvent) => void) {
    return unwrappedRequest.post<ChapterFileDTO>(`/tutor/chapter/${chapterId}/upload`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 300000,
      onUploadProgress: onProgress
    })
  },

  updateChapter(chapterId: number, data: UpdateChapterPayload) {
    return unwrappedRequest.put<ChapterDTO>(`/tutor/chapter/${chapterId}`, data)
  },

  /** 删除章节文件：后端返回 {file_id,deleted}（此前被当成无载荷） */
  deleteFile(fileId: number) {
    return unwrappedRequest.delete<DeleteFileResult>(`/tutor/file/${fileId}`)
  },

  /** 批量删除：后端返回 {success_count,failed_count,failed_ids}（此前被当成无载荷） */
  batchDeleteFiles(data: BatchDeleteFilesPayload) {
    return unwrappedRequest.post<BatchDeleteFilesResult>('/tutor/files/batch-delete', data)
  }
}
