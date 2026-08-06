// Package handler 实现 HTTP 处理器
// 本文件：评估报告生成与下载接口（流程由 ReportCoordinator 单点实现）
// 重构后使用 model.EvaluationDetail + DimensionScores + Suggestions 作为 PDF 输入
package handler

import (
	"context"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"forklift-training/internal/storage"
	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/service"
)

// ReportHandler 报告 HTTP 处理器
// 持有评估报告协调器（加载/生成/回写三 adapter 槽位装配完毕）
type ReportHandler struct {
	coord *ReportCoordinator[model.EvaluationDetail]
}

// NewReportHandler 构造报告处理器
// resolver 用于未回填历史记录重建建议时动态读取 coefficient_configs（与评估流程同一份配置）。
func NewReportHandler(evalRepo EvaluationStore, gen ReportGenerator, l *zap.Logger, st storage.Storage, resolver service.ConfigResolver) *ReportHandler {
	return &ReportHandler{
		coord: &ReportCoordinator[model.EvaluationDetail]{
			logger:      l,
			storage:     st,
			keyPrefix:   "reports/evaluation_report_",
			notFoundMsg: "评估记录不存在",
			loader:      evalRepo.GetEvaluation,
			pathOf:      func(d *model.EvaluationDetail) string { return d.ReportPdfPath },
			writer:      evalRepo.UpdateEvaluationReportPath,
			generator: func(ctx context.Context, d *model.EvaluationDetail) ([]byte, error) {
				// 单一装配点：KTimeAdjusted + 维度分（建议为评估时点锁定值，ADR-0004）
				service.RebuildDerivedFromDetail(d)
				dimScores := make(map[string]float64, len(d.DimensionScores))
				for _, s := range d.DimensionScores {
					dimScores[s.Label] = s.Value
				}
				// 维度分转换为 PDF 生成器 adapter 的输入契约（map），非业务装配
				suggestions := d.Suggestions
				if len(suggestions) == 0 {
					suggestions = service.BuildSuggestions(ctx, service.FromDetail(d), resolver)
				}
				if suggestions == nil {
					suggestions = []string{}
				}
				return gen.GenerateReport(d, dimScores, suggestions)
			},
		},
	}
}

// Generate 处理 POST /api/valuation/evaluations/:id/report
func (h *ReportHandler) Generate(c *gin.Context) {
	h.coord.Generate(c)
}

// Download 处理 GET /api/valuation/evaluations/:id/report
func (h *ReportHandler) Download(c *gin.Context) {
	h.coord.Download(c)
}
