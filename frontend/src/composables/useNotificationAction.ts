// useNotificationAction：通知动作判定 module（Ticket #228）。
// deep module：小 interface（refreshUserInfo / throttle 时钟）藏「资料审核通过」判定、
// 60s 节流与 refreshUserInfo 同步，供 NotificationPanel 等薄 adapter 调用。
// 判定只依赖结构化 payload（type==='profile_review' && review_status==='approved'），
// 不读标题文案；节流窗口与现状 lastUserSync 一致（60s）。
import type { NotificationItem } from '@/api/notification'

export interface NotificationActionOptions {
  /** 审核通过后重新拉取 /auth/me 并合并昵称/头像到本地缓存 */
  refreshUserInfo: () => Promise<void>
  /** 节流窗口（毫秒），默认 60000，与现状 lastUserSync 60s 一致 */
  throttleMs?: number
  /** 时钟源：默认 Date.now，测试可注入假时钟 */
  now?: () => number
}

export function useNotificationAction(options: NotificationActionOptions) {
  const THROTTLE_MS = options.throttleMs ?? 60_000
  const now = options.now ?? Date.now
  let lastSync = 0

  /** 判定「资料审核通过」：type=profile_review 且 payload.review_status=approved */
  function isProfileApproved(item: Pick<NotificationItem, 'type' | 'payload'> | null | undefined): boolean {
    return item?.type === 'profile_review' && item?.payload?.review_status === 'approved'
  }

  async function refreshUserInfo() {
    try {
      await options.refreshUserInfo()
    } catch (e) {
      // 静默失败，下次轮询再同步（现状行为）
    }
  }

  /** 立即同步用户资料（用于面板打开/点击命中通过通知时） */
  function requestImmediateSync(): Promise<void> {
    return refreshUserInfo()
  }

  /** 60s 节流同步：窗口内多次调用只触发一次（现状 refreshUnread 轮询语义） */
  function requestThrottledSync(): Promise<void> {
    const t = now()
    if (t - lastSync < THROTTLE_MS) return Promise.resolve()
    lastSync = t
    return refreshUserInfo()
  }

  /** 列表/面板刷新：存在「未读」审核通过通知时立即同步（保持现状 refresh 行为） */
  function syncIfUnreadApproved(items: NotificationItem[]): Promise<void> {
    if (items.some((it: NotificationItem) => !it.is_read && isProfileApproved(it))) {
      return requestImmediateSync()
    }
    return Promise.resolve()
  }

  return {
    isProfileApproved,
    requestImmediateSync,
    requestThrottledSync,
    syncIfUnreadApproved
  }
}
