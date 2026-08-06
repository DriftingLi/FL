// Package handler 实现 HTTP 处理器
// 本文件：配置类接口（学生端字典只读 GET + 管理员 CRUD）
// 学生端：GET /api/valuation/dictionaries/*  返回各字典表数据（无需 admin 权限）
// 管理员端：/api/valuation/admin/*  对字典表进行增删改（要求 JWT role=admin）
package handler

import (
	"context"
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"forklift-training/internal/cache"
	"forklift-training/internal/valuation/repository"
	"forklift-training/pkg/response"
)

// ConfigHandler 配置类 HTTP 处理器
// 持有字典仓储，提供学生端字典查询与管理员 CRUD 接口
type ConfigHandler struct {
	dictRepo DictionaryConfigStore
	logger   *zap.Logger
}

// invalidateCache 失效指定 pattern 的缓存，失败仅记录日志不影响业务返回。
func (h *ConfigHandler) invalidateCache(ctx context.Context, patterns ...string) {
	for _, p := range patterns {
		if err := cache.InvalidatePattern(ctx, p); err != nil {
			h.logger.Warn("缓存失效失败", zap.String("pattern", p), zap.Error(err))
		}
	}
}

// resultInvalidation 组合字典实体失效 pattern 与评估结果缓存 pattern。
// pattern 集来自 repository 缓存契约（handler 不书写 pattern 字面量，见 repository/dict_cache_keys.go）。
func (h *ConfigHandler) resultInvalidation(patterns []string) []string {
	return append(patterns, repository.ResultCachePattern)
}

// NewConfigHandler 构造配置处理器
func NewConfigHandler(dictRepo DictionaryConfigStore, l *zap.Logger) *ConfigHandler {
	return &ConfigHandler{dictRepo: dictRepo, logger: l}
}

// =====================================================
// 字典 CRUD 共享骨架（bind/validate/Error/invalidateCache 三联收敛于此）
// =====================================================

// createNamed 单一 name 字段实体的创建骨架（门架类型/电池类型/传动/发动机）。
func (h *ConfigHandler) createNamed(c *gin.Context, entity string, patterns []string,
	create func(ctx context.Context, name string) (any, error)) {
	var body struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求体格式错误: "+err.Error())
		return
	}
	v, err := create(c.Request.Context(), body.Name)
	if err != nil {
		h.logger.Error("新增"+entity+"失败", zap.Error(err))
		response.ServerError(c, "新增"+entity+"失败")
		return
	}
	h.invalidateCache(c.Request.Context(), patterns...)
	response.Success(c, v)
}

// createValue 单值实体的创建骨架（吨位/门架高度，bind 闭包负责字段解析与校验）。
func (h *ConfigHandler) createValue(c *gin.Context, entity string, patterns []string,
	bind func(c *gin.Context) (any, bool), create func(ctx context.Context, v any) (any, error)) {
	body, ok := bind(c)
	if !ok {
		return
	}
	v, err := create(c.Request.Context(), body)
	if err != nil {
		h.logger.Error("新增"+entity+"失败", zap.Error(err))
		response.ServerError(c, "新增"+entity+"失败")
		return
	}
	h.invalidateCache(c.Request.Context(), patterns...)
	response.Success(c, v)
}

// deleteOne 删除类接口通用骨架：id 解析 → 删除 → ErrNoRows→404 → 缓存失效 → 返回。
func (h *ConfigHandler) deleteOne(c *gin.Context, entity, notFound string, patterns []string,
	del func(ctx context.Context, id int64) error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "id 必须为整数")
		return
	}
	if err := del(c.Request.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.NotFound(c, notFound)
			return
		}
		h.logger.Error("删除"+entity+"失败", zap.Error(err))
		response.ServerError(c, "删除"+entity+"失败")
		return
	}
	h.invalidateCache(c.Request.Context(), patterns...)
	response.Success(c, gin.H{"id": id})
}

// =====================================================
// 学生端字典查询接口（GET，无需 admin 权限）
// =====================================================

// ListBrands 处理 GET /api/valuation/dictionaries/brands
// 返回全部启用品牌（按 k_brand 倒序）
func (h *ConfigHandler) ListBrands(c *gin.Context) {
	list, err := h.dictRepo.ListBrands(c.Request.Context())
	if err != nil {
		h.logger.Error("查询品牌失败", zap.Error(err))
		response.ServerError(c, "查询品牌失败")
		return
	}
	response.Success(c, list)
}

// ListVehicleTypes 处理 GET /api/valuation/dictionaries/vehicle-types?brand=林德
// brand 可选：传入时基于 original_prices 级联过滤；不传时返回全部车型
func (h *ConfigHandler) ListVehicleTypes(c *gin.Context) {
	brand := c.Query("brand")
	if brand != "" {
		list, err := h.dictRepo.ListVehicleTypesByBrand(c.Request.Context(), brand)
		if err != nil {
			h.logger.Error("级联查询车型失败", zap.Error(err))
			response.ServerError(c, "查询车型失败")
			return
		}
		response.Success(c, list)
		return
	}
	list, err := h.dictRepo.ListVehicleTypes(c.Request.Context())
	if err != nil {
		h.logger.Error("查询车型失败", zap.Error(err))
		response.ServerError(c, "查询车型失败")
		return
	}
	response.Success(c, list)
}

// ListSeries 处理 GET /api/valuation/dictionaries/series?brand=林德&vehicle_type=电动平衡重式
// brand + vehicle_type 可选：同时传入时基于 original_prices 级联过滤
func (h *ConfigHandler) ListSeries(c *gin.Context) {
	brand := c.Query("brand")
	vehicleType := c.Query("vehicle_type")
	if brand != "" && vehicleType != "" {
		list, err := h.dictRepo.ListSeriesByCascade(c.Request.Context(), brand, vehicleType)
		if err != nil {
			h.logger.Error("级联查询系列失败", zap.Error(err))
			response.ServerError(c, "查询系列失败")
			return
		}
		response.Success(c, list)
		return
	}
	list, err := h.dictRepo.ListSeries(c.Request.Context(), brand)
	if err != nil {
		h.logger.Error("查询系列失败", zap.Error(err))
		response.ServerError(c, "查询系列失败")
		return
	}
	response.Success(c, list)
}

// ListTonnages 处理 GET /api/valuation/dictionaries/tonnages?brand=&vehicle_type=&series=
// 级联参数全传时基于 original_prices 过滤；否则返回全部吨位
func (h *ConfigHandler) ListTonnages(c *gin.Context) {
	brand := c.Query("brand")
	vehicleType := c.Query("vehicle_type")
	series := c.Query("series")
	if brand != "" && vehicleType != "" && series != "" {
		list, err := h.dictRepo.ListTonnagesByCascade(c.Request.Context(), brand, vehicleType, series)
		if err != nil {
			h.logger.Error("级联查询吨位失败", zap.Error(err))
			response.ServerError(c, "查询吨位失败")
			return
		}
		response.Success(c, list)
		return
	}
	list, err := h.dictRepo.ListTonnages(c.Request.Context())
	if err != nil {
		h.logger.Error("查询吨位失败", zap.Error(err))
		response.ServerError(c, "查询吨位失败")
		return
	}
	response.Success(c, list)
}

// ListConfigTypes 处理 GET /api/valuation/dictionaries/config-types?brand=&vehicle_type=&series=&tonnage=
// 级联参数全传时基于 original_prices 过滤；否则返回空数组
func (h *ConfigHandler) ListConfigTypes(c *gin.Context) {
	brand := c.Query("brand")
	vehicleType := c.Query("vehicle_type")
	series := c.Query("series")
	tonnage := c.Query("tonnage")
	if brand == "" || vehicleType == "" || series == "" || tonnage == "" {
		response.Success(c, []repository.ConfigOption{})
		return
	}
	list, err := h.dictRepo.ListConfigOptionsByCascade(c.Request.Context(), brand, vehicleType, series, tonnage)
	if err != nil {
		h.logger.Error("级联查询配置类型失败", zap.Error(err))
		response.ServerError(c, "查询配置类型失败")
		return
	}
	response.Success(c, list)
}

// ListMastTypes 处理 GET /api/valuation/dictionaries/mast-types?brand=&vehicle_type=&series=&tonnage=&config_type=
// 级联参数全传时基于 original_prices 过滤；否则返回全部门架类型
func (h *ConfigHandler) ListMastTypes(c *gin.Context) {
	brand := c.Query("brand")
	vehicleType := c.Query("vehicle_type")
	series := c.Query("series")
	tonnage := c.Query("tonnage")
	configType := c.Query("config_type")
	if brand != "" && vehicleType != "" && series != "" && tonnage != "" && configType != "" {
		list, err := h.dictRepo.ListMastTypesByCascade(c.Request.Context(), brand, vehicleType, series, tonnage, configType)
		if err != nil {
			h.logger.Error("级联查询门架类型失败", zap.Error(err))
			response.ServerError(c, "查询门架类型失败")
			return
		}
		response.Success(c, list)
		return
	}
	list, err := h.dictRepo.ListMastTypes(c.Request.Context())
	if err != nil {
		h.logger.Error("查询门架类型失败", zap.Error(err))
		response.ServerError(c, "查询门架类型失败")
		return
	}
	response.Success(c, list)
}

// ListMastHeights 处理 GET /api/valuation/dictionaries/mast-heights?brand=&vehicle_type=&series=&tonnage=&config_type=&mast_type=
// 级联参数全传时基于 original_prices 过滤；否则返回全部门架高度
func (h *ConfigHandler) ListMastHeights(c *gin.Context) {
	brand := c.Query("brand")
	vehicleType := c.Query("vehicle_type")
	series := c.Query("series")
	tonnage := c.Query("tonnage")
	configType := c.Query("config_type")
	mastType := c.Query("mast_type")
	if brand != "" && vehicleType != "" && series != "" && tonnage != "" && configType != "" && mastType != "" {
		list, err := h.dictRepo.ListMastHeightsByCascade(c.Request.Context(), brand, vehicleType, series, tonnage, configType, mastType)
		if err != nil {
			h.logger.Error("级联查询门架高度失败", zap.Error(err))
			response.ServerError(c, "查询门架高度失败")
			return
		}
		response.Success(c, list)
		return
	}
	list, err := h.dictRepo.ListMastHeights(c.Request.Context())
	if err != nil {
		h.logger.Error("查询门架高度失败", zap.Error(err))
		response.ServerError(c, "查询门架高度失败")
		return
	}
	response.Success(c, list)
}

// ListBatteryTypes 处理 GET /api/valuation/dictionaries/battery-types?brand=&vehicle_type=&series=&tonnage=
// 级联参数全传时基于 original_prices 过滤；否则返回全部电池类型
func (h *ConfigHandler) ListBatteryTypes(c *gin.Context) {
	brand := c.Query("brand")
	vehicleType := c.Query("vehicle_type")
	series := c.Query("series")
	tonnage := c.Query("tonnage")
	if brand != "" && vehicleType != "" && series != "" && tonnage != "" {
		list, err := h.dictRepo.ListBatteryTypesByCascade(c.Request.Context(), brand, vehicleType, series, tonnage)
		if err != nil {
			h.logger.Error("级联查询电池类型失败", zap.Error(err))
			response.ServerError(c, "查询电池类型失败")
			return
		}
		response.Success(c, list)
		return
	}
	list, err := h.dictRepo.ListBatteryTypes(c.Request.Context())
	if err != nil {
		h.logger.Error("查询电池类型失败", zap.Error(err))
		response.ServerError(c, "查询电池类型失败")
		return
	}
	response.Success(c, list)
}

// ListTransmissionTypes 处理 GET /api/valuation/dictionaries/transmission-types
func (h *ConfigHandler) ListTransmissionTypes(c *gin.Context) {
	list, err := h.dictRepo.ListTransmissionTypes(c.Request.Context())
	if err != nil {
		h.logger.Error("查询传动系统失败", zap.Error(err))
		response.ServerError(c, "查询传动系统失败")
		return
	}
	response.Success(c, list)
}

// ListEngineTypes 处理 GET /api/valuation/dictionaries/engine-types
func (h *ConfigHandler) ListEngineTypes(c *gin.Context) {
	list, err := h.dictRepo.ListEngineTypes(c.Request.Context())
	if err != nil {
		h.logger.Error("查询发动机类型失败", zap.Error(err))
		response.ServerError(c, "查询发动机类型失败")
		return
	}
	response.Success(c, list)
}

// ListSeriesConfigOptions 处理 GET /api/valuation/dictionaries/series-config-options?brand=&series=
// 返回指定 series 支持的三维度（传动/发动机/电池）可选项
func (h *ConfigHandler) ListSeriesConfigOptions(c *gin.Context) {
	brand := c.Query("brand")
	series := c.Query("series")
	if brand == "" || series == "" {
		response.BadRequest(c, "brand 和 series 参数必填")
		return
	}
	opts, err := h.dictRepo.ListSeriesConfigOptions(c.Request.Context(), brand, series)
	if err != nil {
		h.logger.Error("查询系列配置选项失败", zap.Error(err))
		response.ServerError(c, "查询系列配置选项失败")
		return
	}
	response.Success(c, opts)
}

// ListConditionRatings 处理 GET /api/valuation/dictionaries/condition-ratings
func (h *ConfigHandler) ListConditionRatings(c *gin.Context) {
	list, err := h.dictRepo.ListConditionRatings(c.Request.Context())
	if err != nil {
		h.logger.Error("查询车况评级失败", zap.Error(err))
		response.ServerError(c, "查询车况评级失败")
		return
	}
	response.Success(c, list)
}

// ListRegionCoefficients 处理 GET /api/valuation/dictionaries/region-coefficients?province=江苏
// province 可选，为空时返回全部区域系数
func (h *ConfigHandler) ListRegionCoefficients(c *gin.Context) {
	province := c.Query("province")
	list, err := h.dictRepo.ListRegionCoefficients(c.Request.Context(), province)
	if err != nil {
		h.logger.Error("查询区域系数失败", zap.Error(err))
		response.ServerError(c, "查询区域系数失败")
		return
	}
	response.Success(c, list)
}

// ListProvinces 处理 GET /api/valuation/dictionaries/provinces
// 返回全部省份（去重），用于前端省市级联
func (h *ConfigHandler) ListProvinces(c *gin.Context) {
	list, err := h.dictRepo.ListProvinces(c.Request.Context())
	if err != nil {
		h.logger.Error("查询省份失败", zap.Error(err))
		response.ServerError(c, "查询省份失败")
		return
	}
	response.Success(c, list)
}

// ListCities 处理 GET /api/valuation/dictionaries/cities?province=江苏
// 返回指定省份的全部城市
func (h *ConfigHandler) ListCities(c *gin.Context) {
	province := c.Query("province")
	if province == "" {
		response.BadRequest(c, "province 参数必填")
		return
	}
	list, err := h.dictRepo.ListCities(c.Request.Context(), province)
	if err != nil {
		h.logger.Error("查询城市失败", zap.Error(err))
		response.ServerError(c, "查询城市失败")
		return
	}
	response.Success(c, list)
}

// ListCoefficientConfigs 处理 GET /api/valuation/dictionaries/coefficient-configs
// 返回全部系数配置（学生端只读，仅用于查看默认值）
func (h *ConfigHandler) ListCoefficientConfigs(c *gin.Context) {
	list, err := h.dictRepo.ListCoefficientConfigs(c.Request.Context())
	if err != nil {
		h.logger.Error("查询系数配置失败", zap.Error(err))
		response.ServerError(c, "查询系数配置失败")
		return
	}
	response.Success(c, list)
}

// ListOriginalPrices 处理 GET /api/valuation/dictionaries/original-prices?page=1&page_size=20
// 分页查询基准原价记录
func (h *ConfigHandler) ListOriginalPrices(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	list, total, err := h.dictRepo.ListOriginalPrices(c.Request.Context(), pageSize, offset)
	if err != nil {
		h.logger.Error("查询原价记录失败", zap.Error(err))
		response.ServerError(c, "查询原价记录失败")
		return
	}
	response.Success(c, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"list":      list,
	})
}

// =====================================================
// 管理员 CRUD 接口（/api/valuation/admin/*，要求 JWT role=admin）
// =====================================================

// --- brands ---

// CreateBrand 处理 POST /api/valuation/admin/brands
// Body: {"name":"林德","k_brand":1.10,"is_active":true}
func (h *ConfigHandler) CreateBrand(c *gin.Context) {
	var body repository.Brand
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求体格式错误: "+err.Error())
		return
	}
	if body.Name == "" {
		response.BadRequest(c, "name 必填")
		return
	}
	b, err := h.dictRepo.CreateBrand(c.Request.Context(), body.Name, body.KBrand, body.IsActive)
	if err != nil {
		h.logger.Error("新增品牌失败", zap.Error(err))
		response.ServerError(c, "新增品牌失败")
		return
	}
	h.invalidateCache(c.Request.Context(), h.resultInvalidation(h.dictRepo.BrandInvalidationPatterns())...)
	response.Success(c, b)
}

// UpdateBrand 处理 PUT /api/valuation/admin/brands/:id
// Body: {"k_brand":1.12,"is_active":true}
func (h *ConfigHandler) UpdateBrand(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "id 必须为整数")
		return
	}
	var body struct {
		KBrand   float64 `json:"k_brand" binding:"required"`
		IsActive bool    `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求体格式错误: "+err.Error())
		return
	}
	if err := h.dictRepo.UpdateBrand(c.Request.Context(), id, body.KBrand, body.IsActive); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.NotFound(c, "品牌不存在")
			return
		}
		h.logger.Error("更新品牌失败", zap.Error(err))
		response.ServerError(c, "更新品牌失败")
		return
	}
	h.invalidateCache(c.Request.Context(), h.resultInvalidation(h.dictRepo.BrandInvalidationPatterns())...)
	response.Success(c, gin.H{"id": id, "k_brand": body.KBrand, "is_active": body.IsActive})
}

// DeleteBrand 处理 DELETE /api/valuation/admin/brands/:id
func (h *ConfigHandler) DeleteBrand(c *gin.Context) {
	h.deleteOne(c, "品牌", "品牌不存在", h.resultInvalidation(h.dictRepo.BrandInvalidationPatterns()),
		func(ctx context.Context, id int64) error { return h.dictRepo.DeleteBrand(ctx, id) })
}

// --- vehicle_types ---

// CreateVehicleType 处理 POST /api/valuation/admin/vehicle-types
// Body: {"name":"电动平衡重","power_type":"electric","earliest_factory_year":1995}
func (h *ConfigHandler) CreateVehicleType(c *gin.Context) {
	var body repository.VehicleType
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求体格式错误: "+err.Error())
		return
	}
	if body.Name == "" || body.PowerType == "" {
		response.BadRequest(c, "name 与 power_type 必填")
		return
	}
	if body.EarliestFactoryYear == 0 {
		body.EarliestFactoryYear = 1980
	}
	v, err := h.dictRepo.CreateVehicleType(c.Request.Context(), body.Name, body.PowerType, body.EarliestFactoryYear)
	if err != nil {
		h.logger.Error("新增车型失败", zap.Error(err))
		response.ServerError(c, "新增车型失败")
		return
	}
	h.invalidateCache(c.Request.Context(), h.resultInvalidation(h.dictRepo.VehicleTypeInvalidationPatterns())...)
	response.Success(c, v)
}

// UpdateVehicleType 处理 PUT /api/valuation/admin/vehicle-types/:id
// Body: {"name":"电动平衡重","power_type":"electric","earliest_factory_year":1995}
func (h *ConfigHandler) UpdateVehicleType(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "id 必须为整数")
		return
	}
	var body struct {
		Name                string `json:"name" binding:"required"`
		PowerType           string `json:"power_type" binding:"required"`
		EarliestFactoryYear int    `json:"earliest_factory_year"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求体格式错误: "+err.Error())
		return
	}
	if body.EarliestFactoryYear == 0 {
		body.EarliestFactoryYear = 1980
	}
	if err := h.dictRepo.UpdateVehicleType(c.Request.Context(), id, body.Name, body.PowerType, body.EarliestFactoryYear); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.NotFound(c, "车型不存在")
			return
		}
		h.logger.Error("更新车型失败", zap.Error(err))
		response.ServerError(c, "更新车型失败")
		return
	}
	h.invalidateCache(c.Request.Context(), h.resultInvalidation(h.dictRepo.VehicleTypeInvalidationPatterns())...)
	response.Success(c, gin.H{"id": id, "name": body.Name, "power_type": body.PowerType, "earliest_factory_year": body.EarliestFactoryYear})
}

// DeleteVehicleType 处理 DELETE /api/valuation/admin/vehicle-types/:id
func (h *ConfigHandler) DeleteVehicleType(c *gin.Context) {
	h.deleteOne(c, "车型", "车型不存在", h.resultInvalidation(h.dictRepo.VehicleTypeInvalidationPatterns()),
		func(ctx context.Context, id int64) error { return h.dictRepo.DeleteVehicleType(ctx, int(id)) })
}

// --- series ---

// CreateSeries 处理 POST /api/valuation/admin/series
// Body: {"brand":"林德","name":"E系列","earliest_factory_year":2015}
func (h *ConfigHandler) CreateSeries(c *gin.Context) {
	var body repository.Series
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求体格式错误: "+err.Error())
		return
	}
	if body.Brand == "" || body.Name == "" {
		response.BadRequest(c, "brand 与 name 必填")
		return
	}
	// earliest_factory_year 默认 2000
	if body.EarliestFactoryYear == 0 {
		body.EarliestFactoryYear = 2000
	}
	s, err := h.dictRepo.CreateSeries(c.Request.Context(), body.Brand, body.Name, body.EarliestFactoryYear)
	if err != nil {
		h.logger.Error("新增系列失败", zap.Error(err))
		response.ServerError(c, "新增系列失败")
		return
	}
	h.invalidateCache(c.Request.Context(), h.dictRepo.SeriesInvalidationPatterns()...)
	response.Success(c, s)
}

// UpdateSeries 处理 PUT /api/valuation/admin/series/:id
// Body: {"brand":"林德","name":"E系列","earliest_factory_year":2015}
func (h *ConfigHandler) UpdateSeries(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "id 必须为整数")
		return
	}
	var body struct {
		Brand               string `json:"brand" binding:"required"`
		Name                string `json:"name" binding:"required"`
		EarliestFactoryYear int    `json:"earliest_factory_year"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求体格式错误: "+err.Error())
		return
	}
	if body.EarliestFactoryYear == 0 {
		body.EarliestFactoryYear = 2000
	}
	if err := h.dictRepo.UpdateSeries(c.Request.Context(), id, body.Brand, body.Name, body.EarliestFactoryYear); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.NotFound(c, "系列不存在")
			return
		}
		h.logger.Error("更新系列失败", zap.Error(err))
		response.ServerError(c, "更新系列失败")
		return
	}
	h.invalidateCache(c.Request.Context(), h.dictRepo.SeriesInvalidationPatterns()...)
	response.Success(c, gin.H{"id": id, "brand": body.Brand, "name": body.Name, "earliest_factory_year": body.EarliestFactoryYear})
}

// DeleteSeries 处理 DELETE /api/valuation/admin/series/:id
func (h *ConfigHandler) DeleteSeries(c *gin.Context) {
	h.deleteOne(c, "系列", "系列不存在", h.dictRepo.SeriesInvalidationPatterns(),
		func(ctx context.Context, id int64) error { return h.dictRepo.DeleteSeries(ctx, int(id)) })
}

// --- tonnages ---

// CreateTonnage 处理 POST /api/valuation/admin/tonnages
// Body: {"value":3.0}
func (h *ConfigHandler) CreateTonnage(c *gin.Context) {
	h.createValue(c, "吨位", h.resultInvalidation(h.dictRepo.TonnageInvalidationPatterns()),
		func(c *gin.Context) (any, bool) {
			var body struct {
				Value float64 `json:"value" binding:"required"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				response.BadRequest(c, "请求体格式错误: "+err.Error())
				return nil, false
			}
			return body.Value, true
		},
		func(ctx context.Context, v any) (any, error) {
			return h.dictRepo.CreateTonnage(ctx, v.(float64))
		})
}

// DeleteTonnage 处理 DELETE /api/valuation/admin/tonnages/:id
func (h *ConfigHandler) DeleteTonnage(c *gin.Context) {
	h.deleteOne(c, "吨位", "吨位不存在", h.resultInvalidation(h.dictRepo.TonnageInvalidationPatterns()),
		func(ctx context.Context, id int64) error { return h.dictRepo.DeleteTonnage(ctx, int(id)) })
}

// --- mast_types ---

// CreateMastType 处理 POST /api/valuation/admin/mast-types
// Body: {"name":"三级门架"}
// CreateMastType 处理 POST /api/valuation/admin/mast-type-types
// Body: {"name":"xxx"}
func (h *ConfigHandler) CreateMastType(c *gin.Context) {
	h.createNamed(c, "门架类型", h.resultInvalidation(h.dictRepo.MastTypeInvalidationPatterns()),
		func(ctx context.Context, name string) (any, error) { return h.dictRepo.CreateMastType(ctx, name) })
}

// DeleteMastType 处理 DELETE /api/valuation/admin/mast-types/:id
func (h *ConfigHandler) DeleteMastType(c *gin.Context) {
	h.deleteOne(c, "门架类型", "门架类型不存在", h.resultInvalidation(h.dictRepo.MastTypeInvalidationPatterns()),
		func(ctx context.Context, id int64) error { return h.dictRepo.DeleteMastType(ctx, int(id)) })
}

// --- mast_heights ---

// CreateMastHeight 处理 POST /api/valuation/admin/mast-heights
// Body: {"value_mm":3000}
func (h *ConfigHandler) CreateMastHeight(c *gin.Context) {
	h.createValue(c, "门架高度", h.resultInvalidation(h.dictRepo.MastHeightInvalidationPatterns()),
		func(c *gin.Context) (any, bool) {
			var body struct {
				ValueMM int `json:"value_mm" binding:"required"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				response.BadRequest(c, "请求体格式错误: "+err.Error())
				return nil, false
			}
			return body.ValueMM, true
		},
		func(ctx context.Context, v any) (any, error) {
			return h.dictRepo.CreateMastHeight(ctx, v.(int))
		})
}

// DeleteMastHeight 处理 DELETE /api/valuation/admin/mast-heights/:id
func (h *ConfigHandler) DeleteMastHeight(c *gin.Context) {
	h.deleteOne(c, "门架高度", "门架高度不存在", h.resultInvalidation(h.dictRepo.MastHeightInvalidationPatterns()),
		func(ctx context.Context, id int64) error { return h.dictRepo.DeleteMastHeight(ctx, int(id)) })
}

// --- battery_types ---

// CreateBatteryType 处理 POST /api/valuation/admin/battery-types
// Body: {"name":"磷酸铁锂"}
// CreateBatteryType 处理 POST /api/valuation/admin/battery-type-types
// Body: {"name":"xxx"}
func (h *ConfigHandler) CreateBatteryType(c *gin.Context) {
	h.createNamed(c, "电池类型", h.resultInvalidation(h.dictRepo.BatteryTypeInvalidationPatterns()),
		func(ctx context.Context, name string) (any, error) { return h.dictRepo.CreateBatteryType(ctx, name) })
}

// DeleteBatteryType 处理 DELETE /api/valuation/admin/battery-types/:id
func (h *ConfigHandler) DeleteBatteryType(c *gin.Context) {
	h.deleteOne(c, "电池类型", "电池类型不存在", h.resultInvalidation(h.dictRepo.BatteryTypeInvalidationPatterns()),
		func(ctx context.Context, id int64) error { return h.dictRepo.DeleteBatteryType(ctx, int(id)) })
}

// --- transmission_types ---

// CreateTransmissionType 处理 POST /api/valuation/admin/transmission-types
// Body: {"name":"手波"}
// CreateTransmissionType 处理 POST /api/valuation/admin/transmission-type-types
// Body: {"name":"xxx"}
func (h *ConfigHandler) CreateTransmissionType(c *gin.Context) {
	h.createNamed(c, "传动系统类型", h.resultInvalidation(h.dictRepo.TransmissionTypeInvalidationPatterns()),
		func(ctx context.Context, name string) (any, error) {
			return h.dictRepo.CreateTransmissionType(ctx, name)
		})
}

// DeleteTransmissionType 处理 DELETE /api/valuation/admin/transmission-types/:id
func (h *ConfigHandler) DeleteTransmissionType(c *gin.Context) {
	h.deleteOne(c, "传动系统类型", "传动系统类型不存在", h.resultInvalidation(h.dictRepo.TransmissionTypeInvalidationPatterns()),
		func(ctx context.Context, id int64) error { return h.dictRepo.DeleteTransmissionType(ctx, int(id)) })
}

// --- engine_types ---

// CreateEngineType 处理 POST /api/valuation/admin/engine-types
// Body: {"name":"国产发动机"}
// CreateEngineType 处理 POST /api/valuation/admin/engine-type-types
// Body: {"name":"xxx"}
func (h *ConfigHandler) CreateEngineType(c *gin.Context) {
	h.createNamed(c, "发动机类型", h.resultInvalidation(h.dictRepo.EngineTypeInvalidationPatterns()),
		func(ctx context.Context, name string) (any, error) { return h.dictRepo.CreateEngineType(ctx, name) })
}

// DeleteEngineType 处理 DELETE /api/valuation/admin/engine-types/:id
func (h *ConfigHandler) DeleteEngineType(c *gin.Context) {
	h.deleteOne(c, "发动机类型", "发动机类型不存在", h.resultInvalidation(h.dictRepo.EngineTypeInvalidationPatterns()),
		func(ctx context.Context, id int64) error { return h.dictRepo.DeleteEngineType(ctx, int(id)) })
}

// --- condition_ratings ---

// CreateConditionRating 处理 POST /api/valuation/admin/condition-ratings
// Body: {"rating":"A","label":"优秀","base_coefficient":1.00}
func (h *ConfigHandler) CreateConditionRating(c *gin.Context) {
	var body repository.ConditionRating
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求体格式错误: "+err.Error())
		return
	}
	if body.Rating == "" || body.Label == "" {
		response.BadRequest(c, "rating 与 label 必填")
		return
	}
	cr, err := h.dictRepo.CreateConditionRating(c.Request.Context(), body.Rating, body.Label, body.BaseCoefficient)
	if err != nil {
		h.logger.Error("新增车况评级失败", zap.Error(err))
		response.ServerError(c, "新增车况评级失败")
		return
	}
	h.invalidateCache(c.Request.Context(), h.resultInvalidation(h.dictRepo.ConditionRatingInvalidationPatterns())...)
	response.Success(c, cr)
}

// UpdateConditionRating 处理 PUT /api/valuation/admin/condition-ratings/:id
// Body: {"label":"优秀","base_coefficient":1.00}
func (h *ConfigHandler) UpdateConditionRating(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "id 必须为整数")
		return
	}
	var body struct {
		Label           string  `json:"label" binding:"required"`
		BaseCoefficient float64 `json:"base_coefficient" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求体格式错误: "+err.Error())
		return
	}
	if err := h.dictRepo.UpdateConditionRating(c.Request.Context(), id, body.Label, body.BaseCoefficient); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.NotFound(c, "车况评级不存在")
			return
		}
		h.logger.Error("更新车况评级失败", zap.Error(err))
		response.ServerError(c, "更新车况评级失败")
		return
	}
	h.invalidateCache(c.Request.Context(), h.resultInvalidation(h.dictRepo.ConditionRatingInvalidationPatterns())...)
	response.Success(c, gin.H{"id": id, "label": body.Label, "base_coefficient": body.BaseCoefficient})
}

// DeleteConditionRating 处理 DELETE /api/valuation/admin/condition-ratings/:id
func (h *ConfigHandler) DeleteConditionRating(c *gin.Context) {
	h.deleteOne(c, "车况评级", "车况评级不存在", h.resultInvalidation(h.dictRepo.ConditionRatingInvalidationPatterns()),
		func(ctx context.Context, id int64) error { return h.dictRepo.DeleteConditionRating(ctx, int(id)) })
}

// --- region_coefficients ---

// CreateRegionCoefficient 处理 POST /api/valuation/admin/region-coefficients
// Body: {"province":"江苏","city":"苏州","coefficient":1.02}
func (h *ConfigHandler) CreateRegionCoefficient(c *gin.Context) {
	var body repository.RegionCoefficient
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求体格式错误: "+err.Error())
		return
	}
	if body.Province == "" || body.City == "" {
		response.BadRequest(c, "province 与 city 必填")
		return
	}
	rc, err := h.dictRepo.CreateRegionCoefficient(c.Request.Context(), body.Province, body.City, body.Coefficient)
	if err != nil {
		h.logger.Error("新增区域系数失败", zap.Error(err))
		response.ServerError(c, "新增区域系数失败")
		return
	}
	h.invalidateCache(c.Request.Context(), h.resultInvalidation(h.dictRepo.RegionCoefficientInvalidationPatterns())...)
	response.Success(c, rc)
}

// UpdateRegionCoefficient 处理 PUT /api/valuation/admin/region-coefficients/:id
// Body: {"coefficient":1.05}
func (h *ConfigHandler) UpdateRegionCoefficient(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "id 必须为整数")
		return
	}
	var body struct {
		Coefficient float64 `json:"coefficient" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求体格式错误: "+err.Error())
		return
	}
	if err := h.dictRepo.UpdateRegionCoefficient(c.Request.Context(), id, body.Coefficient); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.NotFound(c, "区域系数不存在")
			return
		}
		h.logger.Error("更新区域系数失败", zap.Error(err))
		response.ServerError(c, "更新区域系数失败")
		return
	}
	h.invalidateCache(c.Request.Context(), h.resultInvalidation(h.dictRepo.RegionCoefficientInvalidationPatterns())...)
	response.Success(c, gin.H{"id": id, "coefficient": body.Coefficient})
}

// DeleteRegionCoefficient 处理 DELETE /api/valuation/admin/region-coefficients/:id
func (h *ConfigHandler) DeleteRegionCoefficient(c *gin.Context) {
	h.deleteOne(c, "区域系数", "区域系数不存在", h.resultInvalidation(h.dictRepo.RegionCoefficientInvalidationPatterns()),
		func(ctx context.Context, id int64) error { return h.dictRepo.DeleteRegionCoefficient(ctx, int(id)) })
}

// --- original_prices ---

// CreateOriginalPrice 处理 POST /api/valuation/admin/original-prices
// Body: 完整 original_prices 行（不含 id 与 updated_at）
func (h *ConfigHandler) CreateOriginalPrice(c *gin.Context) {
	var body repository.OriginalPrice
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求体格式错误: "+err.Error())
		return
	}
	if body.Brand == "" || body.VehicleType == "" || body.Series == "" {
		response.BadRequest(c, "brand/vehicle_type/series 必填")
		return
	}
	if body.OriginalPrice <= 0 {
		response.BadRequest(c, "original_price 必须大于 0")
		return
	}
	if body.EarliestFactoryYear == 0 {
		body.EarliestFactoryYear = 2000
	}
	id, err := h.dictRepo.CreateOriginalPrice(c.Request.Context(), &body)
	if err != nil {
		h.logger.Error("新增原价记录失败", zap.Error(err))
		response.ServerError(c, "新增原价记录失败")
		return
	}
	body.ID = id
	h.invalidateCache(c.Request.Context(), h.resultInvalidation(h.dictRepo.OriginalPriceInvalidationPatterns())...)
	response.Success(c, body)
}

// UpdateOriginalPrice 处理 PUT /api/valuation/admin/original-prices/:id
// Body: 完整 original_prices 行（不含 id 与 updated_at，由后端忽略/覆盖）
// 支持修改全部 7 个唯一约束字段 + original_price
func (h *ConfigHandler) UpdateOriginalPrice(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "id 必须为整数")
		return
	}
	var body repository.OriginalPrice
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求体格式错误: "+err.Error())
		return
	}
	if body.Brand == "" || body.VehicleType == "" || body.Series == "" {
		response.BadRequest(c, "brand/vehicle_type/series 必填")
		return
	}
	if body.OriginalPrice <= 0 {
		response.BadRequest(c, "original_price 必须大于 0")
		return
	}
	if body.EarliestFactoryYear == 0 {
		body.EarliestFactoryYear = 2000
	}
	body.ID = id
	if err := h.dictRepo.UpdateOriginalPrice(c.Request.Context(), &body); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.NotFound(c, "原价记录不存在")
			return
		}
		h.logger.Error("更新原价记录失败", zap.Error(err))
		response.ServerError(c, "更新原价记录失败")
		return
	}
	h.invalidateCache(c.Request.Context(), h.resultInvalidation(h.dictRepo.OriginalPriceInvalidationPatterns())...)
	response.Success(c, body)
}

// DeleteOriginalPrice 处理 DELETE /api/valuation/admin/original-prices/:id
func (h *ConfigHandler) DeleteOriginalPrice(c *gin.Context) {
	h.deleteOne(c, "原价记录", "原价记录不存在", h.resultInvalidation(h.dictRepo.OriginalPriceInvalidationPatterns()),
		func(ctx context.Context, id int64) error { return h.dictRepo.DeleteOriginalPrice(ctx, id) })
}

// GetEarliestFactoryYear 处理 GET /api/valuation/dictionaries/earliest-factory-year
// Query: brand, vehicle_type, series(可选), tonnage
// 返回该组合下 active 原价记录 earliest_factory_year 的最小值，作为学生端出厂年份输入下限
// series 为空或"其它"时忽略 series 条件做降级查询
func (h *ConfigHandler) GetEarliestFactoryYear(c *gin.Context) {
	brand := c.Query("brand")
	vehicleType := c.Query("vehicle_type")
	series := c.Query("series")
	tonnageStr := c.Query("tonnage")
	if brand == "" || vehicleType == "" || tonnageStr == "" {
		response.BadRequest(c, "brand/vehicle_type/tonnage 必填")
		return
	}
	tonnage, err := strconv.ParseFloat(tonnageStr, 64)
	if err != nil {
		response.BadRequest(c, "tonnage 必须为数字")
		return
	}
	year, err := h.dictRepo.GetEarliestFactoryYearByCascade(c.Request.Context(), brand, vehicleType, series, tonnage)
	if err != nil {
		h.logger.Error("查询最早出厂年份失败", zap.Error(err))
		response.ServerError(c, "查询最早出厂年份失败")
		return
	}
	response.Success(c, gin.H{"earliest_factory_year": year})
}

// --- coefficient_configs ---

// UpdateCoefficient 处理 PUT /api/valuation/admin/coefficient-configs/:key
// Body: {"value":0.15}
func (h *ConfigHandler) UpdateCoefficient(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		response.BadRequest(c, "系数 key 不能为空")
		return
	}
	var body struct {
		Value float64 `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求体格式错误: "+err.Error())
		return
	}
	cc, err := h.dictRepo.UpdateCoefficientByKey(c.Request.Context(), key, body.Value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.NotFound(c, "系数 key 不存在: "+key)
			return
		}
		h.logger.Error("更新系数失败", zap.Error(err))
		response.ServerError(c, "更新系数失败")
		return
	}
	h.invalidateCache(c.Request.Context(), h.resultInvalidation(h.dictRepo.CoefficientInvalidationPatterns())...)
	response.Success(c, cc)
}

// --- algorithm_parameters ---

// ListAlgorithmParameters 处理 GET /api/valuation/dictionaries/algorithm-parameters
// 聚合返回 4 类算法参数（全局系数 + 品牌系数 + 车况系数 + 区域系数），供管理员后台一次加载
func (h *ConfigHandler) ListAlgorithmParameters(c *gin.Context) {
	result, err := h.dictRepo.ListAlgorithmParameters(c.Request.Context())
	if err != nil {
		h.logger.Error("查询算法参数失败", zap.Error(err))
		response.ServerError(c, "查询算法参数失败")
		return
	}
	response.Success(c, result)
}
