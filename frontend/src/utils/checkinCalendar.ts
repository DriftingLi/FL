/**
 * 打卡日历三态判定纯函数（spec #599 / #614，根仓库 ADR-0028）
 *
 * 三态：'streak' 当前连续打卡段（主色实心）｜'past' 已断开的历史打卡（同色系浅底）｜'none' 未打卡/未来（灰底）。
 *
 * 定段基准（CONTEXT.md「每日打卡」口径）：连续段事实源在后端打卡服务，客户端不自行回溯判定连续——
 * 段尾 = 今日（已签）或昨日（今日未签），起点 = 段尾 −(streak−1)；streak<=0 无连续段。
 * 与移动端 training-app utils/checkinCalendar.uts 同名同签名，三态用例集两份测试互为镜像（改口径两份同改）。
 */

/** 打卡日历单日输入（与后端整月逐日契约 {date, checked, points} 结构兼容） */
export interface CheckInCalendarDayInput {
  date: string
  checked: boolean
}

/** 三态取值 */
export type CheckInDayState = 'streak' | 'past' | 'none'

/** 不足两位补零 */
function padZero(n: number): string {
  if (n < 10) return '0' + n
  return '' + n
}

/** 日期规范化：截取 YYYY-MM-DD（兼容 RFC3339 带时间后缀的返回） */
function normalizeDate(date: string): string {
  if (date.length > 10) return date.substring(0, 10)
  return date
}

/** 解析 YYYY-MM-DD，非法返回 null */
function parseDate(date: string): { y: number; m: number; d: number } | null {
  const parts = date.split('-')
  if (parts.length !== 3) return null
  const y = parseInt(parts[0])
  const m = parseInt(parts[1])
  const d = parseInt(parts[2])
  if (isNaN(y) || isNaN(m) || isNaN(d)) return null
  if (m < 1 || m > 12 || d < 1 || d > 31) return null
  return { y, m, d }
}

/** 前推/后推 n 天（本地时区日历运算），date 非法时原样返回 */
function shiftDate(date: string, deltaDays: number): string {
  const p = parseDate(date)
  if (p == null) return date
  const dt = new Date(p.y, p.m - 1, p.d + deltaDays)
  return padZero(dt.getFullYear()) + '-' + padZero(dt.getMonth() + 1) + '-' + padZero(dt.getDate())
}

/** YYYY-MM-DD → y*10000+m*100+d 的可比较数值（UTS/Kotlin 端无 String 序比较，双端统一数值定序） */
function dateOrder(date: string): number {
  const p = parseDate(date)
  if (p == null) return 0
  return p.y * 10000 + p.m * 100 + p.d
}

/**
 * 三态计算：返回与 days 等长的状态数组（'streak' | 'past' | 'none'）
 *
 * 段区间由入参 streak 派生：今日已签段尾=今日，今日未签段尾退到昨日（昨日也未签时后端 streak 应为 0），
 * 起点 = 段尾 −(streak−1)，跨月段天然正确；段成员 = 已打卡且落在段区间内。
 */
export function computeDayStates(days: CheckInCalendarDayInput[], today: string, streak: number): CheckInDayState[] {
  const checkedSet = new Map<string, boolean>()
  for (let i = 0; i < days.length; i++) {
    if (days[i].checked) {
      checkedSet.set(normalizeDate(days[i].date), true)
    }
  }
  let end = normalizeDate(today)
  if (streak > 0 && !checkedSet.has(end)) {
    // 今日未签：段尾退到昨日
    end = shiftDate(end, -1)
  }
  const active = streak > 0
  const endOrder = dateOrder(end)
  const startOrder = active ? dateOrder(shiftDate(end, -(streak - 1))) : 0

  const states: CheckInDayState[] = []
  for (let i = 0; i < days.length; i++) {
    if (!days[i].checked) {
      states.push('none')
      continue
    }
    const keyOrder = dateOrder(normalizeDate(days[i].date))
    const inSegment = active && keyOrder >= startOrder && keyOrder <= endOrder
    states.push(inSegment ? 'streak' : 'past')
  }
  return states
}
