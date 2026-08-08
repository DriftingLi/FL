// Package handler 实现 HTTP 处理器
// 本文件：配置类接口（学生端字典只读 GET + 管理员 CRUD）
// 学生端：GET /api/valuation/dictionaries/*  返回各字典表数据（无需 admin 权限）
// 管理员端：/api/valuation/admin/*  对字典表进行增删改（要求 JWT role=admin）
//
// 管理员 CRUD 写面已全部迁至描述符驱动核心（dictcrud 包 + registerDictCRUDRoutes，
// 见 ADR-0008）：本文件只保留学生端只读查询；写路由由 router.go 经描述符注册表统一注册。
package handler

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"
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

// NewConfigHandler 构造配置处理器
func NewConfigHandler(dictRepo DictionaryConfigStore, l *zap.Logger) *ConfigHandler {
	return &ConfigHandler{dictRepo: dictRepo, logger: l}
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
// 管理员 CRUD 写面（/api/valuation/admin/*，要求 JWT role=admin）
// 全部由描述符注册表驱动（dictcrud 包 + registerDictCRUDRoutes，见 router.go），
// 不再逐实体手写骨架。
// =====================================================

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
