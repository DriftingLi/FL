// 已迁移模块：响应类型**不再手写**，唯一事实源是后端注解 → backend/docs/swagger.json →
// `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3，issue #964 片六）。
// 入参（query）类型不生成、仍手写（决策 3）。
import { unwrappedRequest } from './request'
import type { SearchAllDTO, SearchItemDTO, SearchPageDTO, SearchSectionDTO } from './generated/search'

export type { SearchAllDTO, SearchItemDTO, SearchPageDTO, SearchSectionDTO }

/** 搜索类型（与后端 ADR-0018 一致；featured 在学员端称资讯）—— 封闭值集，入参用，不生成（决策 3） */
export type SearchType = 'course' | 'question' | 'content' | 'topic'

// 旧名保留为生成类型别名（既有 import 路径不破）；形状差异（cover/summary 由可选变必填）进字段级清单。
export type SearchItem = SearchItemDTO
export type SearchSection = SearchSectionDTO
export type SearchAllResult = SearchAllDTO
export type SearchPageResult = SearchPageDTO

/**
 * 搜索响应：**同一端点两种形状** —— type 缺省为各分区聚合（SearchAllDTO），
 * 指定 type 为分页结果（SearchPageDTO）。swag 无联合类型表达力（注解取聚合形状），
 * 该边界在注解 @Description 与本别名显式标注，消费处按 type 是否传入自行收窄。
 */
export type SearchResult = SearchAllResult | SearchPageResult

export const searchApi = {
  /** type 缺省返回各分区聚合，指定类型返回分页结果 */
  search(params: { keyword: string; type?: SearchType; page?: number; page_size?: number; credential_id?: number }) {
    // credential_id 由主 client 请求拦截器默认注入（#387）
    return unwrappedRequest.get<SearchResult>('/search', { params })
  }
}
