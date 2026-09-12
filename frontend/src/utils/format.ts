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

/**
 * 业务时区（Asia/Shanghai）自然日字符串 yyyy-MM-dd（打卡/连续签到等跨日边界口径）。
 * 与后端 clock.DayKey 同口径：本地 UTC 偏移不能代表上海自然日，统一走 Intl 时区。
 */
export function shanghaiDateStr(d: Date): string {
  return new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai' }).format(d)
}

/**
 * 时长格式化（紧凑）：90 → "1h30m"，60 → "1h"，非正数 → "0分钟"。
 * 与 formatDurationCn 是**两个不同语义的格式**（勿因同名而合并，见 #796 的教训）。
 */
export function formatDurationCompact(minutes: number): string {
  if (!minutes || minutes <= 0) return '0分钟'
  const hours = Math.floor(minutes / 60)
  const mins = minutes % 60
  if (hours > 0) {
    return mins > 0 ? `${hours}h${mins}m` : `${hours}h`
  }
  return `${mins}m`
}

/**
 * 时长格式化（中文）：90 → "1小时30分钟"，60 → "1小时"，非正数 → "0分钟"。
 * 与 formatDurationCompact 的差异是业务口径（后台报表用中文、图表轴用紧凑）。
 */
export function formatDurationCn(minutes: number): string {
  if (!minutes || minutes <= 0) return '0分钟'
  const hours = Math.floor(minutes / 60)
  const mins = minutes % 60
  if (hours > 0) {
    return mins > 0 ? `${hours}小时${mins}分钟` : `${hours}小时`
  }
  return `${mins}分钟`
}

/**
 * 学习进度 → 颜色（统计页进度条/图表）。
 * 注意：两档为既有裸 hex（EP 出厂警告色/危险色），迁移期保持值不变以免视觉回归（#796）。
 */
export function getProgressColor(progress: number): string {
  if (progress >= 100) return 'var(--color-success)'
  if (progress >= 60) return 'var(--color-primary-500)'
  if (progress >= 30) return '#e6a23c'
  return '#f56c6c'
}
