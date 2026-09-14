// 评估相关 API
// 重构说明：使用新 CreateEvaluationRequest 类型，去除 ForkliftType/EvaluationItem 引用
import client from './client'
// 响应类型来自生成物（ADR-0048 决策 1/3，issue #967 片九）；入参（CreateEvaluationRequest /
// PageQuery）与分页壳不生成（决策 3），仍手写。
import type { EvaluationDetail, EvaluationResponse } from '@/api/generated/valuation'
import type { CreateEvaluationRequest, EvaluationStats, PageQuery, PageResult } from '@/types/valuation/evaluation'

/** 提交评估 */
export function createEvaluation(req: CreateEvaluationRequest): Promise<EvaluationResponse> {
  // 拦截器已解包信封，直接返回业务负载
  return client.post<EvaluationResponse>('/evaluations', req)
}

/** 获取评估详情（含输入参数 + 系数 + 维度评分） */
export function getEvaluationDetail(id: number): Promise<EvaluationDetail> {
  return client.get<EvaluationDetail>(`/evaluations/${id}`)
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