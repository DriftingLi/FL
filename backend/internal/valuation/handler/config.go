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
	"forklift-training/pkg/paging"
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
// @Summary 品牌字典
// @Description 返回全部品牌（按 k_brand 倒序）。公开端点：无需登录。
// @Tags 估值-字典
// @Accept json
// @Produce json
// @Success 200 {object} response.R{data=[]repository.Brand} "success"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/dictionaries/brands [get]
func (h *ConfigHandler) ListBrands(c *gin.Context) {
	list, err := h.dictRepo.ListBrands(c.Request.Context())
	if err != nil {
		h.logger.Error("查询品牌失败", zap.Error(err))
		response.ServerError(c, "查询品牌失败")
		return
	}
	response.Success(c, list)
}

// listCascadeOrFull 级联查询统一分派：级联参数全传时走级联查询，否则走全量列表。
// paramsOK 由调用方按该字典的级联依赖判定；errMsg 为失败时的统一错误文案。
func listCascadeOrFull[T any](h *ConfigHandler, c *gin.Context, paramsOK bool, cascade, full func(ctx context.Context) (T, error), errMsg string) {
	var (
		list T
		err  error
	)
	if paramsOK {
		list, err = cascade(c.Request.Context())
	} else {
		list, err = full(c.Request.Context())
	}
	if err != nil {
		h.logger.Error(errMsg, zap.Error(err))
		response.ServerError(c, errMsg)
		return
	}
	response.Success(c, list)
}

// ListVehicleTypes 处理 GET /api/valuation/dictionaries/vehicle-types?brand=林德
// brand 可选：传入时基于 original_prices 级联过滤；不传时返回全部车型
// @Summary 车型字典
// @Description 叉车车型列表；传 brand 时按 original_prices 级联过滤。公开端点：无需登录。
// @Tags 估值-字典
// @Accept json
// @Produce json
// @Param brand query string false "品牌名（级联过滤）"
// @Success 200 {object} response.R{data=[]repository.VehicleType} "success"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/dictionaries/vehicle-types [get]
func (h *ConfigHandler) ListVehicleTypes(c *gin.Context) {
	brand := c.Query("brand")
	listCascadeOrFull(h, c, brand != "",
		func(ctx context.Context) ([]repository.VehicleType, error) {
			return h.dictRepo.ListVehicleTypesByBrand(ctx, brand)
		},
		h.dictRepo.ListVehicleTypes,
		"查询车型失败")
}

// ListSeries 处理 GET /api/valuation/dictionaries/series?brand=林德&vehicle_type=电动平衡重式
// brand + vehicle_type 可选：同时传入时基于 original_prices 级联过滤
// @Summary 系列字典
// @Description 车辆系列列表；brand + vehicle_type 同时传入时按 original_prices 级联过滤。公开端点：无需登录。
// @Tags 估值-字典
// @Accept json
// @Produce json
// @Param brand query string false "品牌名"
// @Param vehicle_type query string false "车型名"
// @Success 200 {object} response.R{data=[]repository.Series} "success"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/dictionaries/series [get]
func (h *ConfigHandler) ListSeries(c *gin.Context) {
	brand := c.Query("brand")
	vehicleType := c.Query("vehicle_type")
	paramsOK := brand != "" && vehicleType != ""
	listCascadeOrFull(h, c, paramsOK,
		func(ctx context.Context) ([]repository.Series, error) {
			return h.dictRepo.ListSeriesByCascade(ctx, brand, vehicleType)
		},
		func(ctx context.Context) ([]repository.Series, error) {
			return h.dictRepo.ListSeries(ctx, brand)
		},
		"查询系列失败")
}

// ListTonnages 处理 GET /api/valuation/dictionaries/tonnages?brand=&vehicle_type=&series=
// 级联参数全传时基于 original_prices 过滤；否则返回全部吨位
// @Summary 吨位字典
// @Description 吨位列表；brand/vehicle_type/series 全传时按 original_prices 级联过滤。公开端点：无需登录。
// @Tags 估值-字典
// @Accept json
// @Produce json
// @Param brand query string false "品牌名"
// @Param vehicle_type query string false "车型名"
// @Param series query string false "系列名"
// @Success 200 {object} response.R{data=[]repository.Tonnage} "success"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/dictionaries/tonnages [get]
func (h *ConfigHandler) ListTonnages(c *gin.Context) {
	brand := c.Query("brand")
	vehicleType := c.Query("vehicle_type")
	series := c.Query("series")
	paramsOK := brand != "" && vehicleType != "" && series != ""
	listCascadeOrFull(h, c, paramsOK,
		func(ctx context.Context) ([]repository.Tonnage, error) {
			return h.dictRepo.ListTonnagesByCascade(ctx, brand, vehicleType, series)
		},
		h.dictRepo.ListTonnages,
		"查询吨位失败")
}

// ListConfigTypes 处理 GET /api/valuation/dictionaries/config-types?brand=&vehicle_type=&series=&tonnage=
// 级联参数全传时基于 original_prices 过滤；否则返回空数组
// @Summary 配置类型字典
// @Description 配置类型选项（original_prices DISTINCT 派生）；级联参数不全时返回空数组。公开端点：无需登录。
// @Tags 估值-字典
// @Accept json
// @Produce json
// @Param brand query string false "品牌名"
// @Param vehicle_type query string false "车型名"
// @Param series query string false "系列名"
// @Param tonnage query string false "吨位"
// @Success 200 {object} response.R{data=[]repository.ConfigOption} "success"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/dictionaries/config-types [get]
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
// @Summary 门架类型字典
// @Description 门架类型列表；级联参数全传时按 original_prices 过滤。公开端点：无需登录。
// @Tags 估值-字典
// @Accept json
// @Produce json
// @Param brand query string false "品牌名"
// @Param vehicle_type query string false "车型名"
// @Param series query string false "系列名"
// @Param tonnage query string false "吨位"
// @Param config_type query string false "配置类型"
// @Success 200 {object} response.R{data=[]repository.MastType} "success"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/dictionaries/mast-types [get]
func (h *ConfigHandler) ListMastTypes(c *gin.Context) {
	brand := c.Query("brand")
	vehicleType := c.Query("vehicle_type")
	series := c.Query("series")
	tonnage := c.Query("tonnage")
	configType := c.Query("config_type")
	paramsOK := brand != "" && vehicleType != "" && series != "" && tonnage != "" && configType != ""
	listCascadeOrFull(h, c, paramsOK,
		func(ctx context.Context) ([]repository.MastType, error) {
			return h.dictRepo.ListMastTypesByCascade(ctx, brand, vehicleType, series, tonnage, configType)
		},
		h.dictRepo.ListMastTypes,
		"查询门架类型失败")
}

// ListMastHeights 处理 GET /api/valuation/dictionaries/mast-heights?brand=&vehicle_type=&series=&tonnage=&config_type=&mast_type=
// 级联参数全传时基于 original_prices 过滤；否则返回全部门架高度
// @Summary 门架高度字典
// @Description 门架高度列表（value_mm）；级联参数全传时按 original_prices 过滤。公开端点：无需登录。
// @Tags 估值-字典
// @Accept json
// @Produce json
// @Param brand query string false "品牌名"
// @Param vehicle_type query string false "车型名"
// @Param series query string false "系列名"
// @Param tonnage query string false "吨位"
// @Param config_type query string false "配置类型"
// @Param mast_type query string false "门架类型"
// @Success 200 {object} response.R{data=[]repository.MastHeight} "success"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/dictionaries/mast-heights [get]
func (h *ConfigHandler) ListMastHeights(c *gin.Context) {
	brand := c.Query("brand")
	vehicleType := c.Query("vehicle_type")
	series := c.Query("series")
	tonnage := c.Query("tonnage")
	configType := c.Query("config_type")
	mastType := c.Query("mast_type")
	paramsOK := brand != "" && vehicleType != "" && series != "" && tonnage != "" && configType != "" && mastType != ""
	listCascadeOrFull(h, c, paramsOK,
		func(ctx context.Context) ([]repository.MastHeight, error) {
			return h.dictRepo.ListMastHeightsByCascade(ctx, brand, vehicleType, series, tonnage, configType, mastType)
		},
		h.dictRepo.ListMastHeights,
		"查询门架高度失败")
}

// ListBatteryTypes 处理 GET /api/valuation/dictionaries/battery-types?brand=&vehicle_type=&series=&tonnage=
// 级联参数全传时基于 original_prices 过滤；否则返回全部电池类型
// @Summary 电池类型字典
// @Description 电池类型列表；级联参数全传时按 original_prices 过滤。公开端点：无需登录。
// @Tags 估值-字典
// @Accept json
// @Produce json
// @Param brand query string false "品牌名"
// @Param vehicle_type query string false "车型名"
// @Param series query string false "系列名"
// @Param tonnage query string false "吨位"
// @Success 200 {object} response.R{data=[]repository.BatteryTypeDict} "success"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/dictionaries/battery-types [get]
func (h *ConfigHandler) ListBatteryTypes(c *gin.Context) {
	brand := c.Query("brand")
	vehicleType := c.Query("vehicle_type")
	series := c.Query("series")
	tonnage := c.Query("tonnage")
	paramsOK := brand != "" && vehicleType != "" && series != "" && tonnage != ""
	listCascadeOrFull(h, c, paramsOK,
		func(ctx context.Context) ([]repository.BatteryTypeDict, error) {
			return h.dictRepo.ListBatteryTypesByCascade(ctx, brand, vehicleType, series, tonnage)
		},
		h.dictRepo.ListBatteryTypes,
		"查询电池类型失败")
}

// ListTransmissionTypes 处理 GET /api/valuation/dictionaries/transmission-types
// @Summary 传动系统字典
// @Description 传动系统类型字典（手波/自波/无级变速/无）。公开端点：无需登录。
// @Tags 估值-字典
// @Accept json
// @Produce json
// @Success 200 {object} response.R{data=[]repository.TransmissionType} "success"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/dictionaries/transmission-types [get]
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
// @Summary 发动机类型字典
// @Description 发动机类型字典（国产/进口/混合动力/无）。公开端点：无需登录。
// @Tags 估值-字典
// @Accept json
// @Produce json
// @Success 200 {object} response.R{data=[]repository.EngineType} "success"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/dictionaries/engine-types [get]
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
// @Summary 系列配置选项
// @Description 指定 series 支持的三维度（传动/发动机/电池）可选项；数组为空表示该维度不适用。公开端点：无需登录。
// @Tags 估值-字典
// @Accept json
// @Produce json
// @Param brand query string true "品牌名"
// @Param series query string true "系列名"
// @Success 200 {object} response.R{data=repository.SeriesConfigOptions} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/dictionaries/series-config-options [get]
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
// @Summary 车况评级字典
// @Description 车况评级（A–E）与基础系数。公开端点：无需登录。
// @Tags 估值-字典
// @Accept json
// @Produce json
// @Success 200 {object} response.R{data=[]repository.ConditionRating} "success"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/dictionaries/condition-ratings [get]
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
// @Summary 区域系数字典
// @Description 区域系数列表；province 可选。公开端点：无需登录。
// @Tags 估值-字典
// @Accept json
// @Produce json
// @Param province query string false "省份名"
// @Success 200 {object} response.R{data=[]repository.RegionCoefficient} "success"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/dictionaries/region-coefficients [get]
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
// @Summary 省份列表
// @Description 全部省份（去重），用于省市级联。公开端点：无需登录。
// @Tags 估值-字典
// @Accept json
// @Produce json
// @Success 200 {object} response.R{data=[]string} "success"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/dictionaries/provinces [get]
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
// @Summary 城市列表
// @Description 指定省份的全部城市（去重）。公开端点：无需登录。
// @Tags 估值-字典
// @Accept json
// @Produce json
// @Param province query string true "省份名"
// @Success 200 {object} response.R{data=[]string} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/dictionaries/cities [get]
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
// @Summary 系数配置字典
// @Description 全局可调系数配置（学生端只读）。公开端点：无需登录。
// @Tags 估值-字典
// @Accept json
// @Produce json
// @Success 200 {object} response.R{data=[]repository.CoefficientConfig} "success"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/dictionaries/coefficient-configs [get]
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
// @Summary 原价记录分页
// @Description 分页查询基准原价记录（管理端原价表）。公开端点：无需登录。
// @Tags 估值-字典
// @Accept json
// @Produce json
// @Param page query integer false "页码（默认 1）"
// @Param page_size query integer false "每页条数（默认 20，上限 100）"
// @Success 200 {object} response.R{data=object{total=integer,page=integer,page_size=integer,list=[]repository.OriginalPrice}} "success"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/dictionaries/original-prices [get]
func (h *ConfigHandler) ListOriginalPrices(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	page, pageSize = paging.ClampMax(page, pageSize, 20, 100)
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
// @Summary 最早出厂年份
// @Description 该组合下 active 原价记录 earliest_factory_year 的最小值（出厂年份输入下限）。公开端点：无需登录。
// @Tags 估值-字典
// @Accept json
// @Produce json
// @Param brand query string true "品牌名"
// @Param vehicle_type query string true "车型名"
// @Param series query string false "系列名（为空或「其它」时降级查询）"
// @Param tonnage query string true "吨位（数字）"
// @Success 200 {object} response.R{data=object{earliest_factory_year=integer}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/dictionaries/earliest-factory-year [get]
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
// @Summary 算法参数聚合
// @Description 一次返回 4 类算法参数（全局系数 + 品牌系数 + 车况系数 + 区域系数）。公开端点：无需登录。
// @Tags 估值-字典
// @Accept json
// @Produce json
// @Success 200 {object} response.R{data=repository.AlgorithmParameters} "success"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/dictionaries/algorithm-parameters [get]
func (h *ConfigHandler) ListAlgorithmParameters(c *gin.Context) {
	result, err := h.dictRepo.ListAlgorithmParameters(c.Request.Context())
	if err != nil {
		h.logger.Error("查询算法参数失败", zap.Error(err))
		response.ServerError(c, "查询算法参数失败")
		return
	}
	response.Success(c, result)
}
