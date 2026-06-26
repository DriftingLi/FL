// Package handler 实现 HTTP 处理器
// 本文件：配置类接口（部件配置、品牌、系数）
package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
	"forklift-training/internal/valuation/service"
)

// ConfigHandler 配置类 HTTP 处理器
// 聚合部件配置、品牌、系数三类查询接口
type ConfigHandler struct {
	queries     *repository.Queries
	brandLoader *service.BrandLoader
	logger      *zap.Logger
}

// NewConfigHandler 构造配置处理器
func NewConfigHandler(q *repository.Queries, b *service.BrandLoader, l *zap.Logger) *ConfigHandler {
	return &ConfigHandler{queries: q, brandLoader: b, logger: l}
}

// ListPartConfigs 处理 GET /api/v1/part-configs?forklift_type=electric
// 返回指定叉车类型的所有部件配置（按类别与条目顺序）
func (h *ConfigHandler) ListPartConfigs(c *gin.Context) {
	ft := c.Query("forklift_type")
	if ft == "" {
		Error(c, http.StatusBadRequest, CodeBadRequest, "forklift_type 参数必填")
		return
	}
	if !model.ForkliftType(ft).IsValid() {
		Error(c, http.StatusBadRequest, CodeInvalidParam, "forklift_type 必须为 electric 或 combustion")
		return
	}

	rows, err := h.queries.ListPartConfigs(c.Request.Context(), ft)
	if err != nil {
		h.logger.Error("查询部件配置失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询部件配置失败")
		return
	}

	// 转换为 DTO
	out := make([]model.PartConfigInfo, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.PartConfigInfo{
			CategoryCode:   r.CategoryCode,
			CategoryName:   r.CategoryName,
			CategoryWeight: r.CategoryWeight,
			ItemCode:       r.ItemCode,
			ItemName:       r.ItemName,
			ItemWeight:     r.ItemWeight,
		})
	}
	OK(c, out)
}

// ListBrands 处理 GET /api/v1/brands
// 返回所有激活的品牌（从内存加载器读，不查 DB 以提速）
func (h *ConfigHandler) ListBrands(c *gin.Context) {
	OK(c, h.brandLoader.ListBrands())
}

// ListCoefficients 处理 GET /api/v1/coefficients
// 返回所有可调系数（衰减率、权重、市场系数等）
func (h *ConfigHandler) ListCoefficients(c *gin.Context) {
	rows, err := h.queries.ListCoefficientConfigs(c.Request.Context())
	if err != nil {
		h.logger.Error("查询系数配置失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询系数配置失败")
		return
	}

	out := make([]model.CoefficientInfo, 0, len(rows))
	for _, r := range rows {
		desc := ""
		if r.Description.Valid {
			desc = r.Description.String
		}
		out = append(out, model.CoefficientInfo{
			Key:         r.Key,
			Value:       r.Value,
			Description: desc,
		})
	}
	OK(c, out)
}

// UpdateCoefficient 处理 PUT /api/v1/coefficients/:key
// 更新某个系数值（管理用接口，不要求鉴权 - 内网环境）
// Body: {"value": 0.15}
func (h *ConfigHandler) UpdateCoefficient(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		Error(c, http.StatusBadRequest, CodeBadRequest, "系数 key 不能为空")
		return
	}

	var body struct {
		Value float64 `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求体格式错误: "+err.Error())
		return
	}

	row, err := h.queries.UpdateCoefficientValue(c.Request.Context(), repository.UpdateCoefficientValueParams{
		Key:   key,
		Value: body.Value,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "系数 key 不存在: "+key)
			return
		}
		h.logger.Error("更新系数失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "更新系数失败")
		return
	}
	OK(c, gin.H{
		"key":        row.Key,
		"value":      row.Value,
		"updated_at": row.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	})
}
