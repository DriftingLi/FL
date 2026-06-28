// 评估相关 API
// 重构说明：使用新 CreateEvaluationRequest 类型，去除 ForkliftType/EvaluationItem 引用
import client from './client'
import type {
  CreateEvaluationRequest,
  EvaluationDetail,
  EvaluationDetailResponse,
  EvaluationResult,
  PageQuery,
  PageResult
} from '@/types/valuation/evaluation'

/** 提交评估 */
export async function createEvaluation(req: CreateEvaluationRequest): Promise<EvaluationResult> {
  const resp = await client.post<unknown, { data: EvaluationResult }>('/evaluations', req)
  return resp.data
}

/** 获取评估详情（含输入参数 + 系数 + 维度评分） */
export async function getEvaluationDetail(id: number): Promise<EvaluationDetailResponse> {
  const resp = await client.get<unknown, { data: EvaluationDetailResponse }>(`/evaluations/${id}`)
  return resp.data
}

/** 兼容别名：与详情接口同构 */
export async function getEvaluation(id: number): Promise<EvaluationDetail> {
  return getEvaluationDetail(id)
}

/** 评估历史列表（分页） */
export async function listEvaluations(query: PageQuery): Promise<PageResult<EvaluationDetail>> {
  const resp = await client.get<unknown, { data: PageResult<EvaluationDetail> }>('/evaluations', {
    params: query
  })
  return resp.data
}

/** 下载评估 PDF 二进制流（返回 Blob，前端用 a.download 触发下载） */
export async function downloadEvaluationReportBlob(id: number): Promise<Blob> {
  const resp = await client.get<Blob>(`/evaluations/${id}/report`, {
    responseType: 'blob'
  })
  return resp.data
}
