import type { ForumTopicItem } from '@/api/forum'

export interface ForumHistoryItem {
  id: number
  title: string
  excerpt: string
  author: {
    user_id: number
    username: string
    avatar_url: string
  }
  images_count: number
  view_count: number
  reply_count: number
  viewedAt: string
  deleted?: boolean
}

const MAX_HISTORY = 50

function storageKey(userId?: number | string): string {
  const suffix = userId != null && userId !== '' ? String(userId) : 'guest'
  return `forum:history:${suffix}`
}

function safeParse(raw: string | null): ForumHistoryItem[] {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? (parsed as ForumHistoryItem[]) : []
  } catch {
    return []
  }
}

export function getMaxHistory(): number {
  return MAX_HISTORY
}

export function loadHistory(userId?: number | string): ForumHistoryItem[] {
  try {
    return safeParse(localStorage.getItem(storageKey(userId)))
  } catch {
    return []
  }
}

export function saveHistory(items: ForumHistoryItem[], userId?: number | string): void {
  try {
    localStorage.setItem(storageKey(userId), JSON.stringify(items))
  } catch {
    // ignore quota
  }
}

export function pushHistory(item: Omit<ForumHistoryItem, 'viewedAt'>, userId?: number | string): ForumHistoryItem[] {
  const now = new Date().toISOString()
  const existing = loadHistory(userId)
  const filtered = existing.filter((h) => h.id !== item.id)
  const next: ForumHistoryItem = { ...item, viewedAt: now, deleted: false }
  const merged = [next, ...filtered].slice(0, MAX_HISTORY)
  saveHistory(merged, userId)
  return merged
}

export function removeHistoryItem(id: number, userId?: number | string): ForumHistoryItem[] {
  const next = loadHistory(userId).filter((h) => h.id !== id)
  saveHistory(next, userId)
  return next
}

export function clearHistory(userId?: number | string): ForumHistoryItem[] {
  saveHistory([], userId)
  return []
}

export function toHistoryItem(topic: ForumTopicItem): Omit<ForumHistoryItem, 'viewedAt'> {
  return {
    id: topic.id,
    title: topic.title,
    excerpt: topic.content ? topic.content.slice(0, 80) : '',
    author: {
      user_id: topic.author.user_id,
      username: topic.author.username,
      avatar_url: topic.author.avatar_url,
    },
    images_count: topic.images?.length || 0,
    view_count: topic.view_count,
    reply_count: topic.reply_count,
  }
}
