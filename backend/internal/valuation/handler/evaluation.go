// Package handler 实现 HTTP 处理器
// 本文件：评估相关接口（提交计算、查询详情、列表）
// 重构后采用手写 pgx 仓储，service.Persist 持久化评估结果
package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"forklift-training/internal/middleware"
	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/service"
	"forklift-training/pkg/response"
)

// EvaluationHandler 评估 HTTP 处理器
// 持有 valuation service（执行残值计算 + 持久化）与 evalRepo（查询详情 / 列表）
type EvaluationHandler struct {
	valuation *service.ValuationService
	evalRepo  EvaluationStore
	logger    *zap.Logger
}

// NewEvaluationHandler 构造评估处理器
func NewEvaluationHandler(v *service.ValuationService, evalRepo EvaluationStore, l *zap.Logger) *EvaluationHandler {
	return &EvaluationHandler{valuation: v, evalRepo: evalRepo, logger: l}
}

// Create 处理 POST /api/valuation/evaluations
// 提交评估请求：调用 service.Evaluate → service.Persist 持久化 → 返回计算结果
// 走可选认证：登录用户提交时记录 user_id，匿名提交 user_id 为 NULL
func (h *EvaluationHandler) Create(c *gin.Context) {
	var req model.EvaluationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数格式错误: "+err.Error())
		return
	}

	// 1. 调用 service 计算残值（service 内已做业务校验）
	result, err := h.valuation.Evaluate(c.Request.Context(), &req)
	if err != nil {
		// 业务校验失败：返回 400 + 业务错误码
		response.BadRequest(c, err.Error())
		return
	}

	// 2. 持久化评估结果到 evaluations 表（带上当前登录用户 ID，未登录为 0→NULL）
	userID := middleware.CurrentUserID(c)
	id, err := h.valuation.Persist(c.Request.Context(), result, userID)
	if err != nil {
		h.logger.Error("保存评估记录失败", zap.Error(err))
		response.ServerError(c, "保存评估记录失败")
		return
	}

	// 3. 返回响应（ID + 全部 K 系数 + 残值 + 置信区间 + 维度评分 + 建议）
	response.Success(c, buildEvaluationResponse(id, result))
}

// Get 处理 GET /api/valuation/evaluations/:id
// 查询评估详情：输入参数 + 计算结果 + 时间戳
// 仅返回属于当前登录用户的记录（不属于自己 → 404）
// KTimeAdjusted 不入库，读取时实时由 KTime/KHours/KBrand 重算
func (h *EvaluationHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "id 必须为整数")
		return
	}

	userID := middleware.CurrentUserID(c)
	detail, err := h.evalRepo.GetEvaluationByUser(c.Request.Context(), id, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.NotFound(c, "评估记录不存在")
			return
		}
		h.logger.Error("查询评估记录失败", zap.Error(err), zap.Int64("id", id))
		response.ServerError(c, "查询评估记录失败")
		return
	}

	// 重建派生字段（单一装配点：KTimeAdjusted + 维度分；建议为评估时点锁定值，ADR-0004）
	service.RebuildDerivedFromDetail(detail)

	// 详情接口直接返回持久化记录（已含全部输入字段 + 计算结果 + 报告路径）
	response.Success(c, detail)
}

// List 处理 GET /api/valuation/evaluations?page=1&page_size=20&brand=合力
// 分页查询评估历史（可按品牌筛选），仅返回当前登录用户的记录
func (h *EvaluationHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// 品牌筛选参数（为空时不过滤）
	brand := c.Query("brand")
	// 仅查询当前登录用户的记录（List 在鉴权组，userID 必然 >0）
	userID := middleware.CurrentUserID(c)

	// 1. 查询总数
	total, err := h.evalRepo.CountEvaluations(c.Request.Context(), brand, userID)
	if err != nil {
		h.logger.Error("统计评估记录失败", zap.Error(err))
		response.ServerError(c, "查询评估列表失败")
		return
	}

	// 2. 查询当前页列表
	list, err := h.evalRepo.ListEvaluations(c.Request.Context(), brand, userID, pageSize, offset)
	if err != nil {
		h.logger.Error("查询评估列表失败", zap.Error(err))
		response.ServerError(c, "查询评估列表失败")
		return
	}

	// 2.1 重建派生字段（KTimeAdjusted 不入库字段，单一装配点）
	for i := range list {
		service.RebuildDerivedFromDetail(&list[i])
	}

	// 3. 返回分页响应
	response.Success(c, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"list":      list,
	})
}

// Stats 处理 GET /api/valuation/evaluations/stats
// 返回累计评估次数（公开统计全部记录，userID=0 不过滤）
func (h *EvaluationHandler) Stats(c *gin.Context) {
	total, err := h.evalRepo.CountEvaluations(c.Request.Context(), "", 0)
	if err != nil {
		h.logger.Error("统计评估次数失败", zap.Error(err))
		response.ServerError(c, "查询统计数据失败")
		return
	}
	response.Success(c, gin.H{"total": total})
}

// buildEvaluationResponse 把 EvaluationResult + 持久化 ID 转换为响应 DTO
// 维度评分顺序与雷达图保持一致（由 service.BuildDimensionScores 单一装配，此处不再排序）
func buildEvaluationResponse(id int64, r *model.EvaluationResult) model.EvaluationResponse {
	// 兜底：若维度评分缺失，返回空切片（避免 JSON null）
	dimScores := r.DimensionScores
	if dimScores == nil {
		dimScores = []model.DimensionScore{}
	}
	suggestions := r.Suggestions
	if suggestions == nil {
		suggestions = []string{}
	}
	return model.EvaluationResponse{
		ID:              id,
		OriginalPrice:   r.OriginalPrice,
		KTime:           r.KTime,
		KHours:          r.KHours,
		KBrand:          r.KBrand,
		KCondition:      r.KCondition,
		KMarket:         r.KMarket,
		KTimeAdjusted:   r.KTimeAdjusted,
		EstimatedValue:  r.EstimatedValue,
		ConfidenceLow:   r.ConfidenceLow,
		ConfidenceHigh:  r.ConfidenceHigh,
		DimensionScores: dimScores,
		Suggestions:     suggestions,
		// 评估时点锁定的 λ 值（供前端走势图数据驱动）
		LambdaElectric:   r.LambdaElectric,
		LambdaCombustion: r.LambdaCombustion,
	}
}
