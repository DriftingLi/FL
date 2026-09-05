import { unwrappedRequest } from './request'

/**
 * 每日打卡独立模块（ADR-0028：从论坛域迁出，路由 /api/check-in/*）。
 * Web 端唯一消费方；uni-app-x 移动端适配另见 GitHub #587。
 */

export interface CheckInResult {
  checked: boolean
  streak: number
  total: number
  today_checked: boolean
  /** 今日实发积分（基础 + 跨档阶梯，合并单笔；重复打卡/已打卡为 0） */
  points: number
}

export interface CheckInDay {
  date: string
  checked: boolean
  /** 该日实发积分（无流水的历史打卡为 0） */
  points: number
}

export interface CheckInCalendarResult {
  days: CheckInDay[]
  streak: number
  total: number
  today_checked: boolean
}

export interface CheckInRankItem {
  rank: number
  user: { user_id: number; username: string; avatar_url: string }
  total: number
  streak: number
  today_checked: boolean
}

export interface CheckInRankMe {
  rank: number
  total: number
  streak: number
  today_checked: boolean
}

export interface CheckInRankResult {
  items: CheckInRankItem[]
  total: number
  page: number
  pages: number
  me: CheckInRankMe | null
}

// 静默头：打卡调用不依赖请求壳统一 toast（页面自有反馈）。
const SILENT = { headers: { 'X-Silent': '1' } }

export const checkInApi = {
  /** 打卡（幂等；首签直记积分，返回今日实发分） */
  checkIn() {
    return unwrappedRequest.post<CheckInResult>('/check-in', undefined, SILENT)
  },
  /** 日历（按月；逐日 {date, checked, points}） */
  getCalendar(params: { year: number; month: number }) {
    return unwrappedRequest.get<CheckInCalendarResult>('/check-in/calendar', { params, ...SILENT })
  },
  /** 排行榜（累计总榜） */
  getRank(params: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<CheckInRankResult>('/check-in/rank', { params, ...SILENT })
  }
}
