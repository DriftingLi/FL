// 已迁移模块：走 unwrappedRequest（拦截器解包信封，成功直接返回业务数据 Promise<T>，
// 业务失败抛错并统一 toast，调用方不再自检 res.code）
import { unwrappedRequest } from './request'

/** 通知结构化标记（后端 JSONB payload，加性字段：资料审核 review_status、论坛事件 topic_id、采纳 reply_id/points/reason） */
export interface NotificationPayload {
  review_status?: 'approved' | 'rejected'
  /** 论坛事件通知（forum_reply / forum_report / forum_reply_deleted 等）关联帖子 ID */
  topic_id?: number
  /** 采纳通知：被采纳回答 ID */
  reply_id?: number
  /** 采纳通知：分值（40 / 5） */
  points?: number
  /** 采纳通知：流水原因 accepted_bonus / accept_action */
  reason?: string
}

export interface NotificationItem {
  id: number
  type: string
  title: string
  content: string
  link: string
  payload?: NotificationPayload | null
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
