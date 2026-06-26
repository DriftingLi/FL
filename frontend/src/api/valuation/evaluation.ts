// 评估相关 API
import client from './client'
import type {
  CreateEvaluationRequest,
  EvaluationDetail,
  EvaluationDetailResponse,
  EvaluationItem,
  EvaluationResult,
  ForkliftType,
  PageQuery,
  PageResult
} from '@/types/valuation/evaluation'

/** 提交评估 */
export async function createEvaluation(req: CreateEvaluationRequest): Promise<EvaluationResult> {
  const resp = await client.post<unknown, { data: EvaluationResult }>('/evaluations', req)
  return resp.data
}

/** 获取评估详情（含输入参数 + 系数 + 部件状态） */
export async function getEvaluationDetail(id: number): Promise<EvaluationDetailResponse> {
  const resp = await client.get<unknown, { data: EvaluationDetailResponse }>(`/evaluations/${id}`)
  return resp.data
}

/** 获取评估主记录（不含 items） */
export async function getEvaluation(id: number): Promise<EvaluationDetail> {
  const resp = await client.get<unknown, { data: EvaluationDetail }>(`/evaluations/${id}`)
  return resp.data
}

/** 评估历史列表（分页） */
export async function listEvaluations(
  query: PageQuery & { forklift_type?: ForkliftType }
): Promise<PageResult<EvaluationDetail>> {
  const resp = await client.get<unknown, { data: PageResult<EvaluationDetail> }>('/evaluations', {
    params: query
  })
  return resp.data
}

/** 类型守卫：EvaluationDetailResponse 包含 items */
export function isDetailResponse(x: unknown): x is EvaluationDetailResponse {
  return !!x && typeof x === 'object' && 'evaluation' in x && 'items' in x
}

/** 提取部件状态（与 detail.items 同构） */
export type DetailItems = EvaluationItem[]
