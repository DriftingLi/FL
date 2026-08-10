import dayjs from 'dayjs'

/** 秒 → MM:SS 时钟（倒计时/时长展示） */
export function formatClock(seconds: number): string {
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

/** ISO 时间 → yyyy-MM-dd HH:mm；空值/非法返回 fallback */
export function formatDateTime(iso?: string | null, fallback = '-'): string {
  if (!iso) return fallback
  const d = dayjs(iso)
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm') : fallback
}

/** ISO 时间 → 相对时间：刚刚 / x 分钟前 / x 小时前 / MM-DD HH:mm */
export function formatRelativeTime(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (isNaN(d.getTime())) return iso
  const diff = Date.now() - d.getTime()
  if (diff < 60 * 1000) return '刚刚'
  if (diff < 60 * 60 * 1000) return `${Math.floor(diff / 60000)} 分钟前`
  if (diff < 24 * 60 * 60 * 1000) return `${Math.floor(diff / 3600000)} 小时前`
  return d.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

/** ISO 时间 → zh-CN 本地化日期时间；空值返回 fallback，非法返回原文 */
export function formatLocaleDateTime(iso: string, fallback = '-'): string {
  if (!iso) return fallback
  const d = new Date(iso)
  if (isNaN(d.getTime())) return iso
  return d.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

/** ISO 时间 → M/D HH:mm 短格式 */
export function formatShortDateTime(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  return `${d.getMonth() + 1}/${d.getDate()} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

/** 完整日期时间（formatDateTime 的历史名称，语义一致） */
export const formatTime = formatDateTime
