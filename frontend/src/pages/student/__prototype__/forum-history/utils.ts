// PROTOTYPE — throwaway storage helpers for 论坛浏览记录 (localStorage MRU 50)
import type { ForumHistoryItem } from './types'

const MAX_HISTORY = 50
const STORAGE_KEY = 'forum:history:prototype'

export function getStorageKey(): string {
  return STORAGE_KEY
}

export function getMaxHistory(): number {
  return MAX_HISTORY
}

export function loadHistory(): ForumHistoryItem[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? (parsed as ForumHistoryItem[]) : []
  } catch {
    return []
  }
}

export function saveHistory(items: ForumHistoryItem[]): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(items))
  } catch {
    // ignore quota
  }
}

export function pushHistory(item: Omit<ForumHistoryItem, 'viewedAt'>): ForumHistoryItem[] {
  const now = new Date().toISOString()
  const existing = loadHistory()
  const filtered = existing.filter((h) => h.id !== item.id)
  const next: ForumHistoryItem = { ...item, viewedAt: now, deleted: false }
  const merged = [next, ...filtered].slice(0, MAX_HISTORY)
  saveHistory(merged)
  return merged
}

export function removeOne(id: number): ForumHistoryItem[] {
  const next = loadHistory().filter((h) => h.id !== id)
  saveHistory(next)
  return next
}

export function clearAll(): ForumHistoryItem[] {
  saveHistory([])
  return []
}

export function markDeleted(id: number): ForumHistoryItem[] {
  const list = loadHistory()
  const next = list.map((h) => (h.id === id ? { ...h, deleted: true } : h))
  saveHistory(next)
  return next
}

export function seedMockHistory(): ForumHistoryItem[] {
  const now = Date.now()
  const hoursAgo = (h: number) => new Date(now - h * 3600 * 1000).toISOString()
  const daysAgo = (d: number) => new Date(now - d * 24 * 3600 * 1000).toISOString()
  const seed: ForumHistoryItem[] = [
    {
      id: 101,
      title: '叉车制动系统异响怎么排查？',
      excerpt: '最近带学员实操时发现制动踏板有异响，踩下去有轻微金属摩擦声，已检查制动片磨损情况…',
      author: { user_id: 2, username: '李师傅', avatar_url: '' },
      images_count: 2,
      view_count: 342,
      reply_count: 18,
      viewedAt: hoursAgo(1),
    },
    {
      id: 102,
      title: 'N1 叉车司机证考试避坑指南',
      excerpt: '总结了去年考 N1 时的几个易错点：液压、安全操作、叉取姿势等，附高频考点整理…',
      author: { user_id: 5, username: '阿明', avatar_url: '' },
      images_count: 0,
      view_count: 892,
      reply_count: 43,
      viewedAt: hoursAgo(3),
    },
    {
      id: 103,
      title: '电动叉车电池保养日常',
      excerpt: 'LFP 电池的日常充电习惯分享，避免过放、均衡充电周期等，欢迎补充…',
      author: { user_id: 12, username: '电工老张', avatar_url: '' },
      images_count: 1,
      view_count: 210,
      reply_count: 7,
      viewedAt: hoursAgo(5),
    },
    {
      id: 104,
      title: '液压系统漏油现场记录',
      excerpt: '一台 3 吨叉车举升缓慢，现场发现液压缸密封处渗油，附照片与处置流程…',
      author: { user_id: 8, username: '维修小王', avatar_url: '' },
      images_count: 3,
      view_count: 156,
      reply_count: 12,
      viewedAt: daysAgo(1),
    },
    {
      id: 105,
      title: '新手第一次上车实操心得',
      excerpt: '今天第一次独立操作叉车，教练说我的转向还不够稳，记录一下感受…',
      author: { user_id: 21, username: '学员小陈', avatar_url: '' },
      images_count: 0,
      view_count: 98,
      reply_count: 9,
      viewedAt: daysAgo(2),
    },
    {
      id: 106,
      title: '叉车年检需要准备哪些资料？',
      excerpt: '年检快到了，整理了一份资料清单：行驶证、操作证、维保记录等…',
      author: { user_id: 3, username: '安保刘姐', avatar_url: '' },
      images_count: 0,
      view_count: 445,
      reply_count: 22,
      viewedAt: daysAgo(3),
    },
    {
      id: 107,
      title: '已删除的帖子示例',
      excerpt: '该主题已被管理员删除，仅在浏览记录中保留占位用于演示删除置灰与移除…',
      author: { user_id: 99, username: '已注销', avatar_url: '' },
      images_count: 0,
      view_count: 0,
      reply_count: 0,
      viewedAt: daysAgo(4),
      deleted: true,
    },
  ]
  saveHistory(seed)
  return seed
}
