// 报告 API
import client, { API_BASE_URL } from './client'
import type { GenerateReportResponse } from '@/types/valuation/report'

/** 触发后端生成 PDF（落盘 + 回写 report_pdf_path） */
export function generateReport(id: number): Promise<GenerateReportResponse> {
  // 拦截器已解包信封，直接返回业务负载
  return client.post<GenerateReportResponse>(`/evaluations/${id}/report`)
}

/** 获取 PDF 完整 URL（用于浏览器直接下载/预览） */
export function getReportDownloadUrl(id: number): string {
  return `${API_BASE_URL}/evaluations/${id}/report`
}
