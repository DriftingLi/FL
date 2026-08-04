import request from './request'

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
    return request.get<NotificationListData>('/notifications', { params })
  },

  unreadCount() {
    return request.get<{ count: number }>('/notifications/unread-count')
  },

  markRead(id: number) {
    return request.post<null>(`/notifications/${id}/read`)
  },

  markAllRead() {
    return request.post<null>('/notifications/read-all')
  }
}
