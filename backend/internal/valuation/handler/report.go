// Package handler 实现 HTTP 处理器
// 本文件：PDF 报告生成与下载接口
// 重构后使用 model.EvaluationDetail + DimensionScores + Suggestions 作为 PDF 输入
package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"forklift-training/internal/storage"
	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
	"forklift-training/internal/valuation/service"
	"forklift-training/pkg/pdf"
)

// ReportHandler 报告 HTTP 处理器
// 持有评估仓储（查询评估详情）、PDF 生成器与文件存储（上传/检查 PDF）
type ReportHandler struct {
	evalRepo  *repository.EvaluationRepository
	generator *pdf.Generator
	storage   storage.Storage
	logger    *zap.Logger
}

// NewReportHandler 构造报告处理器
func NewReportHandler(evalRepo *repository.EvaluationRepository, gen *pdf.Generator, l *zap.Logger, st storage.Storage) *ReportHandler {
	return &ReportHandler{evalRepo: evalRepo, generator: gen, storage: st, logger: l}
}

// Generate 处理 POST /api/valuation/evaluations/:id/report
// 重新加载评估详情 → 生成 PDF bytes → 上传到存储后端 → 回写 report_pdf_path（R2 URL）
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

	// 1.1 实时重算 KTimeAdjusted（不入库字段），用于维度评分
	detail.KTimeAdjusted = service.AdjustKTimeByBrandAndIntensity(detail.KTime, detail.KHours, detail.KBrand)

	// 2. 重建派生字段（维度评分 + 建议），不重新跑完整算法
	dimScores, suggestions := rebuildDerivedFields(detail)

	// 3. 调用 PDF 生成器（返回 bytes）
	pdfBytes, err := h.generator.GenerateReport(detail, dimScores, suggestions)
	if err != nil {
		h.logger.Error("生成 PDF 失败", zap.Error(err), zap.Int64("id", id))
		Error(c, http.StatusInternalServerError, CodeInternalError, "生成 PDF 失败: "+err.Error())
		return
	}

	// 4. 上传到存储后端，获取可访问 URL
	key := fmt.Sprintf("reports/evaluation_report_%d_%s.pdf", id, time.Now().Format("20060102150405"))
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	pdfURL, err := h.storage.Save(ctx, key, pdfBytes, "application/pdf")
	if err != nil {
		h.logger.Error("上传 PDF 失败", zap.Error(err), zap.Int64("id", id))
		Error(c, http.StatusInternalServerError, CodeInternalError, "上传 PDF 失败: "+err.Error())
		return
	}

	// 5. 回写报告 URL 到 evaluations.report_pdf_path（字段名不变，存 R2 URL）
	if err := h.evalRepo.UpdateEvaluationReportPath(c.Request.Context(), id, pdfURL); err != nil {
		h.logger.Error("回写报告路径失败", zap.Error(err), zap.Int64("id", id))
		// 不中断流程：文件已上传，告知用户报告 URL 即可
	}

	// 6. 返回响应
	OK(c, gin.H{
		"evaluation_id": id,
		"pdf_url":       pdfURL,
		"file_size":     len(pdfBytes),
	})
}

// Download 处理 GET /api/valuation/evaluations/:id/report
// 优先从数据库读取 report_pdf_path（R2 URL）；若不存在或存储中无此文件则即时生成；
// 最终 302 重定向到 R2 公开访问 URL（浏览器直连下载）
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

	// 1.1 实时重算 KTimeAdjusted（不入库字段），用于维度评分
	detail.KTimeAdjusted = service.AdjustKTimeByBrandAndIntensity(detail.KTime, detail.KHours, detail.KBrand)

	// 2. 校验已有 URL 是否有效（为空或存储中不存在则重新生成）
	pdfURL := detail.ReportPdfPath
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	if pdfURL == "" {
		pdfURL = h.regenerateAndUpload(ctx, detail, id)
		if pdfURL == "" {
			Error(c, http.StatusInternalServerError, CodeInternalError, "生成 PDF 失败")
			return
		}
	} else {
		exists, err := h.storage.Exists(ctx, pdfURL)
		if err != nil {
			h.logger.Warn("检查存储中 PDF 是否存在失败，按不存在处理并重新生成",
				zap.String("url", pdfURL), zap.Error(err))
			exists = false
		}
		if !exists {
			pdfURL = h.regenerateAndUpload(ctx, detail, id)
			if pdfURL == "" {
				Error(c, http.StatusInternalServerError, CodeInternalError, "生成 PDF 失败")
				return
			}
		}
	}

	// 3. 302 重定向到 R2 公开访问 URL
	c.Redirect(http.StatusFound, pdfURL)
}

// regenerateAndUpload 重新生成 PDF 并上传到存储后端，返回新的 URL。
// 失败时返回空字符串（错误已在内部记日志）。
func (h *ReportHandler) regenerateAndUpload(ctx context.Context, detail *model.EvaluationDetail, id int64) string {
	dimScores, suggestions := rebuildDerivedFields(detail)
	pdfBytes, genErr := h.generator.GenerateReport(detail, dimScores, suggestions)
	if genErr != nil {
		h.logger.Error("生成 PDF 失败", zap.Error(genErr), zap.Int64("id", id))
		return ""
	}
	key := fmt.Sprintf("reports/evaluation_report_%d_%s.pdf", id, time.Now().Format("20060102150405"))
	pdfURL, saveErr := h.storage.Save(ctx, key, pdfBytes, "application/pdf")
	if saveErr != nil {
		h.logger.Error("上传 PDF 失败", zap.Error(saveErr), zap.Int64("id", id))
		return ""
	}
	if dbErr := h.evalRepo.UpdateEvaluationReportPath(ctx, id, pdfURL); dbErr != nil {
		h.logger.Warn("回写报告路径失败", zap.Error(dbErr), zap.Int64("id", id))
	}
	return pdfURL
}

// =====================================================
// 工具函数
// =====================================================

// rebuildDerivedFields 从持久化记录重建维度评分与文本建议
// 与 service.buildDimensionScores / buildSuggestions 算法一致，避免重新跑完整评估流程
// 维度评分改为 5 维：出厂时间 / 使用强度 / 品牌价值 / 市场需求 / 车辆情况
func rebuildDerivedFields(d *model.EvaluationDetail) (map[string]float64, []string) {
	if d == nil {
		return nil, nil
	}
	// 复用 service.BuildDimensionScores 保证与评估流程维度一致（含 [0,1] 钳制）
	scoreList := service.BuildDimensionScores(d.KTime, d.KHours, d.KBrand, d.KCondition, d.KMarket)
	dimScores := make(map[string]float64, len(scoreList))
	for _, s := range scoreList {
		dimScores[s.Label] = s.Value
	}
	suggestions := buildSuggestionsFromDetail(d)
	if suggestions == nil {
		suggestions = []string{}
	}
	return dimScores, suggestions
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

	// 2. 证件缺失提示 + 可售性警告（百分比取 DB 默认值 10%；若管理员调整过 coefficient_configs，
	//    新评估会通过 service.buildSuggestions 动态读取，此 fallback 仅用于持久化记录重建）
	if !d.HasLicensePlate {
		s = append(s, "缺少车牌，残值扣减 10%，无法正常上路行驶，建议补办后再出售")
	}
	if !d.HasRegistrationCertificate {
		s = append(s, "缺少登记证，残值扣减 10%，无法正常过户，建议补办后交易")
	}
	if !d.HasLicensePlate && !d.HasRegistrationCertificate {
		s = append(s, "车牌与登记证均缺失，无法正常出售与过户，强烈建议补齐证件后再交易")
	}

	// 3. 原厂漆与维保记录加分项提示（百分比取 DB 默认值 2%）
	switch {
	case d.OriginalPaint && d.HasMaintenanceRecords:
		s = append(s, "原厂漆完整且有维保记录，加成 4%，对保值有利")
	case d.OriginalPaint:
		s = append(s, "原厂漆完整，加成 2%")
	case d.HasMaintenanceRecords:
		s = append(s, "有维保记录，加成 2%")
	}

	// 4. 品牌/强度对时间衰减的修正方向
	//    Kb 高 → 衰减速率被压低（保值好）；Kh 高 → 衰减速率被抬高（磨损大）
	ratioHK := 1.0
	if d.KBrand > 0 {
		ratioHK = d.KHours / d.KBrand
	}
	switch {
	case ratioHK >= 1.10:
		s = append(s, "使用强度显著高于品牌保值能力，时间衰减被加速")
	case ratioHK >= 1.05:
		s = append(s, "使用强度略高于品牌保值能力，时间衰减略快")
	case ratioHK <= 0.90:
		s = append(s, "品牌保值能力强于使用强度折损，时间衰减被明显减缓")
	case ratioHK <= 0.95:
		s = append(s, "品牌保值能力略占优，时间衰减略缓")
	}

	// 5. 原始时间衰减水平（不含品牌/强度修正）
	if d.KTime < 0.50 {
		s = append(s, "使用年限较长，原始时间衰减明显")
	}

	// 6. 市场维度
	if d.KMarket < 0.99 {
		s = append(s, "区域市场系数偏低，二手需求较弱")
	} else if d.KMarket > 1.02 {
		s = append(s, "区域市场系数偏高，二手需求旺盛")
	}

	// 7. 残值率（已钳制 ≤ 100%）
	if d.OriginalPrice > 0 {
		rate := d.EstimatedValue / d.OriginalPrice
		switch {
		case rate >= 1.0:
			s = append(s, "残值率达 100% 上限（综合车况、市场极优），按原价出售")
		case rate >= 0.7:
			s = append(s, "残值率较高，建议按当前车况正常出售")
		case rate < 0.3:
			s = append(s, "残值率较低，建议拆件出售或作为配件使用")
		}
	}

	return s
}
