import { unwrappedRequest } from './request'

/** 搜索类型（与后端 ADR-0018 一致；featured 在学员端称资讯） */
export type SearchType = 'course' | 'question' | 'content' | 'topic'

/** 搜索结果条目 */
export interface SearchItem {
  type: SearchType
  id: number
  title: string
  cover?: string
  summary?: string
}

/** 单分区（全部搜索时每类 top N + 总数） */
export interface SearchSection {
  items: SearchItem[]
  total: number
}

/** 全部搜索响应（type 缺省） */
export interface SearchAllResult {
  keyword: string
  courses: SearchSection
  questions: SearchSection
  contents: SearchSection
  topics: SearchSection
}

/** 指定类型搜索响应（分页） */
export interface SearchPageResult {
  keyword: string
  type: SearchType
  total: number
  page: number
  pages: number
  items: SearchItem[]
}

export const searchApi = {
  /** type 缺省返回各分区聚合，指定类型返回分页结果 */
  search(params: { keyword: string; type?: SearchType; page?: number; page_size?: number; credential_id?: number }) {
    // credential_id 由主 client 请求拦截器默认注入（#387）
    return unwrappedRequest.get<SearchAllResult | SearchPageResult>('/search', { params })
  }
}
