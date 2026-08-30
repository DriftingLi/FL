// 电池 RUL 评估 API（独立模块，与 evaluation.ts 物理隔离）
import client from './client'
import type {
  CreateBatteryRequest,
  BatteryEvaluationDetail,
  CreateBatteryResponse,
  BatteryReportResponse
} from '@/types/valuation/battery'

/** 提交电池循环数据并预测 RUL */
export function createBatteryEvaluation(req: CreateBatteryRequest): Promise<CreateBatteryResponse> {
  // 拦截器已解包信封，直接返回业务负载
  return client.post<CreateBatteryResponse>('/battery/evaluations', req)
}

/** 详情查询（含 cycle_features 数组） */
export function getBatteryEvaluation(id: number): Promise<BatteryEvaluationDetail> {
  return client.get<BatteryEvaluationDetail>(`/battery/evaluations/${id}`)
}

/** 触发后端生成 PDF 报告 */
export function generateBatteryReport(id: number): Promise<BatteryReportResponse> {
  return client.post<BatteryReportResponse>(`/battery/evaluations/${id}/report`)
}

/** 下载 PDF 二进制流（返回 Blob，前端用 a.download 触发下载） */
export function downloadBatteryReportBlob(id: number): Promise<Blob> {
  // 二进制响应直接放行（共享 client 返回 Blob 本身）
  return client.get<Blob>(`/battery/evaluations/${id}/report`, {
    responseType: 'blob'
  })
}
