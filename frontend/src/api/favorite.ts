import { unwrappedRequest } from './request'
import type { FavoriteCheckDTO, FavoriteDTO, FavoritePageResult } from './generated/favorite'

/**
 * 收藏对象类型（与后端 ADR-0018 多态收藏一致）。
 *
 * 注解层只有 string（枚举词汇尚未进注解，ADR-0048 片一已知限制），故这里保留 UI 侧窄化联合，
 * 由消费处按值断言；生成物 FavoriteDTO.target_type 渲染为 string。
 */
export type FavoriteTargetType = 'course' | 'chapter' | 'question' | 'featured' | 'topic'

/** 收藏条目（列表项，含目标快照；目标已删除的条目后端不返回）——响应形状来自生成物（ADR-0048）。 */
export type FavoriteItem = FavoriteDTO

/** 收藏列表响应 */
export type FavoriteListData = FavoritePageResult

/** 收藏状态查询响应 */
export type FavoriteCheckData = FavoriteCheckDTO

export const favoriteApi = {
  // 入参（query）类型保留手写：ADR-0048 决策 3 不生成入参类型。
  list(params: { target_type?: FavoriteTargetType; page?: number; page_size?: number; credential_id?: number }) {
    // credential_id 由主 client 请求拦截器默认注入（#387）
    return unwrappedRequest.get<FavoriteListData>('/favorites', { params })
  },

  add(data: { target_type: FavoriteTargetType; target_id: number }) {
    return unwrappedRequest.post<FavoriteItem>('/favorites', data)
  },

  remove(favoriteId: number) {
    return unwrappedRequest.delete<null>(`/favorites/${favoriteId}`)
  },

  check(params: { target_type: FavoriteTargetType; target_id: number }) {
    return unwrappedRequest.get<FavoriteCheckData>('/favorites/check', { params })
  }
}
