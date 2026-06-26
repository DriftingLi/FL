// Package handler 实现 HTTP 处理器
// 本文件：PDF 报告生成与下载接口
package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
	"forklift-training/internal/valuation/service"
	"forklift-training/pkg/pdf"
)

// ReportHandler 报告 HTTP 处理器
type ReportHandler struct {
	queries   *repository.Queries
	generator *pdf.Generator
	logger    *zap.Logger
}

// NewReportHandler 构造报告处理器
func NewReportHandler(q *repository.Queries, gen *pdf.Generator, l *zap.Logger) *ReportHandler {
	return &ReportHandler{queries: q, generator: gen, logger: l}
}

// Generate 处理 POST /api/v1/evaluations/:id/report
// 重新加载评估详情 + 部件状态 → 调用 PDF 生成器 → 落盘 → 回写 report_pdf_path
func (h *ReportHandler) Generate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}

	// 1. 读取评估主记录
	eval, err := h.queries.GetEvaluation(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "评估记录不存在")
			return
		}
		h.logger.Error("查询评估记录失败", zap.Error(err), zap.Int64("id", id))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询评估记录失败")
		return
	}

	// 2. 读取部件状态
	items, err := h.queries.ListEvaluationItems(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("查询部件状态失败", zap.Error(err), zap.Int64("id", id))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询部件状态失败")
		return
	}

	// 3. 转换为 PDF 生成器所需的 DTO
	report := convertEvaluationToResponse(eval)
	itemDTOs := convertItemsToDTO(items)
	// 补全派生字段：维度评分 + 建议（不落库，按需重建）
	dims, suggs := service.ReconstructFromRow(eval, convertItemsToItemResults(items))
	report.DimensionScores = dims
	report.Suggestions = suggs

	// 3.1 加载加权权重（用于 PDF 计算过程展示）
	weights, err := h.loadCalcWeights(c.Request.Context())
	if err != nil {
		h.logger.Error("加载加权权重失败", zap.Error(err), zap.Int64("id", id))
		Error(c, http.StatusInternalServerError, CodeInternalError, "加载加权权重失败")
		return
	}

	// 4. 调用 PDF 生成器
	pdfPath, err := h.generator.GenerateReport(&report, itemDTOs, weights)
	if err != nil {
		h.logger.Error("生成 PDF 失败", zap.Error(err), zap.Int64("id", id))
		Error(c, http.StatusInternalServerError, CodeInternalError, "生成 PDF 失败: "+err.Error())
		return
	}

	// 5. 落库：把报告路径写回 evaluations.report_pdf_path
	if _, err := h.queries.UpdateEvaluationReportPath(c.Request.Context(), repository.UpdateEvaluationReportPathParams{
		ID:            id,
		ReportPdfPath: pgtype.Text{String: pdfPath, Valid: true},
	}); err != nil {
		h.logger.Error("回写报告路径失败", zap.Error(err), zap.Int64("id", id))
		// 不中断流程：文件已生成，告知用户报告路径即可
	}

	// 6. 返回响应
	OK(c, gin.H{
		"evaluation_id": id,
		"pdf_path":      pdfPath,
		"file_name":     filepath.Base(pdfPath),
		"file_size":     fileSize(pdfPath),
	})
}

// Download 处理 GET /api/v1/evaluations/:id/report
// 优先从数据库读取 report_pdf_path；若不存在则即时生成；最终以 attachment 返回文件
func (h *ReportHandler) Download(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}

	// 1. 读取评估主记录
	eval, err := h.queries.GetEvaluation(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "评估记录不存在")
			return
		}
		h.logger.Error("查询评估记录失败", zap.Error(err), zap.Int64("id", id))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询评估记录失败")
		return
	}

	// 2. 解析已有路径
	pdfPath := ""
	if eval.ReportPdfPath.Valid && eval.ReportPdfPath.String != "" {
		pdfPath = eval.ReportPdfPath.String
	}

	// 3. 路径无效或文件不存在 → 重新生成
	if pdfPath == "" || !fileExists(pdfPath) {
		items, err := h.queries.ListEvaluationItems(c.Request.Context(), id)
		if err != nil {
			h.logger.Error("查询部件状态失败", zap.Error(err), zap.Int64("id", id))
			Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询部件状态失败")
			return
		}
		report := convertEvaluationToResponse(eval)
		itemDTOs := convertItemsToDTO(items)
		// 补全派生字段：维度评分 + 建议
		dims, suggs := service.ReconstructFromRow(eval, convertItemsToItemResults(items))
		report.DimensionScores = dims
		report.Suggestions = suggs

		// 加载加权权重（用于 PDF 计算过程展示）
		weights, wErr := h.loadCalcWeights(c.Request.Context())
		if wErr != nil {
			h.logger.Error("加载加权权重失败", zap.Error(wErr), zap.Int64("id", id))
			Error(c, http.StatusInternalServerError, CodeInternalError, "加载加权权重失败")
			return
		}

		newPath, genErr := h.generator.GenerateReport(&report, itemDTOs, weights)
		if genErr != nil {
			h.logger.Error("生成 PDF 失败", zap.Error(genErr), zap.Int64("id", id))
			Error(c, http.StatusInternalServerError, CodeInternalError, "生成 PDF 失败: "+genErr.Error())
			return
		}
		pdfPath = newPath
		// 异步写回数据库（不阻塞下载）
		if _, dbErr := h.queries.UpdateEvaluationReportPath(c.Request.Context(), repository.UpdateEvaluationReportPathParams{
			ID:            id,
			ReportPdfPath: pgtype.Text{String: pdfPath, Valid: true},
		}); dbErr != nil {
			h.logger.Warn("回写报告路径失败", zap.Error(dbErr), zap.Int64("id", id))
		}
	}

	// 4. 设置下载响应头
	fileName := fmt.Sprintf("evaluation_report_%d.pdf", eval.ID)
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.Header("Content-Type", "application/pdf")
	c.File(pdfPath)
}

// ========== 工具函数 ==========

// fileExists 判断文件是否存在
func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// fileSize 获取文件大小（字节），失败返回 0
func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// loadCalcWeights 从 coefficient_configs 表加载 4 个加权权重
// 用于 PDF 报告中残值计算过程的展示
func (h *ReportHandler) loadCalcWeights(ctx context.Context) (model.CalcWeights, error) {
	// 加载工况权重 w₁
	wWork, err := h.queries.GetCoefficientByKey(ctx, "w_work_condition")
	if err != nil {
		return model.CalcWeights{}, fmt.Errorf("加载 w_work_condition 失败: %w", err)
	}
	// 加载品牌权重 w₂
	wBrand, err := h.queries.GetCoefficientByKey(ctx, "w_brand")
	if err != nil {
		return model.CalcWeights{}, fmt.Errorf("加载 w_brand 失败: %w", err)
	}
	// 加载车况权重 w₃
	wCondition, err := h.queries.GetCoefficientByKey(ctx, "w_condition")
	if err != nil {
		return model.CalcWeights{}, fmt.Errorf("加载 w_condition 失败: %w", err)
	}
	// 加载市场权重 w₄
	wMarket, err := h.queries.GetCoefficientByKey(ctx, "w_market")
	if err != nil {
		return model.CalcWeights{}, fmt.Errorf("加载 w_market 失败: %w", err)
	}
	return model.CalcWeights{
		WWorkCondition: wWork.Value,
		WBrand:         wBrand.Value,
		WCondition:     wCondition.Value,
		WMarket:        wMarket.Value,
	}, nil
}
