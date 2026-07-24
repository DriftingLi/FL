// Package handler - 电池 RUL 评估 HTTP 接口
// 5 个端点：Create / List / Get / GenerateReport / DownloadReport
// 路径前缀：/api/v1/battery/*，与现有 /api/v1/evaluations/* 完全独立
package handler

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
	"forklift-training/internal/valuation/service"
	"forklift-training/pkg/pdf"
)

// BatteryHandler 电池 RUL 评估 HTTP 处理器
type BatteryHandler struct {
	repo         *repository.BatteryRepository
	service      *service.BatteryRULService
	logger       *zap.Logger
	pdfOutputDir string
}

// NewBatteryHandler 构造电池处理器
func NewBatteryHandler(repo *repository.BatteryRepository, svc *service.BatteryRULService, l *zap.Logger, pdfOutputDir string) *BatteryHandler {
	return &BatteryHandler{repo: repo, service: svc, logger: l, pdfOutputDir: pdfOutputDir}
}

// Create 处理 POST /api/v1/battery/evaluations
// 接收循环充放电数据 → 调用 service 预测 → 持久化 → 返回 RUL/SOH
func (h *BatteryHandler) Create(c *gin.Context) {
	var req model.CreateBatteryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求参数格式错误: "+err.Error())
		return
	}
	// 业务校验
	if err := req.Validate(); err != nil {
		Error(c, http.StatusBadRequest, CodeInvalidParam, err.Error())
		return
	}

	// 调用 service 预测
	result, err := h.service.Predict(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("电池 RUL 预测失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeInternalError, "预测失败: "+err.Error())
		return
	}

	// 构造评估主记录
	eval := &model.BatteryEvaluation{
		BatteryType:       req.BatteryType,
		BatteryModel:      req.BatteryModel,
		CycleCount:        len(req.Cycles),
		RulCycles:         result.RulCycles,
		SohPercent:        result.SohPercent,
		Confidence:        result.Confidence,
		ConfidenceLow:     result.ConfidenceLow,
		ConfidenceHigh:    result.ConfidenceHigh,
		FeatureImportance: result.FeatureImportance,
		Suggestions:       result.Suggestions,
	}

	// 持久化（带上当前登录用户 ID）
	userID := currentValuationUserID(c)
	saved, err := h.repo.CreateEvaluation(c.Request.Context(), eval, result.CycleFeatures, userID)
	if err != nil {
		h.logger.Error("保存电池评估记录失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "保存评估记录失败")
		return
	}

	// 返回响应
	OK(c, model.CreateBatteryResponse{
		EvaluationID:   saved.ID,
		BatteryType:    saved.BatteryType,
		CycleCount:     saved.CycleCount,
		RulCycles:      saved.RulCycles,
		SohPercent:     saved.SohPercent,
		Confidence:     saved.Confidence,
		ConfidenceLow:  saved.ConfidenceLow,
		ConfidenceHigh: saved.ConfidenceHigh,
		Suggestions:    saved.Suggestions,
		CreatedAt:      saved.CreatedAt,
	})
}

// List 处理 GET /api/v1/battery/evaluations?battery_type=lfp
// 分页查询评估历史摘要
func (h *BatteryHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	batteryType := c.Query("battery_type")
	// 简单校验：必须是合法值
	if batteryType != "" {
		if !model.BatteryType(batteryType).IsValid() {
			Error(c, http.StatusBadRequest, CodeInvalidParam, "电池类型非法：仅支持 lfp / ncm / other")
			return
		}
	}

	// 仅查询当前登录用户的记录（List 在鉴权组，userID 必然 >0）
	userID := currentValuationUserID(c)
	items, total, err := h.repo.ListEvaluations(c.Request.Context(), batteryType, userID, pageSize, offset)
	if err != nil {
		h.logger.Error("查询电池评估列表失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询评估列表失败")
		return
	}

	OK(c, model.ListBatteryResponse{
		Total: total,
		Items: items,
	})
}

// Get 处理 GET /api/v1/battery/evaluations/:id
// 查询评估详情（含周期特征），仅返回属于当前登录用户的记录
func (h *BatteryHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}

	userID := currentValuationUserID(c)
	eval, err := h.repo.GetEvaluationByUser(c.Request.Context(), id, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "电池评估记录不存在")
			return
		}
		h.logger.Error("查询电池评估详情失败", zap.Error(err), zap.Int64("id", id))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询评估详情失败")
		return
	}

	// 重新生成建议（如果 DB 中没有存）
	if len(eval.Suggestions) == 0 {
		eval.Suggestions = h.buildSuggestionsFromRecord(eval)
	}

	OK(c, eval)
}

// GenerateReport 处理 POST /api/v1/battery/evaluations/:id/report
// 生成 PDF 报告并落盘
func (h *BatteryHandler) GenerateReport(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}

	eval, err := h.repo.GetEvaluation(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "电池评估记录不存在")
			return
		}
		h.logger.Error("查询电池评估失败", zap.Error(err), zap.Int64("id", id))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询评估记录失败")
		return
	}

	// 如果已有 PDF 路径且文件存在，直接返回（避免重复生成）
	if eval.ReportPdfPath != "" {
		if _, err := os.Stat(eval.ReportPdfPath); err == nil {
			OK(c, model.BatteryReportResponse{
				EvaluationID: eval.ID,
				ReportPath:   eval.ReportPdfPath,
				GeneratedAt:  eval.UpdatedAt,
			})
			return
		}
	}

	// 生成 PDF
	// 使用注入的 PDF 输出目录
	pdfDir := h.pdfOutputDir
	if pdfDir == "" {
		pdfDir = "./storage/reports"
	}
	filename := fmt.Sprintf("battery_report_%d_%s.pdf", eval.ID, time.Now().Format("20060102150405"))
	fullPath := pdfDir + "/" + filename

	if err := os.MkdirAll(pdfDir, 0o755); err != nil {
		h.logger.Error("创建 PDF 目录失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeInternalError, "创建 PDF 目录失败")
		return
	}

	if err := h.generateBatteryPDF(fullPath, eval); err != nil {
		h.logger.Error("生成电池 PDF 失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeInternalError, "生成 PDF 失败: "+err.Error())
		return
	}

	// 更新报告路径
	if err := h.repo.UpdateReportPath(c.Request.Context(), eval.ID, fullPath); err != nil {
		h.logger.Warn("更新 PDF 路径失败", zap.Error(err))
	}

	OK(c, model.BatteryReportResponse{
		EvaluationID: eval.ID,
		ReportPath:   fullPath,
		GeneratedAt:  time.Now().Format("2006-01-02T15:04:05Z07:00"),
	})
}

// DownloadReport 处理 GET /api/v1/battery/evaluations/:id/report
// 下载 PDF 报告
func (h *BatteryHandler) DownloadReport(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}

	eval, err := h.repo.GetEvaluation(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "电池评估记录不存在")
			return
		}
		h.logger.Error("查询电池评估失败", zap.Error(err), zap.Int64("id", id))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询评估记录失败")
		return
	}

	if eval.ReportPdfPath == "" {
		Error(c, http.StatusNotFound, CodeNotFound, "报告尚未生成，请先调用 POST /:id/report")
		return
	}

	// 文件不存在则重新生成
	if _, err := os.Stat(eval.ReportPdfPath); err != nil {
		h.logger.Warn("PDF 文件不存在，重新生成", zap.String("path", eval.ReportPdfPath))
		if err := h.generateBatteryPDF(eval.ReportPdfPath, eval); err != nil {
			h.logger.Error("重新生成 PDF 失败", zap.Error(err))
			Error(c, http.StatusInternalServerError, CodeInternalError, "重新生成 PDF 失败")
			return
		}
	}

	c.FileAttachment(eval.ReportPdfPath, filenameFromPath(eval.ReportPdfPath))
}

// buildSuggestionsFromRecord 基于评估字段生成简单建议（详情接口 fallback）
func (h *BatteryHandler) buildSuggestionsFromRecord(eval *model.BatteryEvaluation) []string {
	out := []string{}
	// EOL 阈值 60%（与 service 端 estimateRUL 保持一致）
	switch {
	case eval.SohPercent >= 95:
		out = append(out, "电池健康度优秀（SOH≥95%），处于生命初期，建议常规巡检。")
	case eval.SohPercent >= 80:
		out = append(out, "电池健康度良好（80%≤SOH<95%），状态稳定，可继续投入使用。")
	case eval.SohPercent >= 60:
		out = append(out, "电池健康度临近梯次利用边界（60%≤SOH<80%），建议评估应用场景与监测频率。")
	default:
		out = append(out, fmt.Sprintf("电池健康度偏低（SOH=%.1f%%<60%%），已低于 EOL 标准，建议尽快更换。", eval.SohPercent))
	}
	out = append(out, fmt.Sprintf("预测剩余循环数约 %d 次（置信区间 %d~%d）。", eval.RulCycles, eval.ConfidenceLow, eval.ConfidenceHigh))
	switch eval.BatteryType {
	case model.BatteryTypeLFP:
		out = append(out, "LFP 电池循环寿命长，安全性好；如 SOH 仍高，可考虑梯次利用。")
	case model.BatteryTypeNCM:
		out = append(out, "NCM 电池能量密度高但循环寿命较短，注意高温环境与过充风险。")
	}
	return out
}

// generateBatteryPDF 调用 pdf 包生成电池报告
func (h *BatteryHandler) generateBatteryPDF(path string, eval *model.BatteryEvaluation) error {
	return pdf.GenerateBatteryReportToPath("", path, eval)
}

// filenameFromPath 从完整路径中提取文件名
func filenameFromPath(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}
