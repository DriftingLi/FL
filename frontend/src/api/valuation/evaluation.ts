// 评估相关 API
// 重构说明：使用新 CreateEvaluationRequest 类型，去除 ForkliftType/EvaluationItem 引用
import client from './client'
import type {
  CreateEvaluationRequest,
  EvaluationDetail,
  EvaluationDetailResponse,
  EvaluationResult,
  EvaluationStats,
  PageQuery,
  PageResult
} from '@/types/valuation/evaluation'

/** 提交评估 */
export function createEvaluation(req: CreateEvaluationRequest): Promise<EvaluationResult> {
  // 拦截器已解包信封，直接返回业务负载
  return client.post<EvaluationResult>('/evaluations', req)
}

/** 获取评估详情（含输入参数 + 系数 + 维度评分） */
export function getEvaluationDetail(id: number): Promise<EvaluationDetailResponse> {
  return client.get<EvaluationDetailResponse>(`/evaluations/${id}`)
}

/** 评估历史列表（分页） */
export function listEvaluations(query: PageQuery): Promise<PageResult<EvaluationDetail>> {
  return client.get<PageResult<EvaluationDetail>>('/evaluations', {
    params: query
  })
}

/** 下载评估 PDF 二进制流（返回 Blob，前端用 a.download 触发下载） */
export function downloadEvaluationReportBlob(id: number): Promise<Blob> {
  // 二进制响应直接放行（共享 client 返回 Blob 本身）
  return client.get<Blob>(`/evaluations/${id}/report`, {
    responseType: 'blob'
  })
}

/** 查询累计评估次数 */
export function getEvaluationStats(): Promise<EvaluationStats> {
  return client.get<EvaluationStats>('/evaluations/stats')
}
