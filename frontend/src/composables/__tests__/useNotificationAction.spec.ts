// useNotificationAction（Ticket #228）单测：
// 「资料审核通过」判定（仅依赖结构化 payload）+ 60s 节流 + refreshUserInfo 同步回调。
import { describe, it, expect, vi } from 'vitest'
import { useNotificationAction } from '../useNotificationAction'
import type { NotificationItem } from '@/api/notification'

function makeItem(partial: Partial<NotificationItem>): NotificationItem {
  return {
    id: 1,
    type: 'system',
    title: '标题',
    content: '内容',
    link: '',
    is_read: false,
    created_at: '2026-01-01T00:00:00Z',
    ...partial
  }
}

describe('useNotificationAction 判定', () => {
  it('profile_review + payload.review_status=approved 判定为资料审核通过', () => {
    const { isProfileApproved } = useNotificationAction({ refreshUserInfo: vi.fn() })
    const item = makeItem({ type: 'profile_review', payload: { review_status: 'approved' } })
    expect(isProfileApproved(item)).toBe(true)
  })

  it('profile_review + review_status=rejected 不算通过', () => {
    const { isProfileApproved } = useNotificationAction({ refreshUserInfo: vi.fn() })
    const item = makeItem({ type: 'profile_review', payload: { review_status: 'rejected' } })
    expect(isProfileApproved(item)).toBe(false)
  })

  it('无 payload 或非 profile_review 类型不算通过（不依赖标题文案）', () => {
    const { isProfileApproved } = useNotificationAction({ refreshUserInfo: vi.fn() })
    expect(isProfileApproved(makeItem({ type: 'profile_review' }))).toBe(false)
    expect(isProfileApproved(makeItem({ type: 'profile_review', payload: null }))).toBe(false)
    expect(isProfileApproved(makeItem({ type: 'system', title: '通过', payload: { review_status: 'approved' } }))).toBe(false)
    expect(isProfileApproved(null)).toBe(false)
    expect(isProfileApproved(undefined)).toBe(false)
  })
})

describe('useNotificationAction 60s 节流', () => {
  it('窗口内多次 requestThrottledSync 只刷新一次，跨窗口再刷新', async () => {
    const refreshUserInfo = vi.fn().mockResolvedValue(undefined)
    let fakeNow = 0
    const { requestThrottledSync } = useNotificationAction({
      refreshUserInfo,
      now: () => fakeNow
    })

    await requestThrottledSync()
    expect(refreshUserInfo).toHaveBeenCalledTimes(1)

    // 30s 内再次调用：节流不触发
    fakeNow += 30_000
    await requestThrottledSync()
    expect(refreshUserInfo).toHaveBeenCalledTimes(1)

    // 超过 60s 窗口：再次刷新
    fakeNow += 31_000 // 累计 61s
    await requestThrottledSync()
    expect(refreshUserInfo).toHaveBeenCalledTimes(2)
  })

  it('refreshUserInfo 抛错时静默失败，不阻断再次同步', async () => {
    const refreshUserInfo = vi.fn().mockRejectedValue(new Error('network'))
    let fakeNow = 0
    const { requestThrottledSync } = useNotificationAction({
      refreshUserInfo,
      now: () => fakeNow
    })
    await requestThrottledSync()
    fakeNow += 60_001
    await requestThrottledSync()
    expect(refreshUserInfo).toHaveBeenCalledTimes(2)
  })

  it('requestImmediateSync 每次立即刷新，不受节流限制', async () => {
    const refreshUserInfo = vi.fn().mockResolvedValue(undefined)
    let fakeNow = 0
    const { requestImmediateSync } = useNotificationAction({
      refreshUserInfo,
      now: () => fakeNow
    })
    await requestImmediateSync()
    await requestImmediateSync()
    expect(refreshUserInfo).toHaveBeenCalledTimes(2)
  })
})

describe('useNotificationAction syncIfUnreadApproved', () => {
  it('存在未读审核通过通知时同步用户资料', async () => {
    const refreshUserInfo = vi.fn().mockResolvedValue(undefined)
    const { syncIfUnreadApproved } = useNotificationAction({ refreshUserInfo })
    await syncIfUnreadApproved([
      makeItem({ id: 1, type: 'profile_review', payload: { review_status: 'approved' }, is_read: false })
    ])
    expect(refreshUserInfo).toHaveBeenCalledTimes(1)
  })

  it('无未读审核通过通知时不触发同步', async () => {
    const refreshUserInfo = vi.fn().mockResolvedValue(undefined)
    const { syncIfUnreadApproved } = useNotificationAction({ refreshUserInfo })
    await syncIfUnreadApproved([
      makeItem({ id: 1, type: 'profile_review', payload: { review_status: 'approved' }, is_read: true }),
      makeItem({ id: 2, type: 'profile_review', payload: { review_status: 'rejected' }, is_read: false }),
      makeItem({ id: 3, type: 'system' })
    ])
    expect(refreshUserInfo).not.toHaveBeenCalled()
  })
})
