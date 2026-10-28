// 已迁移模块：走 unwrappedRequest（拦截器解包信封，成功直接返回业务负载 Promise<T>）。
//
// 响应类型**不再手写**：唯一事实源是后端注解 → backend/docs/swagger.json →
// `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3）。
//
// 本模块只放**管理端**两条（学员面提交/历史在移动端；Web 学员端没有反馈入口，不留死代码）。
import { unwrappedRequest } from './request'
import { toPage, type Page } from './page'
import type { FeedbackDTO, FeedbackListResult } from './generated/feedback'

export type { FeedbackDTO }

/** 列表查询参数：status 省略 = 全部（后端接受 pending / handled / 空）。 */
export interface FeedbackListParams {
  page?: number
  page_size?: number
  status?: string
}

export const feedbackApi = {
  /** 管理端反馈列表（分页容器由 api 层出口给，ADR-0060 决策 6）。 */
  async list(params: FeedbackListParams): Promise<Page<FeedbackDTO>> {
    const res = await unwrappedRequest.get<FeedbackListResult>('/admin/feedback', { params })
    return toPage(res?.items, res?.total)
  },
  /** 标记已处理（后端幂等：已处理再点返回同一行，不报错）。 */
  markHandled(id: number) {
    return unwrappedRequest.post<FeedbackDTO>(`/admin/feedback/${id}/handle`)
  }
}
