import { unwrappedRequest } from './request'
import type { TaskGroup, TaskStatus } from '@/utils/taskCenter'

export interface PointsBalance {
  balance: number
  total_earned: number
  /** 累计支出（#509：delta<0 流水绝对值聚合） */
  total_spent: number
}

export interface PointsLedgerItem {
  id: number
  delta: number
  reason: string
  ref_type: string
  ref_id: string
  created_at: string
  /** 过期时间（#509 设计位）：首版恒 null（永久有效） */
  expires_at: string | null
}

export interface PointsLedgerData {
  items: PointsLedgerItem[]
  total: number
  page: number
  pages: number
}

export interface PointsTaskItem {
  code: string
  group: TaskGroup
  title: string
  desc: string
  points: number
  // 三态收编（#388/#409）：todo / claimable / claimed，契约中不存在独立 claimed 布尔位。
  status: TaskStatus
  progress: number
  total: number
}

export interface PointsTasksData {
  tasks: PointsTaskItem[]
}

export interface PointsClaimData {
  balance: number
  total_earned: number
  task_status: TaskStatus
}

// 静默头（#409）：领取与任务/余额读取都不再由请求壳统一 toast——页面用自有错误态与
// 语义分级提示，避免「拦截器弹一次 + 页面弹一次」的双 toast。
const SILENT = { headers: { 'X-Silent': '1' } }

export const pointsApi = {
  getBalance() {
    return unwrappedRequest.get<PointsBalance>('/points/balance', SILENT)
  },
  getLedger(params: { page?: number; page_size?: number; direction?: 'in' | 'out' }) {
    return unwrappedRequest.get<PointsLedgerData>('/points/ledger', { params })
  },
  getTasks() {
    return unwrappedRequest.get<PointsTasksData>('/points/tasks', SILENT)
  },
  claim(code: string) {
    return unwrappedRequest.post<PointsClaimData>('/points/tasks/' + code + '/claim', undefined, SILENT)
  },
  redeemCourse(courseId: number) {
    return unwrappedRequest.post<{ balance: number; total_earned: number; sku: string; ref_id: string }>('/points/shop/course/' + courseId + '/redeem')
  },
  redeemShop(sku: string) {
    return unwrappedRequest.post<{ balance: number; total_earned: number; sku: string; ref_id: string }>('/points/shop/' + sku + '/redeem')
  },
}