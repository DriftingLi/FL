// 已迁移模块：走 unwrappedRequest（拦截器解包信封，成功直接返回业务数据 Promise<T>，
// 业务失败抛错并统一 toast，调用方不再自检 res.code）
import { unwrappedRequest } from './request'

export interface NotificationItem {
  id: number
  type: string
  title: string
  content: string
  link: string
  is_read: boolean
  created_at: string
  read_at?: string
}

export interface NotificationListData {
  total: number
  page: number
  pages: number
  unread_count: number
  items: NotificationItem[]
}

export const notificationApi = {
  list(params: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<NotificationListData>('/notifications', { params })
  },

  unreadCount() {
    return unwrappedRequest.get<{ count: number }>('/notifications/unread-count')
  },

  markRead(id: number) {
    return unwrappedRequest.post<null>(`/notifications/${id}/read`)
  },

  markAllRead() {
    return unwrappedRequest.post<null>('/notifications/read-all')
  }
}
