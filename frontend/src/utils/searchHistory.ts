// 本地搜索历史（ADR-0049 决策 7）：搜索历史是学员个人意图痕迹，**只留在端上**，不入服务端。
//
// 上限 10 条、去重前置（同词再次搜索提到最前）、支持单条删除与清空；与移动端口径一致。
// 存储不可用（隐私模式 / 配额满 / SSR）时全部方法静默降级为空操作——历史是锦上添花，
// 不允许它把搜索本身搞崩。

const STORAGE_KEY = 'search_history'

export const SEARCH_HISTORY_MAX = 10

function storage(): Storage | null {
  try {
    return typeof window === 'undefined' ? null : window.localStorage
  } catch {
    return null
  }
}

function write(list: string[]): void {
  const s = storage()
  if (!s) return
  try {
    s.setItem(STORAGE_KEY, JSON.stringify(list))
  } catch {
    // 配额满 / 被禁用：忽略
  }
}

/** 读取历史（新→旧）。数据损坏时返回空列表，不抛错。 */
export function loadSearchHistory(): string[] {
  const s = storage()
  if (!s) return []
  try {
    const raw = s.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter((v): v is string => typeof v === 'string' && v.trim() !== '').slice(0, SEARCH_HISTORY_MAX)
  } catch {
    return []
  }
}

/** 记一条历史：去重前置 + 截断到上限，返回新列表。 */
export function pushSearchHistory(keyword: string): string[] {
  const kw = keyword.trim()
  if (!kw) return loadSearchHistory()
  const next = [kw, ...loadSearchHistory().filter((v) => v !== kw)].slice(0, SEARCH_HISTORY_MAX)
  write(next)
  return next
}

/** 删除单条，返回新列表。 */
export function removeSearchHistory(keyword: string): string[] {
  const next = loadSearchHistory().filter((v) => v !== keyword)
  write(next)
  return next
}

/** 清空历史。 */
export function clearSearchHistory(): void {
  const s = storage()
  if (!s) return
  try {
    s.removeItem(STORAGE_KEY)
  } catch {
    // 忽略
  }
}
