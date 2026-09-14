// 已迁移模块：走 unwrappedRequest（拦截器解包信封，成功直接返回业务数据 Promise<T>，
// 业务失败抛错并统一 toast，调用方不再自检 res.code）
import { unwrappedRequest } from './request'
import type { NotificationDTO, NotificationListPageResult, NotificationUnreadCountDTO } from './generated/notification'

/**
 * 通知结构化标记（后端 JSONB payload，加性字段：资料审核 review_status、论坛事件 topic_id、采纳 reply_id/points/reason）。
 *
 * ADR-0048 决策 6：站内信 JSONB 落库 payload **不在定型范围**（注解层用 swaggertype 钉成不透明
 * object，生成物渲染 Record<string, unknown>），故这里保留唯一的 UI 侧收窄类型，
 * NotificationItem 由生成类型 Omit 掉 payload 后再挂上它。
 */
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

/** 通知条目：响应形状来自生成物，仅 payload 是 UI 收窄（非生成面）。 */
export type NotificationItem = Omit<NotificationDTO, 'payload'> & { payload?: NotificationPayload | null }

/** 通知列表响应（含未读数） */
export type NotificationListData = NotificationListPageResult

export const notificationApi = {
  list(params: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<NotificationListData>('/notifications', { params })
  },

  unreadCount() {
    return unwrappedRequest.get<NotificationUnreadCountDTO>('/notifications/unread-count')
  },

  markRead(id: number) {
    return unwrappedRequest.post<null>(`/notifications/${id}/read`)
  },

  markAllRead() {
    return unwrappedRequest.post<null>('/notifications/read-all')
  }
}
