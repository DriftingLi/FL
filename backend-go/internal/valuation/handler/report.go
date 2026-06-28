// Package handler 实现 HTTP 处理器
// 本文件：PDF 报告生成与下载接口
// 重构后使用 model.EvaluationDetail + DimensionScores + Suggestions 作为 PDF 输入
package handler

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
	"forklift-training/pkg/pdf"
)

// ReportHandler 报告 HTTP 处理器
// 持有评估仓储（查询评估详情）与 PDF 生成器
type ReportHandler struct {
	evalRepo  *repository.EvaluationRepository
	generator *pdf.Generator
	logger    *zap.Logger
}

// NewReportHandler 构造报告处理器
func NewReportHandler(evalRepo *repository.EvaluationRepository, gen *pdf.Generator, l *zap.Logger) *ReportHandler {
	return &ReportHandler{evalRepo: evalRepo, generator: gen, logger: l}
}

// Generate 处理 POST /api/valuation/evaluations/:id/report
// 重新加载评估详情 → 调用 PDF 生成器 → 落盘 → 回写 report_pdf_path
func (h *ReportHandler) Generate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}

	// 1. 读取评估详情（含输入字段 + 计算结果 + 时间戳）
	detail, err := h.evalRepo.GetEvaluation(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "评估记录不存在")
			return
		}
		h.logger.Error("查询评估记录失败", zap.Error(err), zap.Int64("id", id))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询评估记录失败")
		return
	}

	// 2. 重建派生字段（维度评分 + 建议），不重新跑完整算法
	dimScores, suggestions := rebuildDerivedFields(detail)

	// 3. 调用 PDF 生成器
	pdfPath, err := h.generator.GenerateReport(detail, dimScores, suggestions)
	if err != nil {
		h.logger.Error("生成 PDF 失败", zap.Error(err), zap.Int64("id", id))
		Error(c, http.StatusInternalServerError, CodeInternalError, "生成 PDF 失败: "+err.Error())
		return
	}

	// 4. 回写报告路径到 evaluations.report_pdf_path
	if err := h.evalRepo.UpdateEvaluationReportPath(c.Request.Context(), id, pdfPath); err != nil {
		h.logger.Error("回写报告路径失败", zap.Error(err), zap.Int64("id", id))
		// 不中断流程：文件已生成，告知用户报告路径即可
	}

	// 5. 返回响应
	OK(c, gin.H{
		"evaluation_id": id,
		"pdf_path":      pdfPath,
		"file_name":     filenameFromPath(pdfPath),
		"file_size":     fileSize(pdfPath),
	})
}

// Download 处理 GET /api/valuation/evaluations/:id/report
// 优先从数据库读取 report_pdf_path；若不存在则即时生成；最终以 attachment 返回文件
func (h *ReportHandler) Download(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}

	// 1. 读取评估详情
	detail, err := h.evalRepo.GetEvaluation(c.Request.Context(), id)
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
	pdfPath := detail.ReportPdfPath

	// 3. 路径无效或文件不存在 → 重新生成
	if pdfPath == "" || !fileExists(pdfPath) {
		dimScores, suggestions := rebuildDerivedFields(detail)
		newPath, genErr := h.generator.GenerateReport(detail, dimScores, suggestions)
		if genErr != nil {
			h.logger.Error("生成 PDF 失败", zap.Error(genErr), zap.Int64("id", id))
			Error(c, http.StatusInternalServerError, CodeInternalError, "生成 PDF 失败: "+genErr.Error())
			return
		}
		pdfPath = newPath
		// 异步写回数据库（不阻塞下载）
		if dbErr := h.evalRepo.UpdateEvaluationReportPath(c.Request.Context(), id, pdfPath); dbErr != nil {
			h.logger.Warn("回写报告路径失败", zap.Error(dbErr), zap.Int64("id", id))
		}
	}

	// 4. 设置下载响应头
	fileName := fmt.Sprintf("evaluation_report_%d.pdf", detail.ID)
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.Header("Content-Type", "application/pdf")
	c.File(pdfPath)
}

// =====================================================
// 工具函数
// =====================================================

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

// rebuildDerivedFields 从持久化记录重建维度评分与文本建议
// 与 service.buildDimensionScores / buildSuggestions 算法一致，避免重新跑完整评估流程
func rebuildDerivedFields(d *model.EvaluationDetail) (map[string]float64, []string) {
	if d == nil {
		return nil, nil
	}
	dimScores := map[string]float64{
		"时间维度": roundTo4(d.KTime),
		"使用强度": roundTo4(d.KHours),
		"品牌":   roundTo4(d.KBrand),
		"车况":   roundTo4(d.KCondition),
		"市场":   roundTo4(d.KMarket),
	}
	suggestions := buildSuggestionsFromDetail(d)
	if suggestions == nil {
		suggestions = []string{}
	}
	return dimScores, suggestions
}

// roundTo4 四舍五入到 4 位小数（与 service.roundTo4 一致）
func roundTo4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

// buildSuggestionsFromDetail 基于持久化记录重建文本建议
// 与 service.buildSuggestions 算法一致，仅以 EvaluationDetail 字段为输入
func buildSuggestionsFromDetail(d *model.EvaluationDetail) []string {
	if d == nil {
		return nil
	}
	s := make([]string, 0, 8)

	// 1. 车况维度（核心）
	switch {
	case d.KCondition >= 1.00:
		s = append(s, "车况优秀，原漆、维保记录、证件齐全，建议正常出售")
	case d.KCondition >= 0.85:
		s = append(s, "车况良好，残值稳定，可作为二手设备出售")
	case d.KCondition >= 0.65:
		s = append(s, "车况一般，建议整备后出售以提升残值")
	case d.KCondition >= 0.45:
		s = append(s, "车况较差，多个维度有折损，建议折价处理")
	default:
		s = append(s, "车况很差，建议拆件出售或作为配件使用")
	}

	// 2. 证件缺失提示
	if !d.HasLicensePlate {
		s = append(s, "缺少车牌，残值扣减 5%，建议补办后再出售")
	}
	if !d.HasRegistrationCertificate {
		s = append(s, "缺少登记证，残值扣减 5%，过户需提供登记证")
	}

	// 3. 原厂漆与维保记录加分项提示
	if d.OriginalPaint && d.HasMaintenanceRecords {
		s = append(s, "原厂漆完整且有维保记录，加成 6%，对保值有利")
	} else if d.OriginalPaint {
		s = append(s, "原厂漆完整，加成 3%")
	} else if d.HasMaintenanceRecords {
		s = append(s, "有维保记录，加成 3%")
	}

	// 4. 品牌维度
	switch {
	case d.KBrand >= 1.10:
		s = append(s, "品牌力强（进口一线），保值能力优秀")
	case d.KBrand >= 1.00:
		s = append(s, "品牌力较好，残值具备一定支撑")
	case d.KBrand >= 0.85:
		s = append(s, "品牌力中等，残值持平行业平均")
	default:
		s = append(s, "品牌力偏弱，残值相对偏低")
	}

	// 5. 时间维度
	if d.KTime < 0.50 {
		s = append(s, "使用年限较长，残值随时间明显折减")
	}

	// 6. 使用强度维度
	switch {
	case d.KHours >= 1.10:
		s = append(s, "累计使用小时远低于行业平均，机械磨损小")
	case d.KHours <= 0.85:
		s = append(s, "累计使用小时偏高，机械磨损较大")
	}

	// 7. 市场维度
	if d.KMarket < 0.99 {
		s = append(s, "区域市场系数偏低，二手需求较弱")
	} else if d.KMarket > 1.02 {
		s = append(s, "区域市场系数偏高，二手需求旺盛")
	}

	// 8. 残值率
	if d.OriginalPrice > 0 {
		rate := d.EstimatedValue / d.OriginalPrice
		switch {
		case rate >= 0.7:
			s = append(s, "残值率较高，建议按当前车况正常出售")
		case rate < 0.3:
			s = append(s, "残值率较低，建议拆件出售或作为配件使用")
		}
	}

	return s
}
