import { unwrappedRequest } from './request'

/** 学习资料条目（chapter_file 聚合视图，与后端 ADR-0018 MaterialDTO 对齐） */
export interface MaterialItem {
  file_id: number
  chapter_id?: number | null
  chapter_title?: string
  course_id: number
  course_name: string
  file_name: string
  file_url: string
  content_type?: string
  file_size?: number
  created_at?: string
}

/** 资料列表响应 */
export interface MaterialListData {
  materials: MaterialItem[]
  total: number
  page: number
  pages: number
}

export const materialApi = {
  list(params: { course_id?: number; page?: number; page_size?: number }) {
    return unwrappedRequest.get<MaterialListData>('/materials', { params })
  }
}
