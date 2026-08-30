import { unwrappedRequest } from './request'

/** 收藏对象类型（与后端 ADR-0018 多态收藏一致） */
export type FavoriteTargetType = 'course' | 'chapter' | 'question' | 'featured' | 'topic'

/** 收藏条目（列表项，含目标快照；目标已删除的条目后端不返回） */
export interface FavoriteItem {
  favorite_id: number
  target_type: FavoriteTargetType
  target_id: number
  title?: string
  cover?: string
  created_at?: string
}

/** 收藏列表响应 */
export interface FavoriteListData {
  favorites: FavoriteItem[]
  total: number
  page: number
  pages: number
}

/** 收藏状态查询响应 */
export interface FavoriteCheckData {
  favorited: boolean
  favorite_id: number
}

export const favoriteApi = {
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
