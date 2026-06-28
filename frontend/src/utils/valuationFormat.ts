// 通用格式化工具
import dayjs from 'dayjs'

/** 金额（万元）格式化：保留 2 位小数 + 单位 */
export function formatWan(value: number | undefined | null, digits = 2): string {
  if (value == null || Number.isNaN(value)) return '-'
  return `${value.toFixed(digits)} 万元`
}

/** 系数格式化：4 位小数 */
export function formatCoefficient(value: number | undefined | null, digits = 4): string {
  if (value == null || Number.isNaN(value)) return '-'
  return value.toFixed(digits)
}

/** 百分比（0~1 → 0%~100%） */
export function formatPercent(ratio: number | undefined | null, digits = 1): string {
  if (ratio == null || Number.isNaN(ratio)) return '-'
  return `${(ratio * 100).toFixed(digits)}%`
}

/** 整数（小时数） */
export function formatInt(value: number | undefined | null): string {
  if (value == null || Number.isNaN(value)) return '-'
  return Math.round(value).toLocaleString('zh-CN')
}

/** ISO 时间字符串 → yyyy-MM-dd HH:mm */
export function formatDateTime(iso?: string | null): string {
  if (!iso) return '-'
  return dayjs(iso).format('YYYY-MM-DD HH:mm')
}

/** 文件大小 → 自适应单位 */
export function formatBytes(bytes: number | undefined | null): string {
  if (bytes == null || bytes < 0) return '-'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`
}

/** 吨位格式化：保留 1 位小数 + 单位 */
export function formatTonnage(value: number | undefined | null): string {
  if (value == null || Number.isNaN(value)) return '-'
  return `${value.toFixed(1)} 吨`
}

/** 门架高度（mm）格式化：自动换算 m */
export function formatMastHeight(mm: number | undefined | null): string {
  if (mm == null || Number.isNaN(mm)) return '-'
  if (mm >= 1000) return `${(mm / 1000).toFixed(2)} m（${mm} mm）`
  return `${mm} mm`
}

/** 布尔值 → 是/否 */
export function formatBoolean(v: boolean | undefined | null): string {
  if (v == null) return '-'
  return v ? '是' : '否'
}

/** 通用数字格式化：可指定小数位 */
export function formatNumber(value: number | undefined | null, digits = 2): string {
  if (value == null || Number.isNaN(value)) return '-'
  return value.toFixed(digits)
}
