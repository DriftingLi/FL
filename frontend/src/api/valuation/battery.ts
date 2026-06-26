// 电池 RUL 评估 API（独立模块，与 evaluation.ts 物理隔离）
import client from './client'
import type {
  CreateBatteryRequest,
  BatteryEvaluationDetail,
  BatteryEvaluationListItem,
  BatteryListResponse,
  CreateBatteryResponse,
  BatteryReportResponse
} from '@/types/valuation/battery'

/** 提交电池循环数据并预测 RUL */
export async function createBatteryEvaluation(req: CreateBatteryRequest): Promise<CreateBatteryResponse> {
  const resp = await client.post<unknown, { data: CreateBatteryResponse }>('/battery/evaluations', req)
  return resp.data
}

/** 列表查询（支持 ?battery_type=lfp&page=1&page_size=20） */
export async function listBatteryEvaluations(params: {
  battery_type?: string
  page?: number
  page_size?: number
}): Promise<BatteryListResponse> {
  const resp = await client.get<unknown, { data: BatteryListResponse }>('/battery/evaluations', { params })
  return resp.data
}

/** 详情查询（含 cycle_features 数组） */
export async function getBatteryEvaluation(id: number): Promise<BatteryEvaluationDetail> {
  const resp = await client.get<unknown, { data: BatteryEvaluationDetail }>(`/battery/evaluations/${id}`)
  return resp.data
}

/** 触发后端生成 PDF 报告 */
export async function generateBatteryReport(id: number): Promise<BatteryReportResponse> {
  const resp = await client.post<unknown, { data: BatteryReportResponse }>(
    `/battery/evaluations/${id}/report`
  )
  return resp.data
}

/** 下载 PDF 二进制流（返回 Blob，前端用 a.download 触发下载） */
export async function downloadBatteryReportBlob(id: number): Promise<Blob> {
  const resp = await client.get<Blob>(`/battery/evaluations/${id}/report`, {
    responseType: 'blob'
  })
  return resp.data
}

// 静态导出，方便列表类型推导
export type { BatteryEvaluationListItem }
