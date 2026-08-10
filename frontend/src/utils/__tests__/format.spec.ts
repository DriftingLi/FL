// 时间格式化工具（format.ts）单测
import { describe, it, expect, vi, afterEach } from 'vitest'
import { formatClock, formatDateTime, formatRelativeTime, formatLocaleDateTime, formatShortDateTime, formatTime } from '../format'

describe('formatClock', () => {
  it('格式化秒数为 MM:SS', () => {
    expect(formatClock(0)).toBe('00:00')
    expect(formatClock(59)).toBe('00:59')
    expect(formatClock(60)).toBe('01:00')
    expect(formatClock(125)).toBe('02:05')
    expect(formatClock(3599)).toBe('59:59')
    expect(formatClock(3600)).toBe('60:00')
  })
})

describe('formatDateTime', () => {
  it('格式化 ISO 时间为 yyyy-MM-dd HH:mm', () => {
    expect(formatDateTime('2026-08-10T09:05:00')).toBe('2026-08-10 09:05')
  })

  it('空值返回默认 fallback', () => {
    expect(formatDateTime()).toBe('-')
    expect(formatDateTime(null)).toBe('-')
    expect(formatDateTime(undefined, '')).toBe('')
    expect(formatDateTime('', '--')).toBe('--')
  })

  it('非法时间返回 fallback', () => {
    expect(formatDateTime('not-a-date')).toBe('-')
    expect(formatDateTime('garbage', '')).toBe('')
  })
})

describe('formatTime（formatDateTime 兼容别名）', () => {
  it('与 formatDateTime 行为一致', () => {
    expect(formatTime('2026-08-10T09:05:00')).toBe(formatDateTime('2026-08-10T09:05:00'))
    expect(formatTime()).toBe('-')
  })
})

describe('formatRelativeTime', () => {
  afterEach(() => vi.useRealTimers())

  it('空值返回空串', () => {
    expect(formatRelativeTime('')).toBe('')
  })

  it('非法时间返回原文', () => {
    expect(formatRelativeTime('not-a-date')).toBe('not-a-date')
  })

  it('一分钟内显示"刚刚"', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-10T10:00:00Z'))
    expect(formatRelativeTime('2026-08-10T09:59:30Z')).toBe('刚刚')
  })

  it('一小时内显示 x 分钟前', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-10T10:00:00Z'))
    expect(formatRelativeTime('2026-08-10T09:30:00Z')).toBe('30 分钟前')
  })

  it('24 小时内显示 x 小时前', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-10T10:00:00Z'))
    expect(formatRelativeTime('2026-08-10T04:00:00Z')).toBe('6 小时前')
  })

  it('超过 24 小时显示本地日期时间', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-10T10:00:00Z'))
    const out = formatRelativeTime('2026-08-01T10:00:00Z')
    expect(out).toMatch(/08\/0[12]/)
  })
})

describe('formatLocaleDateTime', () => {
  it('空值返回默认 fallback，非法返回原文', () => {
    expect(formatLocaleDateTime('')).toBe('-')
    expect(formatLocaleDateTime('not-a-date')).toBe('not-a-date')
    expect(formatLocaleDateTime('', '--')).toBe('--')
  })

  it('格式化本地化日期时间', () => {
    const out = formatLocaleDateTime('2026-08-10T09:05:00Z')
    expect(out).toContain('2026')
    expect(out).toContain('08')
    expect(out).toContain('10')
  })
})

describe('formatShortDateTime', () => {
  it('空值返回空串', () => {
    expect(formatShortDateTime('')).toBe('')
  })

  it('格式化为 M/D HH:mm', () => {
    const d = new Date('2026-08-10T09:05:00Z')
    const expected = `${d.getMonth() + 1}/${d.getDate()} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
    expect(formatShortDateTime('2026-08-10T09:05:00Z')).toBe(expected)
  })
})
