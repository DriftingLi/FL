// Package handler - 电池 RUL 评估 HTTP 接口
// 5 个端点：Create / List / Get / GenerateReport / DownloadReport
// 路径前缀：/api/v1/battery/*，与现有 /api/v1/evaluations/* 完全独立
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

	"forklift-training/internal/middleware"
	"forklift-training/internal/storage"
	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
	"forklift-training/internal/valuation/service"
	"forklift-training/pkg/pdf"
)

// BatteryHandler 电池 RUL 评估 HTTP 处理器
type BatteryHandler struct {
	repo    *repository.BatteryRepository
	service *service.BatteryRULService
	logger  *zap.Logger
	storage storage.Storage
}

// NewBatteryHandler 构造电池处理器
func NewBatteryHandler(repo *repository.BatteryRepository, svc *service.BatteryRULService, l *zap.Logger, st storage.Storage) *BatteryHandler {
	return &BatteryHandler{repo: repo, service: svc, logger: l, storage: st}
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
	userID := middleware.CurrentUserID(c)
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
	userID := middleware.CurrentUserID(c)
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

	userID := middleware.CurrentUserID(c)
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
// 重新加载电池评估 → 生成 PDF bytes → 上传到存储后端 → 回写 report_pdf_path（R2 URL）
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

	// 1. 生成 PDF bytes
	pdfBytes, err := pdf.GenerateBatteryReportBytes(eval)
	if err != nil {
		h.logger.Error("生成电池 PDF 失败", zap.Error(err), zap.Int64("id", id))
		Error(c, http.StatusInternalServerError, CodeInternalError, "生成 PDF 失败: "+err.Error())
		return
	}

	// 2. 上传到存储后端，获取可访问 URL
	key := fmt.Sprintf("reports/battery_report_%d_%s.pdf", id, time.Now().Format("20060102150405"))
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	pdfURL, err := h.storage.Save(ctx, key, pdfBytes, "application/pdf")
	if err != nil {
		h.logger.Error("上传电池 PDF 失败", zap.Error(err), zap.Int64("id", id))
		Error(c, http.StatusInternalServerError, CodeInternalError, "上传 PDF 失败: "+err.Error())
		return
	}

	// 3. 回写报告 URL 到 battery_evaluations.report_pdf_path（字段名不变，存 R2 URL）
	if err := h.repo.UpdateReportPath(c.Request.Context(), eval.ID, pdfURL); err != nil {
		h.logger.Warn("回写电池 PDF 路径失败", zap.Error(err), zap.Int64("id", id))
		// 不中断流程：文件已上传，告知用户报告 URL 即可
	}

	// 4. 返回响应
	OK(c, gin.H{
		"evaluation_id": id,
		"pdf_url":       pdfURL,
		"file_size":     len(pdfBytes),
	})
}

// DownloadReport 处理 GET /api/v1/battery/evaluations/:id/report
// 优先从数据库读取 report_pdf_path（R2 URL）；若不存在或存储中无此文件则即时生成；
// 最终 302 重定向到 R2 公开访问 URL（浏览器直连下载）
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

	// 1. 校验已有 URL 是否有效（为空或存储中不存在则重新生成）
	pdfURL := eval.ReportPdfPath
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	if pdfURL == "" {
		pdfURL = h.regenerateAndUpload(ctx, eval, id)
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
			pdfURL = h.regenerateAndUpload(ctx, eval, id)
			if pdfURL == "" {
				Error(c, http.StatusInternalServerError, CodeInternalError, "生成 PDF 失败")
				return
			}
		}
	}

	// 2. 302 重定向到 R2 公开访问 URL
	c.Redirect(http.StatusFound, pdfURL)
}

// regenerateAndUpload 重新生成电池 PDF 并上传到存储后端，返回新的 URL。
// 失败时返回空字符串（错误已在内部记日志）。
func (h *BatteryHandler) regenerateAndUpload(ctx context.Context, eval *model.BatteryEvaluation, id int64) string {
	pdfBytes, genErr := pdf.GenerateBatteryReportBytes(eval)
	if genErr != nil {
		h.logger.Error("生成电池 PDF 失败", zap.Error(genErr), zap.Int64("id", id))
		return ""
	}
	key := fmt.Sprintf("reports/battery_report_%d_%s.pdf", id, time.Now().Format("20060102150405"))
	pdfURL, saveErr := h.storage.Save(ctx, key, pdfBytes, "application/pdf")
	if saveErr != nil {
		h.logger.Error("上传电池 PDF 失败", zap.Error(saveErr), zap.Int64("id", id))
		return ""
	}
	if dbErr := h.repo.UpdateReportPath(ctx, id, pdfURL); dbErr != nil {
		h.logger.Warn("回写电池 PDF 路径失败", zap.Error(dbErr), zap.Int64("id", id))
	}
	return pdfURL
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
