import { unwrappedRequest } from './request'

export interface PointsBalance {
  balance: number
  total_earned: number
}

export interface PointsLedgerItem {
  id: number
  delta: number
  reason: string
  ref_type: string
  ref_id: string
  created_at: string
}

export interface PointsLedgerData {
  items: PointsLedgerItem[]
  total: number
  page: number
  pages: number
}

export interface PointsTaskItem {
  code: string
  group: string
  title: string
  desc: string
  points: number
  status: string
  progress: number
  total: number
}

export interface PointsTasksData {
  tasks: PointsTaskItem[]
}

export const pointsApi = {
  getBalance() {
    return unwrappedRequest.get<PointsBalance>('/points/balance')
  },
  getLedger(params: { page?: number; page_size?: number }) {
    return unwrappedRequest.get<PointsLedgerData>('/points/ledger', { params })
  },
  getTasks() {
    return unwrappedRequest.get<PointsTasksData>('/points/tasks')
  },
  claim(code: string) {
    return unwrappedRequest.post<{ balance: number; total_earned: number; task_status: string }>(`/points/tasks/${code}/claim`)
  },
  redeemCourse(courseId: number) {
    return unwrappedRequest.post<{ balance: number; total_earned: number; sku: string; ref_id: string }>(`/points/shop/course/${courseId}/redeem`)
  },
  redeemShop(sku: string) {
    return unwrappedRequest.post<{ balance: number; total_earned: number; sku: string; ref_id: string }>(`/points/shop/${sku}/redeem`)
  },
}
