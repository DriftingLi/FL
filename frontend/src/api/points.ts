// 已迁移模块：走 unwrappedRequest（拦截器解包信封，成功直接返回业务数据 Promise<T>，
// 业务失败抛错并统一 toast，调用方不再自检 res.code）
//
// 响应类型**不再手写**：唯一事实源是后端注解 → backend/docs/swagger.json →
// `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3，issue #965 片七）。
// 本文件只留请求壳、端点装配与**名称适配**，入参（query / body）类型不生成、仍手写
// （ADR-0048 决策 3：swag 对 body 描述弱）。
//
// 别名规则（片一统一口径）：旧名与生成形状一致时保留旧名（`export type 旧名 = 生成名`）；
// 形状不同时删旧名、直接用生成名（差异逐条记于片七字段级差异清单）。
import { unwrappedRequest } from './request'
import type { TaskGroup, TaskStatus } from '@/utils/taskCenter'
import type {
  PointsBalanceResult,
  PointsClaimResult,
  PointsLedgerItem,
  PointsLedgerResult,
  PointsTaskItem as GeneratedPointsTaskItem,
  PointsTasksResult,
  RedeemResult
} from './generated/points'

export type {
  PointsBalanceResult as PointsBalance,
  PointsLedgerItem,
  PointsLedgerResult as PointsLedgerData,
  RedeemResult
}

/**
 * 任务三态与分组：注解层只到 `string`（swag 尚无 enum → TS 联合的渲染能力，
 * 见 ADR-0048 片一「已知限制」），故这里从生成物**收窄**而不是手写整份形状
 * —— 字段集仍由生成物唯一决定，仅 status / group 两个封闭值集在前端窄化。
 */
export type PointsTaskItem = Omit<GeneratedPointsTaskItem, 'status' | 'group'> & {
  status: TaskStatus
  group: TaskGroup
}

export type PointsTasksData = Omit<PointsTasksResult, 'tasks'> & { tasks: PointsTaskItem[] }
export type PointsClaimData = Omit<PointsClaimResult, 'task_status'> & { task_status: TaskStatus }

// 静默头（#409）：领取与任务/余额读取都不再由请求壳统一 toast——页面用自有错误态与
// 语义分级提示，避免「拦截器弹一次 + 页面弹一次」的双 toast。
const SILENT = { headers: { 'X-Silent': '1' } }

export const pointsApi = {
  getBalance() {
    return unwrappedRequest.get<PointsBalanceResult>('/points/balance', SILENT)
  },
  getLedger(params: { page?: number; page_size?: number; direction?: 'in' | 'out' }) {
    return unwrappedRequest.get<PointsLedgerResult>('/points/ledger', { params })
  },
  getTasks() {
    return unwrappedRequest.get<PointsTasksData>('/points/tasks', SILENT)
  },
  claim(code: string) {
    return unwrappedRequest.post<PointsClaimData>('/points/tasks/' + code + '/claim', undefined, SILENT)
  },
  redeemCourse(courseId: number) {
    return unwrappedRequest.post<RedeemResult>('/points/shop/course/' + courseId + '/redeem')
  },
  redeemShop(sku: string) {
    return unwrappedRequest.post<RedeemResult>('/points/shop/' + sku + '/redeem')
  },
}
