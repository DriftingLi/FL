// Package handler - 电池 RUL 评估 HTTP 接口
// 5 个端点：Create / List / Get / GenerateReport / DownloadReport
// 路径前缀：/api/valuation/battery/*，与 /api/valuation/evaluations/* 完全独立
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
	"forklift-training/internal/valuation/pdf"
	"forklift-training/internal/valuation/report"
	"forklift-training/internal/valuation/service"
	"forklift-training/pkg/paging"
	"forklift-training/pkg/response"
)

// BatteryHandler 电池 RUL 评估 HTTP 处理器
type BatteryHandler struct {
	repo    BatteryStore
	service *service.BatteryRULService
	logger  *zap.Logger
	storage storage.Storage
	// coord 电池报告流程协调器（生成/下载/再生成单点实现，gin-free）
	coord *report.Coordinator[model.BatteryEvaluation]
	// prepareSuggestions 建议 fallback 单点：详情端点与报告生成共用（不再两处复制）
	prepareSuggestions func(ctx context.Context, e *model.BatteryEvaluation)
}

// NewBatteryHandler 构造电池处理器
func NewBatteryHandler(repo BatteryStore, svc *service.BatteryRULService, l *zap.Logger, st storage.Storage) *BatteryHandler {
	prepareSuggestions := func(_ context.Context, e *model.BatteryEvaluation) {
		// 旧记录建议 fallback 单入口：health 由记录置信度反推（缺失默认 1.0）
		service.EnsureBatterySuggestions(e)
	}
	return &BatteryHandler{
		repo:               repo,
		service:            svc,
		logger:             l,
		storage:            st,
		prepareSuggestions: prepareSuggestions,
		coord: report.New(report.Spec[model.BatteryEvaluation]{
			Logger:    l,
			Storage:   st,
			KeyPrefix: "reports/battery_report_",
			Loader:    repo.GetEvaluation,
			PathOf:    func(e *model.BatteryEvaluation) string { return e.ReportPdfPath },
			Writer:    repo.UpdateReportPath,
			Prepare:   prepareSuggestions,
			Render: func(_ context.Context, e *model.BatteryEvaluation) ([]byte, error) {
				return pdf.GenerateBatteryReportBytes(e)
			},
		}),
	}
}

// Create 处理 POST /api/valuation/battery/evaluations
// 接收循环充放电数据 → 调用 service 预测 → 持久化 → 返回 RUL/SOH
// @Summary 提交电池 RUL 评估
// @Description 接收循环充放电数据 → 预测 RUL/SOH → 持久化。**可选认证**：匿名可提交（user_id 落 NULL），带 Bearer 则归属当前用户。
// @Tags 估值-电池
// @Accept json
// @Produce json
// @Param body body model.CreateBatteryRequest true "电池循环数据（至少 10 个完整循环）"
// @Success 200 {object} response.R{data=model.CreateBatteryResponse} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/battery/evaluations [post]
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

// List 处理 GET /api/valuation/battery/evaluations?battery_type=lfp
// 分页查询评估历史摘要
// @Summary 电池评估历史
// @Description 分页查询电池 RUL 评估历史摘要（可按电池类型筛选），仅返回当前登录用户的记录。需登录（估值鉴权组）。
// @Tags 估值-电池
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query integer false "页码（默认 1）"
// @Param page_size query integer false "每页条数（默认 20，上限 100）"
// @Param battery_type query string false "电池类型筛选（lfp / ncm / other）"
// @Success 200 {object} response.R{data=model.ListBatteryResponse} "success"
// @Failure 400 {object} response.R "电池类型非法"
// @Failure 401 {object} response.R "未认证"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/battery/evaluations [get]
func (h *BatteryHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	page, pageSize = paging.ClampMax(page, pageSize, 20, 100)
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

// Get 处理 GET /api/valuation/battery/evaluations/:id
// 查询评估详情（含周期特征），仅返回属于当前登录用户的记录
// @Summary 电池评估详情
// @Description 查询电池 RUL 评估详情（含 cycle_features 周期特征与 feature_importance），仅返回属于当前登录用户的记录。需登录（估值鉴权组）。
// @Tags 估值-电池
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path integer true "电池评估记录 ID"
// @Success 200 {object} response.R{data=model.BatteryEvaluation} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "电池评估记录不存在"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/battery/evaluations/{id} [get]
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
		h.prepareSuggestions(c.Request.Context(), eval)
	}

	response.Success(c, eval)
}

// GenerateReport 处理 POST /api/valuation/battery/evaluations/:id/report
// @Summary 生成电池报告
// @Description 触发后端生成电池评估 PDF 报告（落盘 + 回写 report_pdf_path），返回 PDF URL 与大小。公开端点：无需登录。
// @Tags 估值-报告
// @Accept json
// @Produce json
// @Param id path integer true "电池评估记录 ID"
// @Success 200 {object} response.R{data=object{evaluation_id=integer,pdf_url=string,file_size=integer}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 404 {object} response.R "电池评估记录不存在"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/battery/evaluations/{id}/report [post]
func (h *BatteryHandler) GenerateReport(c *gin.Context) {
	serveReportGenerate(c, h.coord, "电池评估记录不存在", h.logger)
}

// DownloadReport 处理 GET /api/valuation/battery/evaluations/:id/report
// @Summary 下载电池报告
// @Description 经 storage 代理流式返回电池评估 PDF（Content-Disposition attachment）。公开端点：无需登录。
// @Tags 估值-报告
// @Produce application/pdf
// @Param id path integer true "电池评估记录 ID"
// @Success 200 {file} file "PDF 二进制流"
// @Failure 400 {object} response.R "参数错误"
// @Failure 404 {object} response.R "电池评估记录不存在"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/battery/evaluations/{id}/report [get]
func (h *BatteryHandler) DownloadReport(c *gin.Context) {
	serveReportDownload(c, h.coord, h.storage, "电池评估记录不存在", h.logger)
}
