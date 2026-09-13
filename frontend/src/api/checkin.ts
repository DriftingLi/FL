import { unwrappedRequest } from './request'
import type {
  CheckInCalendarResult,
  CheckInDay,
  CheckInRankItem,
  CheckInRankResult,
  CheckInResult
} from './generated/checkin'

/**
 * 每日打卡独立模块（ADR-0028：从论坛域迁出，路由 /api/check-in/*）。
 * Web 端唯一消费方；uni-app-x 移动端适配另见 GitHub #587。
 *
 * 响应类型**不再手写**：唯一事实源是后端注解 → backend/docs/swagger.json →
 * `cd backend && go run ./cmd/gen-apitypes`（ADR-0019 契约 codegen 专项第一步 / spec #940 片五③）。
 * 本文件只保留请求壳与端点装配，是薄 adapter；生成物由后端 codegen_test.go 字节级钉住。
 */
export type { CheckInCalendarResult, CheckInDay, CheckInRankItem, CheckInRankResult, CheckInResult }

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
