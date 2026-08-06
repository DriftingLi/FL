// Package handler - 电池 RUL 评估 HTTP 接口
// 5 个端点：Create / List / Get / GenerateReport / DownloadReport
// 路径前缀：/api/v1/battery/*，与现有 /api/v1/evaluations/* 完全独立
package handler

import (
	"context"
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"forklift-training/internal/middleware"
	"forklift-training/internal/storage"
	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/service"
	"forklift-training/pkg/pdf"
	"forklift-training/pkg/response"
)

// BatteryHandler 电池 RUL 评估 HTTP 处理器
type BatteryHandler struct {
	repo    BatteryStore
	service *service.BatteryRULService
	logger  *zap.Logger
	storage storage.Storage
	// reportCoord 电池报告流程协调器（生成/下载/再生成单点实现）
	reportCoord *ReportCoordinator[model.BatteryEvaluation]
}

// NewBatteryHandler 构造电池处理器
func NewBatteryHandler(repo BatteryStore, svc *service.BatteryRULService, l *zap.Logger, st storage.Storage) *BatteryHandler {
	return &BatteryHandler{
		repo:    repo,
		service: svc,
		logger:  l,
		storage: st,
		reportCoord: &ReportCoordinator[model.BatteryEvaluation]{
			logger:      l,
			storage:     st,
			keyPrefix:   "reports/battery_report_",
			notFoundMsg: "电池评估记录不存在",
			loader:      repo.GetEvaluation,
			pathOf:      func(e *model.BatteryEvaluation) string { return e.ReportPdfPath },
			writer:      repo.UpdateReportPath,
			generator: func(ctx context.Context, e *model.BatteryEvaluation) ([]byte, error) {
				// 记录不含特征稳定性分数，health 传 1.0（不触发稳定性提示），与预测流程共用 builder
				if len(e.Suggestions) == 0 {
					e.Suggestions = service.BuildBatterySuggestions(e.BatteryType, e.SohPercent,
						e.RulCycles, e.ConfidenceLow, e.ConfidenceHigh, 1.0)
				}
				return pdf.GenerateBatteryReportBytes(e)
			},
		},
	}
}

// Create 处理 POST /api/v1/battery/evaluations
// 接收循环充放电数据 → 调用 service 预测 → 持久化 → 返回 RUL/SOH
func (h *BatteryHandler) Create(c *gin.Context) {
	var req model.CreateBatteryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数格式错误: "+err.Error())
		return
	}
	// 业务校验
	if err := req.Validate(); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 调用 service 预测
	result, err := h.service.Predict(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("电池 RUL 预测失败", zap.Error(err))
		response.ServerError(c, "预测失败: "+err.Error())
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
		response.ServerError(c, "保存评估记录失败")
		return
	}

	// 返回响应
	response.Success(c, model.CreateBatteryResponse{
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
			response.BadRequest(c, "电池类型非法：仅支持 lfp / ncm / other")
			return
		}
	}

	// 仅查询当前登录用户的记录（List 在鉴权组，userID 必然 >0）
	userID := middleware.CurrentUserID(c)
	items, total, err := h.repo.ListEvaluations(c.Request.Context(), batteryType, userID, pageSize, offset)
	if err != nil {
		h.logger.Error("查询电池评估列表失败", zap.Error(err))
		response.ServerError(c, "查询评估列表失败")
		return
	}

	response.Success(c, model.ListBatteryResponse{
		Total: total,
		Items: items,
	})
}

// Get 处理 GET /api/v1/battery/evaluations/:id
// 查询评估详情（含周期特征），仅返回属于当前登录用户的记录
func (h *BatteryHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "id 必须为整数")
		return
	}

	userID := middleware.CurrentUserID(c)
	eval, err := h.repo.GetEvaluationByUser(c.Request.Context(), id, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.NotFound(c, "电池评估记录不存在")
			return
		}
		h.logger.Error("查询电池评估详情失败", zap.Error(err), zap.Int64("id", id))
		response.ServerError(c, "查询评估详情失败")
		return
	}

	// 重新生成建议（如果 DB 中没有存）
	if len(eval.Suggestions) == 0 {
		eval.Suggestions = h.buildSuggestionsFromRecord(eval)
	}

	response.Success(c, eval)
}

// GenerateReport 处理 POST /api/v1/battery/evaluations/:id/report
func (h *BatteryHandler) GenerateReport(c *gin.Context) {
	h.reportCoord.Generate(c)
}

// DownloadReport 处理 GET /api/v1/battery/evaluations/:id/report
func (h *BatteryHandler) DownloadReport(c *gin.Context) {
	h.reportCoord.Download(c)
}

// buildSuggestionsFromRecord 基于评估字段生成建议（详情接口 fallback）
// 与预测流程共用 service.BuildBatterySuggestions；记录不含特征稳定性分数，health 传 1.0（不触发稳定性提示）。
func (h *BatteryHandler) buildSuggestionsFromRecord(eval *model.BatteryEvaluation) []string {
	return service.BuildBatterySuggestions(eval.BatteryType, eval.SohPercent,
		eval.RulCycles, eval.ConfidenceLow, eval.ConfidenceHigh, 1.0)
}
