// 岗位字典（ADR-0053 §7：页面不再直连请求层，端点知识收敛到本模块）。
//
// 响应类型**不手写**：唯一事实源是后端注解 → backend/docs/swagger.json →
// `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3）。
// 岗位端点（/positions 与 /admin/positions）是 ADR-0053 §7 补进契约的那两个。
// 入参（body）类型不生成、仍手写（ADR-0048 决策 3）。
import { unwrappedRequest } from './request'
import type { PositionDict, PositionListDTO } from './generated/training'

export type { PositionDict, PositionListDTO }

/** X-Silent：选项字典加载失败不弹全局 toast（由调用方自行分类呈现）。 */
const SILENT = { headers: { 'X-Silent': '1' } }

/** 创建/更新岗位入参（复用培训域 catalog 的既有语义：Code/Name 为空表示不改动）。 */
export interface PositionPayload {
  code: string
  name: string
  description?: string | null
  sort_order?: number | null
  status?: number | null
}

export const positionApi = {
  /** 公开岗位字典（仅启用项，学员端 / 招聘端共用）。silent 见 SILENT 说明。 */
  listPublic(opts?: { silent?: boolean }) {
    return unwrappedRequest.get<PositionListDTO>('/positions', opts?.silent ? SILENT : undefined)
  },
  /** 管理端岗位字典（含停用项）。 */
  listAdmin(opts?: { silent?: boolean }) {
    return unwrappedRequest.get<PositionListDTO>('/admin/positions', opts?.silent ? SILENT : undefined)
  },
  create(data: PositionPayload) {
    return unwrappedRequest.post<PositionDict>('/admin/position', data)
  },
  update(id: number, data: Partial<PositionPayload>) {
    return unwrappedRequest.put<PositionDict>(`/admin/position/${id}`, data)
  },
  remove(id: number) {
    return unwrappedRequest.delete<null>(`/admin/position/${id}`)
  },
  swap(id: number, swapWith: number) {
    return unwrappedRequest.put<null>(`/admin/position/${id}/sort`, { swap_with: swapWith })
  }
}
