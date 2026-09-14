// 报告类型 —— 唯一事实源是后端注解（ADR-0048，issue #967 片九）。
//
// POST /evaluations/:id/report 的响应体是 handler 内联构造的
// `gin.H{evaluation_id, pdf_url, file_size}`（片九「非统一信封」登记项之一，未改造），
// 注解层用 swag 内联 object{} 描述；生成器只渲染具名类型，故此处保留该三字段窄形状 ——
// 形状与注解**逐字段一致**（旧手写版本的 pdf_path / file_name 是凭空字段，已按判定 ② 修正）。
export interface GenerateReportResponse {
  evaluation_id: number
  /** 报告 URL（storage 路径）；文件名从 URL 末段派生 */
  pdf_url: string
  file_size: number
}
