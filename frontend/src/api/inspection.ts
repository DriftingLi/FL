// 管理端巡检面（ADR-0053 §7：页面不再直连请求层，端点与筛选参数的知识收敛到本模块）。
//
// 类型说明：本模块覆盖的端点**尚未进契约生成面**（它们的响应在此之前都是页面里的 `any`），
// 所以这里只提供**形状要点 + 调用签名**，行类型由调用方给出（`PagedResult<LedgerItem>`）——
// 不新造一套与后端注解平行的「手写契约」。这些端点补上注解与 apitypes 登记后，
// 再把本模块的返回类型替换为 generated 产物（与 api/position.ts 同一收口方式）。
import { unwrappedRequest } from './request'

/** X-Silent：巡检面多为轮询/汇总型加载，失败不弹全局 toast（由调用方自行分类呈现）。 */
const SILENT = { headers: { 'X-Silent': '1' } }

/** 分页信封要点（后端 items + total 约定）。 */
export interface PagedResult<T> {
  items: T[]
  total: number
}

/** 单值计数信封（如「已受理后删除」计数）。 */
export interface CountResult {
  count: number
}

/** 积分流水筛选轴（#411：默认锁定问答域，显式切换才跨域全量）。 */
export interface PointsLedgerParams {
  page: number
  page_size: number
  ref_type?: string
  reason?: string
  user_id?: string
}

/** 分页查询筛选轴。 */
export interface PageParams {
  page: number
  page_size: number
  recruiter_id?: string
}

export const inspectionApi = {
  /** 积分流水分页（行类型由调用方给出）。 */
  pointsLedger<T>(params: PointsLedgerParams) {
    return unwrappedRequest.get<PagedResult<T>>('/admin/points/ledger', { params, ...SILENT })
  },
  /** 「已受理后又被删除」计数。 */
  deletedAfterAccepted() {
    return unwrappedRequest.get<CountResult>('/admin/inspection/deleted-after-accepted', SILENT)
  },
  /** 简历查看留痕分页（#418，只呈现事实字段，不含学员明文联系方式）。 */
  resumeViews<T>(params: PageParams) {
    return unwrappedRequest.get<PagedResult<T>>('/admin/recruit/views', { params, ...SILENT })
  },
  /** 联系方式交换申请分页（#418）。 */
  contactRequests<T>(params: PageParams) {
    return unwrappedRequest.get<PagedResult<T>>('/admin/recruit/requests', { params, ...SILENT })
  },
  /** 职位巡检分页（#454，可按招聘者过滤）。 */
  jobs<T>(params: PageParams) {
    return unwrappedRequest.get<PagedResult<T>>('/admin/jobs', { params, ...SILENT })
  },
  /** 职位举报队列分页（#454）。 */
  jobReports<T>(params: PageParams) {
    return unwrappedRequest.get<PagedResult<T>>('/admin/job-reports', { params, ...SILENT })
  },
  /** 强制下架职位（带原因，邮件通知企业）。 */
  forceOfflineJob(id: number, reason: string) {
    return unwrappedRequest.post<null>(`/admin/jobs/${id}/force-offline`, { reason })
  },
  /** 把举报标记为已处理。 */
  handleJobReport(id: number) {
    return unwrappedRequest.post<null>(`/admin/job-reports/${id}/handle`)
  }
}
