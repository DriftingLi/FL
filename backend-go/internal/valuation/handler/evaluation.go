// Package handler 实现 HTTP 处理器
// 本文件：评估相关接口（提交计算、查询详情、列表）
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
	"forklift-training/internal/valuation/service"
)

// EvaluationHandler 评估 HTTP 处理器
type EvaluationHandler struct {
	queries   *repository.Queries
	valuation *service.ValuationService
	logger    *zap.Logger
}

// NewEvaluationHandler 构造评估处理器
func NewEvaluationHandler(q *repository.Queries, v *service.ValuationService, l *zap.Logger) *EvaluationHandler {
	return &EvaluationHandler{queries: q, valuation: v, logger: l}
}

// Create 处理 POST /api/v1/evaluations
// 提交评估请求：调用 service 计算 → 持久化到数据库 → 返回计算结果
func (h *EvaluationHandler) Create(c *gin.Context) {
	var req model.EvaluationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求参数格式错误: "+err.Error())
		return
	}

	// 1. 调用 service 计算（service 内已做业务校验）
	result, err := h.valuation.Evaluate(c.Request.Context(), &req)
	if err != nil {
		// 业务校验失败：返回 400 + 业务错误码
		Error(c, http.StatusBadRequest, CodeInvalidParam, err.Error())
		return
	}

	// 2. 持久化主表
	modelText := pgtype.Text{}
	if result.Model != "" {
		modelText = pgtype.Text{String: result.Model, Valid: true}
	}
	fuelText := pgtype.Text{}
	if result.FuelType != "" {
		fuelText = pgtype.Text{String: string(result.FuelType), Valid: true}
	}
	createParams := repository.CreateEvaluationParams{
		ForkliftType:   string(result.ForkliftType),
		Brand:          result.Brand,
		Model:          modelText,
		OriginalPrice:  result.OriginalPrice,
		PurchaseYear:   int32(result.PurchaseYear),
		SaleYear:       int32(result.SaleYear),
		UsageHours:     int32(result.UsageHours),
		WorkCondition:  string(result.WorkCondition),
		FuelType:       fuelText,
		CanDrive:       result.CanDrive,
		HydraulicOk:    result.HydraulicOk,
		KTime:          result.KTime,
		KHours:         result.KHours,
		KWork:          result.KWork,
		KBrand:         result.KBrand,
		KCondition:     result.KCondition,
		KMarket:        result.KMarket,
		EstimatedValue: result.EstimatedValue,
		ConfidenceLow:  result.ConfidenceLow,
		ConfidenceHigh: result.ConfidenceHigh,
	}
	row, err := h.queries.CreateEvaluation(c.Request.Context(), createParams)
	if err != nil {
		h.logger.Error("创建评估记录失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "保存评估记录失败")
		return
	}

	// 3. 持久化部件状态
	for _, it := range result.Items {
		_, err := h.queries.CreateEvaluationItem(c.Request.Context(), repository.CreateEvaluationItemParams{
			EvaluationID:   row.ID,
			CategoryCode:   it.CategoryCode,
			CategoryName:   it.CategoryName,
			ItemCode:       it.ItemCode,
			ItemName:       it.ItemName,
			Status:         string(it.Status),
			CategoryWeight: it.CategoryWeight,
			ItemWeight:     it.ItemWeight,
			Score:          it.Score,
		})
		if err != nil {
			h.logger.Error("保存部件状态失败", zap.Error(err), zap.Int64("evaluation_id", row.ID))
		}
	}

	// 4. 返回响应
	OK(c, model.EvaluationResponse{
		ID:             row.ID,
		KTime:          result.KTime,
		KHours:         result.KHours,
		KWork:          result.KWork,
		KBrand:         result.KBrand,
		KCondition:     result.KCondition,
		KMarket:        result.KMarket,
		EstimatedValue: result.EstimatedValue,
		ConfidenceLow:  result.ConfidenceLow,
		ConfidenceHigh: result.ConfidenceHigh,
		OriginalPrice:  result.OriginalPrice,
		DimensionScores: result.DimensionScores,
		Suggestions:     result.Suggestions,
	})
}

// Get 处理 GET /api/v1/evaluations/:id
// 查询评估详情（输入参数 + 计算结果 + 部件状态）
func (h *EvaluationHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}

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

	items, err := h.queries.ListEvaluationItems(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("查询部件状态失败", zap.Error(err), zap.Int64("id", id))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询部件状态失败")
		return
	}

	// 转换为响应 DTO
	resp := convertEvaluationToResponse(eval)
	resp.Items = convertItemsToDTO(items)
	// 补全派生字段：维度评分 + 文本建议（从持久化行 + items 重建，不重新跑完整算法）
	dimScores, suggestions := service.ReconstructFromRow(eval, convertItemsToItemResults(items))
	resp.DimensionScores = dimScores
	resp.Suggestions = suggestions
	OK(c, resp)
}

// List 处理 GET /api/v1/evaluations?page=1&page_size=20&forklift_type=electric
// 分页查询评估历史
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

	// 构造筛选参数
	forkliftType := pgtype.Text{}
	if ft := c.Query("forklift_type"); ft != "" {
		forkliftType = pgtype.Text{String: ft, Valid: true}
	}

	// 查询总数
	total, err := h.queries.CountEvaluations(c.Request.Context(), forkliftType)
	if err != nil {
		h.logger.Error("统计评估记录失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询评估列表失败")
		return
	}

	// 查询列表
	rows, err := h.queries.ListEvaluations(c.Request.Context(), repository.ListEvaluationsParams{
		Limit:        int32(pageSize),
		Offset:       int32(offset),
		ForkliftType: forkliftType,
	})
	if err != nil {
		h.logger.Error("查询评估列表失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询评估列表失败")
		return
	}

	// 转换响应
	list := make([]model.EvaluationDetailResponse, 0, len(rows))
	for _, e := range rows {
		list = append(list, convertEvaluationToResponse(e))
	}

	OK(c, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"list":      list,
	})
}

// convertEvaluationToResponse 数据库实体 → 响应 DTO
func convertEvaluationToResponse(e repository.Evaluation) model.EvaluationDetailResponse {
	return model.EvaluationDetailResponse{
		ID:             e.ID,
		ForkliftType:   model.ForkliftType(e.ForkliftType),
		Brand:          e.Brand,
		Model:          textOrEmpty(e.Model),
		OriginalPrice:  e.OriginalPrice,
		PurchaseYear:   int(e.PurchaseYear),
		SaleYear:       int(e.SaleYear),
		UsageHours:     int(e.UsageHours),
		WorkCondition:  model.WorkCondition(e.WorkCondition),
		FuelType:       model.FuelType(textOrEmpty(e.FuelType)),
		CanDrive:       e.CanDrive,
		HydraulicOk:    e.HydraulicOk,
		KTime:          e.KTime,
		KHours:         e.KHours,
		KWork:          e.KWork,
		KBrand:         e.KBrand,
		KCondition:     e.KCondition,
		KMarket:        e.KMarket,
		EstimatedValue: e.EstimatedValue,
		ConfidenceLow:  e.ConfidenceLow,
		ConfidenceHigh: e.ConfidenceHigh,
		ReportPdfPath:  textOrEmpty(e.ReportPdfPath),
		CreatedAt:      formatPgTime(e.CreatedAt),
		UpdatedAt:      formatPgTime(e.UpdatedAt),
	}
}

// convertItemsToDTO 部件实体 → DTO
func convertItemsToDTO(items []repository.EvaluationItem) []model.EvaluationItemDTO {
	out := make([]model.EvaluationItemDTO, 0, len(items))
	for _, it := range items {
		out = append(out, model.EvaluationItemDTO{
			ID:             it.ID,
			CategoryCode:   it.CategoryCode,
			CategoryName:   it.CategoryName,
			ItemCode:       it.ItemCode,
			ItemName:       it.ItemName,
			Status:         model.ItemStatus(it.Status),
			CategoryWeight: it.CategoryWeight,
			ItemWeight:     it.ItemWeight,
			Score:          it.Score,
		})
	}
	return out
}

// convertItemsToItemResults 部件实体 → model.ItemResult（供 service.ReconstructFromRow 重建建议）
func convertItemsToItemResults(items []repository.EvaluationItem) []model.ItemResult {
	out := make([]model.ItemResult, 0, len(items))
	for _, it := range items {
		out = append(out, model.ItemResult{
			CategoryCode:   it.CategoryCode,
			CategoryName:   it.CategoryName,
			ItemCode:       it.ItemCode,
			ItemName:       it.ItemName,
			Status:         model.ItemStatus(it.Status),
			CategoryWeight: it.CategoryWeight,
			ItemWeight:     it.ItemWeight,
			Score:          it.Score,
		})
	}
	return out
}

// textOrEmpty pgtype.Text → string（NULL 转为空串）
func textOrEmpty(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

// formatPgTime pgtype.Timestamp → RFC3339 字符串
func formatPgTime(t pgtype.Timestamp) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format("2006-01-02T15:04:05Z07:00")
}
