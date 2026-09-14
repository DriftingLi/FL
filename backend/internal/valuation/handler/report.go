// Package handler 实现 HTTP 处理器
// 本文件：评估报告生成与下载接口（流程由 report.Coordinator 单点实现，此处只做注册）
// 重构后使用 model.EvaluationDetail + DimensionScores + Suggestions 作为 PDF 输入
package handler

import (
	"context"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"forklift-training/internal/storage"
	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/report"
	"forklift-training/internal/valuation/service"
)

// ReportHandler 报告 HTTP 处理器（薄壳：协调器装配 + 端点注册）。
type ReportHandler struct {
	coord   *report.Coordinator[model.EvaluationDetail]
	storage storage.Storage
	logger  *zap.Logger
}

// NewReportHandler 构造报告处理器
// resolver 用于未回填历史记录重建建议时动态读取 coefficient_configs（与评估流程同一份配置）。
func NewReportHandler(evalRepo EvaluationStore, gen ReportGenerator, l *zap.Logger, st storage.Storage, resolver service.CoefficientResolver) *ReportHandler {
	return &ReportHandler{
		logger:  l,
		storage: st,
		coord: report.New(report.Spec[model.EvaluationDetail]{
			Logger:    l,
			Storage:   st,
			KeyPrefix: "reports/evaluation_report_",
			Loader:    evalRepo.GetEvaluation,
			PathOf:    func(d *model.EvaluationDetail) string { return d.ReportPdfPath },
			Writer:    evalRepo.UpdateEvaluationReportPath,
			Prepare: func(ctx context.Context, d *model.EvaluationDetail) {
				// 单一装配点：KTimeAdjusted + 维度分（建议为评估时点锁定值，ADR-0004）
				service.RebuildDerivedFromDetail(d)
				service.EnsureSuggestions(ctx, d, resolver)
			},
			Render: func(_ context.Context, d *model.EvaluationDetail) ([]byte, error) {
				// 维度分为 typed 切片直传（顺序契约在 model.DimensionLabels，不再经 label 拼接）
				return gen.GenerateReport(d, d.DimensionScores, d.Suggestions)
			},
		}),
	}
}

// Generate 处理 POST /api/valuation/evaluations/:id/report
// @Summary 生成评估报告
// @Description 触发后端生成评估 PDF 报告（落盘 + 回写 report_pdf_path），返回 PDF URL 与大小。公开端点：无需登录。
// @Tags 估值-报告
// @Accept json
// @Produce json
// @Param id path integer true "评估记录 ID"
// @Success 200 {object} response.R{data=object{evaluation_id=integer,pdf_url=string,file_size=integer}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 404 {object} response.R "评估记录不存在"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/evaluations/{id}/report [post]
func (h *ReportHandler) Generate(c *gin.Context) {
	serveReportGenerate(c, h.coord, "评估记录不存在", h.logger)
}

// Download 处理 GET /api/valuation/evaluations/:id/report
// @Summary 下载评估报告
// @Description 经 storage 代理流式返回评估 PDF（Content-Disposition attachment）；URL 失效时并发安全地再生成。公开端点：无需登录。
// @Tags 估值-报告
// @Produce application/pdf
// @Param id path integer true "评估记录 ID"
// @Success 200 {file} file "PDF 二进制流"
// @Failure 400 {object} response.R "参数错误"
// @Failure 404 {object} response.R "评估记录不存在"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/evaluations/{id}/report [get]
func (h *ReportHandler) Download(c *gin.Context) {
	serveReportDownload(c, h.coord, h.storage, "评估记录不存在", h.logger)
}
