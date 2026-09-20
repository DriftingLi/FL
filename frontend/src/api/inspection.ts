// 管理端巡检面（ADR-0053 §7：页面不再直连请求层，端点与筛选参数的知识收敛到本模块）。
//
// 类型说明（ADR-0056 §11 / issue #1100）：本模块的 8 个端点**全部**在契约生成面内，行/响应类型
// 的**缺省值**取自 generated 产物（泛型参数保留，调用方仍可显式给类型 —— 页面侧旧类型因此不需要
// 跟着改，页面换生成物属 PR-2 的面）：
//   - 4 个职位治理端点（/admin/jobs、/admin/job-reports 及其处置动作）→ generated/job.ts；
//   - 4 个巡检端点（积分流水 / 删除计数 / 简历留痕 / 联系方式申请）→ generated/inspection.ts。
// 后者由 #1097 在 internal/api/admin_inspection.go 补齐 swagger 注解、并在 apitypes.Domains 新建
// inspection 域登记，原先 4 条 ALLOWLIST 欠条已随之销账（见 scripts/check-api-consumers.mjs 头部留痕）。
import { unwrappedRequest } from './request'
// 票 6（ADR-0060 决策 6）：分页容器不再由本模块自备一份 `PagedResult` 副本，统一吃 api 侧的
// `Page`；本模块的 5 条分页端点后端回的就是 items + total，故 `toPage` 只做兜底归一。
import { toPage, type Page } from './page'
import type {
  ContactRequestRowDTO,
  InspectionCountDTO,
  PointsLedgerItem,
  RecruitResumeViewDTO
} from '@/api/generated/inspection'
import type { JobPostingDTO, ReportDTO } from '@/api/generated/job'

/** X-Silent：巡检面多为轮询/汇总型加载，失败不弹全局 toast（由调用方自行分类呈现）。 */
const SILENT = { headers: { 'X-Silent': '1' } }

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
  /** 积分流水分页。行类型缺省 = 生成物的 PointsLedgerItem；出口 = 中立容器 Page<T>。 */
  async pointsLedger<T = PointsLedgerItem>(params: PointsLedgerParams): Promise<Page<T>> {
    const res = await unwrappedRequest.get<Page<T>>('/admin/points/ledger', { params, ...SILENT })
    return toPage(res?.items, res?.total)
  },
  /** 「已受理后又被删除」计数。响应类型 = 生成物的 InspectionCountDTO（与 CountResult 同形）。 */
  deletedAfterAccepted() {
    return unwrappedRequest.get<InspectionCountDTO>('/admin/inspection/deleted-after-accepted', SILENT)
  },
  /** 简历查看留痕分页（#418，只呈现事实字段，不含学员明文联系方式）。行类型缺省 = RecruitResumeViewDTO。 */
  async resumeViews<T = RecruitResumeViewDTO>(params: PageParams): Promise<Page<T>> {
    const res = await unwrappedRequest.get<Page<T>>('/admin/recruit/views', { params, ...SILENT })
    return toPage(res?.items, res?.total)
  },
  /** 联系方式交换申请分页（#418）。行类型缺省 = ContactRequestRowDTO。 */
  async contactRequests<T = ContactRequestRowDTO>(params: PageParams): Promise<Page<T>> {
    const res = await unwrappedRequest.get<Page<T>>('/admin/recruit/requests', { params, ...SILENT })
    return toPage(res?.items, res?.total)
  },
  /** 职位巡检分页（#454，可按招聘者过滤）。行类型缺省 = 生成物的 JobPostingDTO。 */
  async jobs<T = JobPostingDTO>(params: PageParams): Promise<Page<T>> {
    const res = await unwrappedRequest.get<Page<T>>('/admin/jobs', { params, ...SILENT })
    return toPage(res?.items, res?.total)
  },
  /** 职位举报队列分页（#454）。行类型缺省 = 生成物的 ReportDTO。 */
  async jobReports<T = ReportDTO>(params: PageParams): Promise<Page<T>> {
    const res = await unwrappedRequest.get<Page<T>>('/admin/job-reports', { params, ...SILENT })
    return toPage(res?.items, res?.total)
  },
  /** 强制下架职位（带原因，邮件通知企业）。响应是下架后的职位行（生成物 JobPostingDTO）。 */
  forceOfflineJob(id: number, reason: string) {
    return unwrappedRequest.post<JobPostingDTO>(`/admin/jobs/${id}/force-offline`, { reason })
  },
  /** 把举报标记为已处理。响应是处理后的举报行（生成物 ReportDTO）。 */
  handleJobReport(id: number) {
    return unwrappedRequest.post<ReportDTO>(`/admin/job-reports/${id}/handle`)
  }
}
