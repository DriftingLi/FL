// Package handler 实现 HTTP 处理器
// 本文件：评估相关接口（提交计算、查询详情、列表）
// 重构后采用手写 pgx 仓储，service.Persist 持久化评估结果
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
	"forklift-training/internal/valuation/service"
)

// EvaluationHandler 评估 HTTP 处理器
// 持有 valuation service（执行残值计算 + 持久化）与 evalRepo（查询详情 / 列表）
type EvaluationHandler struct {
	valuation *service.ValuationService
	evalRepo  *repository.EvaluationRepository
	logger    *zap.Logger
}

// NewEvaluationHandler 构造评估处理器
func NewEvaluationHandler(v *service.ValuationService, evalRepo *repository.EvaluationRepository, l *zap.Logger) *EvaluationHandler {
	return &EvaluationHandler{valuation: v, evalRepo: evalRepo, logger: l}
}

// Create 处理 POST /api/valuation/evaluations
// 提交评估请求：调用 service.Evaluate → service.Persist 持久化 → 返回计算结果
func (h *EvaluationHandler) Create(c *gin.Context) {
	var req model.EvaluationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求参数格式错误: "+err.Error())
		return
	}

	// 1. 调用 service 计算残值（service 内已做业务校验）
	result, err := h.valuation.Evaluate(c.Request.Context(), &req)
	if err != nil {
		// 业务校验失败：返回 400 + 业务错误码
		Error(c, http.StatusBadRequest, CodeInvalidParam, err.Error())
		return
	}

	// 2. 持久化评估结果到 evaluations 表
	id, err := h.valuation.Persist(c.Request.Context(), result)
	if err != nil {
		h.logger.Error("保存评估记录失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "保存评估记录失败")
		return
	}

	// 3. 返回响应（ID + 全部 K 系数 + 残值 + 置信区间 + 维度评分 + 建议）
	OK(c, buildEvaluationResponse(id, result))
}

// Get 处理 GET /api/valuation/evaluations/:id
// 查询评估详情：输入参数 + 计算结果 + 时间戳
func (h *EvaluationHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}

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

	// 详情接口直接返回持久化记录（已含全部输入字段 + 计算结果 + 报告路径）
	OK(c, detail)
}

// List 处理 GET /api/valuation/evaluations?page=1&page_size=20&brand=合力
// 分页查询评估历史（可按品牌筛选）
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

	// 1. 查询总数
	total, err := h.evalRepo.CountEvaluations(c.Request.Context(), brand)
	if err != nil {
		h.logger.Error("统计评估记录失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询评估列表失败")
		return
	}

	// 2. 查询当前页列表
	list, err := h.evalRepo.ListEvaluations(c.Request.Context(), brand, pageSize, offset)
	if err != nil {
		h.logger.Error("查询评估列表失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询评估列表失败")
		return
	}

	// 3. 返回分页响应
	OK(c, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"list":      list,
	})
}

// buildEvaluationResponse 把 EvaluationResult + 持久化 ID 转换为响应 DTO
// 维度评分顺序与雷达图保持一致：时间维度 / 使用强度 / 品牌 / 车况 / 市场
func buildEvaluationResponse(id int64, r *model.EvaluationResult) model.EvaluationResponse {
	// 维度评分 map → 切片（保持固定顺序）
	dimScores := make([]model.DimensionScore, 0, len(r.DimensionScores))
	for _, label := range []string{"时间维度", "使用强度", "品牌", "车况", "市场"} {
		if v, ok := r.DimensionScores[label]; ok {
			dimScores = append(dimScores, model.DimensionScore{Label: label, Value: v})
		}
	}
	// 兜底：若维度评分缺失，返回空切片（避免 JSON null）
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
		EstimatedValue:  r.EstimatedValue,
		ConfidenceLow:   r.ConfidenceLow,
		ConfidenceHigh:  r.ConfidenceHigh,
		DimensionScores: dimScores,
		Suggestions:     suggestions,
	}
}
