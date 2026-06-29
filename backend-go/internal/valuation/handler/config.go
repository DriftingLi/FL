// Package handler 实现 HTTP 处理器
// 本文件：配置类接口（学生端字典只读 GET + 管理员 CRUD）
// 学生端：GET /api/valuation/dictionaries/*  返回各字典表数据（无需 admin 权限）
// 管理员端：/api/valuation/admin/*  对字典表进行增删改（要求 JWT role=admin）
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"forklift-training/internal/valuation/repository"
)

// ConfigHandler 配置类 HTTP 处理器
// 持有字典仓储，提供学生端字典查询与管理员 CRUD 接口
type ConfigHandler struct {
	dictRepo *repository.DictionaryRepository
	logger   *zap.Logger
}

// NewConfigHandler 构造配置处理器
func NewConfigHandler(dictRepo *repository.DictionaryRepository, l *zap.Logger) *ConfigHandler {
	return &ConfigHandler{dictRepo: dictRepo, logger: l}
}

// =====================================================
// 学生端字典查询接口（GET，无需 admin 权限）
// =====================================================

// ListBrandTypes 处理 GET /api/valuation/dictionaries/brand-types
func (h *ConfigHandler) ListBrandTypes(c *gin.Context) {
	list, err := h.dictRepo.ListBrandTypes(c.Request.Context())
	if err != nil {
		h.logger.Error("查询品牌类型失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询品牌类型失败")
		return
	}
	OK(c, list)
}

// ListBrands 处理 GET /api/valuation/dictionaries/brands?brand_type=进口一线
// brand_type 可选，为空时返回全部品牌
func (h *ConfigHandler) ListBrands(c *gin.Context) {
	brandType := c.Query("brand_type")
	list, err := h.dictRepo.ListBrands(c.Request.Context(), brandType)
	if err != nil {
		h.logger.Error("查询品牌失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询品牌失败")
		return
	}
	OK(c, list)
}

// ListVehicleTypes 处理 GET /api/valuation/dictionaries/vehicle-types?brand=林德
// brand 可选：传入时基于 original_prices 级联过滤；不传时返回全部车型
func (h *ConfigHandler) ListVehicleTypes(c *gin.Context) {
	brand := c.Query("brand")
	if brand != "" {
		list, err := h.dictRepo.ListVehicleTypesByBrand(c.Request.Context(), brand)
		if err != nil {
			h.logger.Error("级联查询车型失败", zap.Error(err))
			Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询车型失败")
			return
		}
		OK(c, list)
		return
	}
	list, err := h.dictRepo.ListVehicleTypes(c.Request.Context())
	if err != nil {
		h.logger.Error("查询车型失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询车型失败")
		return
	}
	OK(c, list)
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
			Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询系列失败")
			return
		}
		OK(c, list)
		return
	}
	list, err := h.dictRepo.ListSeries(c.Request.Context(), brand)
	if err != nil {
		h.logger.Error("查询系列失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询系列失败")
		return
	}
	OK(c, list)
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
			Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询吨位失败")
			return
		}
		OK(c, list)
		return
	}
	list, err := h.dictRepo.ListTonnages(c.Request.Context())
	if err != nil {
		h.logger.Error("查询吨位失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询吨位失败")
		return
	}
	OK(c, list)
}

// ListConfigTypes 处理 GET /api/valuation/dictionaries/config-types?brand=&vehicle_type=&series=&tonnage=
// 级联参数全传时基于 original_prices 过滤；否则返回全部配置类型
func (h *ConfigHandler) ListConfigTypes(c *gin.Context) {
	brand := c.Query("brand")
	vehicleType := c.Query("vehicle_type")
	series := c.Query("series")
	tonnage := c.Query("tonnage")
	if brand != "" && vehicleType != "" && series != "" && tonnage != "" {
		list, err := h.dictRepo.ListConfigTypesByCascade(c.Request.Context(), brand, vehicleType, series, tonnage)
		if err != nil {
			h.logger.Error("级联查询配置类型失败", zap.Error(err))
			Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询配置类型失败")
			return
		}
		OK(c, list)
		return
	}
	list, err := h.dictRepo.ListConfigTypes(c.Request.Context())
	if err != nil {
		h.logger.Error("查询配置类型失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询配置类型失败")
		return
	}
	OK(c, list)
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
			Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询门架类型失败")
			return
		}
		OK(c, list)
		return
	}
	list, err := h.dictRepo.ListMastTypes(c.Request.Context())
	if err != nil {
		h.logger.Error("查询门架类型失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询门架类型失败")
		return
	}
	OK(c, list)
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
			Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询门架高度失败")
			return
		}
		OK(c, list)
		return
	}
	list, err := h.dictRepo.ListMastHeights(c.Request.Context())
	if err != nil {
		h.logger.Error("查询门架高度失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询门架高度失败")
		return
	}
	OK(c, list)
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
			Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询电池类型失败")
			return
		}
		OK(c, list)
		return
	}
	list, err := h.dictRepo.ListBatteryTypes(c.Request.Context())
	if err != nil {
		h.logger.Error("查询电池类型失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询电池类型失败")
		return
	}
	OK(c, list)
}

// ListTransmissionTypes 处理 GET /api/valuation/dictionaries/transmission-types
func (h *ConfigHandler) ListTransmissionTypes(c *gin.Context) {
	list, err := h.dictRepo.ListTransmissionTypes(c.Request.Context())
	if err != nil {
		h.logger.Error("查询传动系统失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询传动系统失败")
		return
	}
	OK(c, list)
}

// ListEngineTypes 处理 GET /api/valuation/dictionaries/engine-types
func (h *ConfigHandler) ListEngineTypes(c *gin.Context) {
	list, err := h.dictRepo.ListEngineTypes(c.Request.Context())
	if err != nil {
		h.logger.Error("查询发动机类型失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询发动机类型失败")
		return
	}
	OK(c, list)
}

// ListSeriesConfigOptions 处理 GET /api/valuation/dictionaries/series-config-options?brand=&series=
// 返回指定 series 支持的三维度（传动/发动机/电池）可选项
func (h *ConfigHandler) ListSeriesConfigOptions(c *gin.Context) {
	brand := c.Query("brand")
	series := c.Query("series")
	if brand == "" || series == "" {
		Error(c, http.StatusBadRequest, CodeBadRequest, "brand 和 series 参数必填")
		return
	}
	opts, err := h.dictRepo.ListSeriesConfigOptions(c.Request.Context(), brand, series)
	if err != nil {
		h.logger.Error("查询系列配置选项失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询系列配置选项失败")
		return
	}
	OK(c, opts)
}

// ListConditionRatings 处理 GET /api/valuation/dictionaries/condition-ratings
func (h *ConfigHandler) ListConditionRatings(c *gin.Context) {
	list, err := h.dictRepo.ListConditionRatings(c.Request.Context())
	if err != nil {
		h.logger.Error("查询车况评级失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询车况评级失败")
		return
	}
	OK(c, list)
}

// ListRegionCoefficients 处理 GET /api/valuation/dictionaries/region-coefficients?province=江苏
// province 可选，为空时返回全部区域系数
func (h *ConfigHandler) ListRegionCoefficients(c *gin.Context) {
	province := c.Query("province")
	list, err := h.dictRepo.ListRegionCoefficients(c.Request.Context(), province)
	if err != nil {
		h.logger.Error("查询区域系数失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询区域系数失败")
		return
	}
	OK(c, list)
}

// ListProvinces 处理 GET /api/valuation/dictionaries/provinces
// 返回全部省份（去重），用于前端省市级联
func (h *ConfigHandler) ListProvinces(c *gin.Context) {
	list, err := h.dictRepo.ListProvinces(c.Request.Context())
	if err != nil {
		h.logger.Error("查询省份失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询省份失败")
		return
	}
	OK(c, list)
}

// ListCities 处理 GET /api/valuation/dictionaries/cities?province=江苏
// 返回指定省份的全部城市
func (h *ConfigHandler) ListCities(c *gin.Context) {
	province := c.Query("province")
	if province == "" {
		Error(c, http.StatusBadRequest, CodeBadRequest, "province 参数必填")
		return
	}
	list, err := h.dictRepo.ListCities(c.Request.Context(), province)
	if err != nil {
		h.logger.Error("查询城市失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询城市失败")
		return
	}
	OK(c, list)
}

// ListCoefficientConfigs 处理 GET /api/valuation/dictionaries/coefficient-configs
// 返回全部系数配置（学生端只读，仅用于查看默认值）
func (h *ConfigHandler) ListCoefficientConfigs(c *gin.Context) {
	list, err := h.dictRepo.ListCoefficientConfigs(c.Request.Context())
	if err != nil {
		h.logger.Error("查询系数配置失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询系数配置失败")
		return
	}
	OK(c, list)
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
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询原价记录失败")
		return
	}
	OK(c, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"list":      list,
	})
}

// =====================================================
// 管理员 CRUD 接口（/api/valuation/admin/*，要求 JWT role=admin）
// =====================================================

// --- brand_types ---

// CreateBrandType 处理 POST /api/valuation/admin/brand-types
// Body: {"name":"进口一线","k_type":1.10}
func (h *ConfigHandler) CreateBrandType(c *gin.Context) {
	var body struct {
		Name  string  `json:"name" binding:"required"`
		KType float64 `json:"k_type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	bt, err := h.dictRepo.CreateBrandType(c.Request.Context(), body.Name, body.KType)
	if err != nil {
		h.logger.Error("新增品牌类型失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "新增品牌类型失败")
		return
	}
	OK(c, bt)
}

// UpdateBrandType 处理 PUT /api/valuation/admin/brand-types/:name
// Body: {"k_type":1.12}
func (h *ConfigHandler) UpdateBrandType(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		Error(c, http.StatusBadRequest, CodeBadRequest, "品牌类型 name 不能为空")
		return
	}
	var body struct {
		KType float64 `json:"k_type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	if err := h.dictRepo.UpdateBrandType(c.Request.Context(), name, body.KType); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "品牌类型不存在: "+name)
			return
		}
		h.logger.Error("更新品牌类型失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "更新品牌类型失败")
		return
	}
	OK(c, gin.H{"name": name, "k_type": body.KType})
}

// DeleteBrandType 处理 DELETE /api/valuation/admin/brand-types/:name
func (h *ConfigHandler) DeleteBrandType(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		Error(c, http.StatusBadRequest, CodeBadRequest, "品牌类型 name 不能为空")
		return
	}
	if err := h.dictRepo.DeleteBrandType(c.Request.Context(), name); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "品牌类型不存在: "+name)
			return
		}
		h.logger.Error("删除品牌类型失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "删除品牌类型失败")
		return
	}
	OK(c, gin.H{"name": name})
}

// --- brands ---

// CreateBrand 处理 POST /api/valuation/admin/brands
// Body: {"name":"林德","brand_type":"进口一线","k_brand":1.10,"is_active":true}
func (h *ConfigHandler) CreateBrand(c *gin.Context) {
	var body repository.Brand
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	if body.Name == "" || body.BrandType == "" {
		Error(c, http.StatusBadRequest, CodeBadRequest, "name 与 brand_type 必填")
		return
	}
	b, err := h.dictRepo.CreateBrand(c.Request.Context(), body.Name, body.BrandType, body.KBrand, body.IsActive)
	if err != nil {
		h.logger.Error("新增品牌失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "新增品牌失败")
		return
	}
	OK(c, b)
}

// UpdateBrand 处理 PUT /api/valuation/admin/brands/:id
// Body: {"brand_type":"进口一线","k_brand":1.12,"is_active":true}
func (h *ConfigHandler) UpdateBrand(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}
	var body struct {
		BrandType string  `json:"brand_type" binding:"required"`
		KBrand    float64 `json:"k_brand" binding:"required"`
		IsActive  bool    `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	if err := h.dictRepo.UpdateBrand(c.Request.Context(), id, body.BrandType, body.KBrand, body.IsActive); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "品牌不存在")
			return
		}
		h.logger.Error("更新品牌失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "更新品牌失败")
		return
	}
	OK(c, gin.H{"id": id, "brand_type": body.BrandType, "k_brand": body.KBrand, "is_active": body.IsActive})
}

// DeleteBrand 处理 DELETE /api/valuation/admin/brands/:id
func (h *ConfigHandler) DeleteBrand(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}
	if err := h.dictRepo.DeleteBrand(c.Request.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "品牌不存在")
			return
		}
		h.logger.Error("删除品牌失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "删除品牌失败")
		return
	}
	OK(c, gin.H{"id": id})
}

// --- vehicle_types ---

// CreateVehicleType 处理 POST /api/valuation/admin/vehicle-types
// Body: {"name":"电动平衡重","power_type":"electric","earliest_factory_year":1995}
func (h *ConfigHandler) CreateVehicleType(c *gin.Context) {
	var body repository.VehicleType
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	if body.Name == "" || body.PowerType == "" {
		Error(c, http.StatusBadRequest, CodeBadRequest, "name 与 power_type 必填")
		return
	}
	if body.EarliestFactoryYear == 0 {
		body.EarliestFactoryYear = 1980
	}
	v, err := h.dictRepo.CreateVehicleType(c.Request.Context(), body.Name, body.PowerType, body.EarliestFactoryYear)
	if err != nil {
		h.logger.Error("新增车型失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "新增车型失败")
		return
	}
	OK(c, v)
}

// UpdateVehicleType 处理 PUT /api/valuation/admin/vehicle-types/:id
// Body: {"name":"电动平衡重","power_type":"electric","earliest_factory_year":1995}
func (h *ConfigHandler) UpdateVehicleType(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}
	var body struct {
		Name                string `json:"name" binding:"required"`
		PowerType           string `json:"power_type" binding:"required"`
		EarliestFactoryYear int    `json:"earliest_factory_year"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	if body.EarliestFactoryYear == 0 {
		body.EarliestFactoryYear = 1980
	}
	if err := h.dictRepo.UpdateVehicleType(c.Request.Context(), id, body.Name, body.PowerType, body.EarliestFactoryYear); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "车型不存在")
			return
		}
		h.logger.Error("更新车型失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "更新车型失败")
		return
	}
	OK(c, gin.H{"id": id, "name": body.Name, "power_type": body.PowerType, "earliest_factory_year": body.EarliestFactoryYear})
}

// DeleteVehicleType 处理 DELETE /api/valuation/admin/vehicle-types/:id
func (h *ConfigHandler) DeleteVehicleType(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}
	if err := h.dictRepo.DeleteVehicleType(c.Request.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "车型不存在")
			return
		}
		h.logger.Error("删除车型失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "删除车型失败")
		return
	}
	OK(c, gin.H{"id": id})
}

// --- series ---

// CreateSeries 处理 POST /api/valuation/admin/series
// Body: {"brand":"林德","name":"E系列","earliest_factory_year":2015}
func (h *ConfigHandler) CreateSeries(c *gin.Context) {
	var body repository.Series
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	if body.Brand == "" || body.Name == "" {
		Error(c, http.StatusBadRequest, CodeBadRequest, "brand 与 name 必填")
		return
	}
	// earliest_factory_year 默认 2000
	if body.EarliestFactoryYear == 0 {
		body.EarliestFactoryYear = 2000
	}
	s, err := h.dictRepo.CreateSeries(c.Request.Context(), body.Brand, body.Name, body.EarliestFactoryYear)
	if err != nil {
		h.logger.Error("新增系列失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "新增系列失败")
		return
	}
	OK(c, s)
}

// UpdateSeries 处理 PUT /api/valuation/admin/series/:id
// Body: {"brand":"林德","name":"E系列","earliest_factory_year":2015}
func (h *ConfigHandler) UpdateSeries(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}
	var body struct {
		Brand               string `json:"brand" binding:"required"`
		Name                string `json:"name" binding:"required"`
		EarliestFactoryYear int    `json:"earliest_factory_year"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	if body.EarliestFactoryYear == 0 {
		body.EarliestFactoryYear = 2000
	}
	if err := h.dictRepo.UpdateSeries(c.Request.Context(), id, body.Brand, body.Name, body.EarliestFactoryYear); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "系列不存在")
			return
		}
		h.logger.Error("更新系列失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "更新系列失败")
		return
	}
	OK(c, gin.H{"id": id, "brand": body.Brand, "name": body.Name, "earliest_factory_year": body.EarliestFactoryYear})
}

// DeleteSeries 处理 DELETE /api/valuation/admin/series/:id
func (h *ConfigHandler) DeleteSeries(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}
	if err := h.dictRepo.DeleteSeries(c.Request.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "系列不存在")
			return
		}
		h.logger.Error("删除系列失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "删除系列失败")
		return
	}
	OK(c, gin.H{"id": id})
}

// --- tonnages ---

// CreateTonnage 处理 POST /api/valuation/admin/tonnages
// Body: {"value":3.0}
func (h *ConfigHandler) CreateTonnage(c *gin.Context) {
	var body struct {
		Value float64 `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	t, err := h.dictRepo.CreateTonnage(c.Request.Context(), body.Value)
	if err != nil {
		h.logger.Error("新增吨位失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "新增吨位失败")
		return
	}
	OK(c, t)
}

// DeleteTonnage 处理 DELETE /api/valuation/admin/tonnages/:id
func (h *ConfigHandler) DeleteTonnage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}
	if err := h.dictRepo.DeleteTonnage(c.Request.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "吨位不存在")
			return
		}
		h.logger.Error("删除吨位失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "删除吨位失败")
		return
	}
	OK(c, gin.H{"id": id})
}

// --- config_types ---

// CreateConfigType 处理 POST /api/valuation/admin/config-types
// Body: {"name":"标准配置"}
func (h *ConfigHandler) CreateConfigType(c *gin.Context) {
	var body struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	ct, err := h.dictRepo.CreateConfigType(c.Request.Context(), body.Name)
	if err != nil {
		h.logger.Error("新增配置类型失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "新增配置类型失败")
		return
	}
	OK(c, ct)
}

// DeleteConfigType 处理 DELETE /api/valuation/admin/config-types/:id
func (h *ConfigHandler) DeleteConfigType(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}
	if err := h.dictRepo.DeleteConfigType(c.Request.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "配置类型不存在")
			return
		}
		h.logger.Error("删除配置类型失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "删除配置类型失败")
		return
	}
	OK(c, gin.H{"id": id})
}

// --- mast_types ---

// CreateMastType 处理 POST /api/valuation/admin/mast-types
// Body: {"name":"三级门架"}
func (h *ConfigHandler) CreateMastType(c *gin.Context) {
	var body struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	m, err := h.dictRepo.CreateMastType(c.Request.Context(), body.Name)
	if err != nil {
		h.logger.Error("新增门架类型失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "新增门架类型失败")
		return
	}
	OK(c, m)
}

// DeleteMastType 处理 DELETE /api/valuation/admin/mast-types/:id
func (h *ConfigHandler) DeleteMastType(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}
	if err := h.dictRepo.DeleteMastType(c.Request.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "门架类型不存在")
			return
		}
		h.logger.Error("删除门架类型失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "删除门架类型失败")
		return
	}
	OK(c, gin.H{"id": id})
}

// --- mast_heights ---

// CreateMastHeight 处理 POST /api/valuation/admin/mast-heights
// Body: {"value_mm":3000}
func (h *ConfigHandler) CreateMastHeight(c *gin.Context) {
	var body struct {
		ValueMM int `json:"value_mm" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	m, err := h.dictRepo.CreateMastHeight(c.Request.Context(), body.ValueMM)
	if err != nil {
		h.logger.Error("新增门架高度失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "新增门架高度失败")
		return
	}
	OK(c, m)
}

// DeleteMastHeight 处理 DELETE /api/valuation/admin/mast-heights/:id
func (h *ConfigHandler) DeleteMastHeight(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}
	if err := h.dictRepo.DeleteMastHeight(c.Request.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "门架高度不存在")
			return
		}
		h.logger.Error("删除门架高度失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "删除门架高度失败")
		return
	}
	OK(c, gin.H{"id": id})
}

// --- battery_types ---

// CreateBatteryType 处理 POST /api/valuation/admin/battery-types
// Body: {"name":"磷酸铁锂"}
func (h *ConfigHandler) CreateBatteryType(c *gin.Context) {
	var body struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	b, err := h.dictRepo.CreateBatteryType(c.Request.Context(), body.Name)
	if err != nil {
		h.logger.Error("新增电池类型失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "新增电池类型失败")
		return
	}
	OK(c, b)
}

// DeleteBatteryType 处理 DELETE /api/valuation/admin/battery-types/:id
func (h *ConfigHandler) DeleteBatteryType(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}
	if err := h.dictRepo.DeleteBatteryType(c.Request.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "电池类型不存在")
			return
		}
		h.logger.Error("删除电池类型失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "删除电池类型失败")
		return
	}
	OK(c, gin.H{"id": id})
}

// --- transmission_types ---

// CreateTransmissionType 处理 POST /api/valuation/admin/transmission-types
// Body: {"name":"手波"}
func (h *ConfigHandler) CreateTransmissionType(c *gin.Context) {
	var body struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	t, err := h.dictRepo.CreateTransmissionType(c.Request.Context(), body.Name)
	if err != nil {
		h.logger.Error("新增传动系统类型失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "新增传动系统类型失败")
		return
	}
	OK(c, t)
}

// DeleteTransmissionType 处理 DELETE /api/valuation/admin/transmission-types/:id
func (h *ConfigHandler) DeleteTransmissionType(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}
	if err := h.dictRepo.DeleteTransmissionType(c.Request.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "传动系统类型不存在")
			return
		}
		h.logger.Error("删除传动系统类型失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "删除传动系统类型失败")
		return
	}
	OK(c, gin.H{"id": id})
}

// --- engine_types ---

// CreateEngineType 处理 POST /api/valuation/admin/engine-types
// Body: {"name":"国产发动机"}
func (h *ConfigHandler) CreateEngineType(c *gin.Context) {
	var body struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	e, err := h.dictRepo.CreateEngineType(c.Request.Context(), body.Name)
	if err != nil {
		h.logger.Error("新增发动机类型失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "新增发动机类型失败")
		return
	}
	OK(c, e)
}

// DeleteEngineType 处理 DELETE /api/valuation/admin/engine-types/:id
func (h *ConfigHandler) DeleteEngineType(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}
	if err := h.dictRepo.DeleteEngineType(c.Request.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "发动机类型不存在")
			return
		}
		h.logger.Error("删除发动机类型失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "删除发动机类型失败")
		return
	}
	OK(c, gin.H{"id": id})
}

// --- condition_ratings ---

// CreateConditionRating 处理 POST /api/valuation/admin/condition-ratings
// Body: {"rating":"A","label":"优秀","base_coefficient":1.00}
func (h *ConfigHandler) CreateConditionRating(c *gin.Context) {
	var body repository.ConditionRating
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	if body.Rating == "" || body.Label == "" {
		Error(c, http.StatusBadRequest, CodeBadRequest, "rating 与 label 必填")
		return
	}
	cr, err := h.dictRepo.CreateConditionRating(c.Request.Context(), body.Rating, body.Label, body.BaseCoefficient)
	if err != nil {
		h.logger.Error("新增车况评级失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "新增车况评级失败")
		return
	}
	OK(c, cr)
}

// UpdateConditionRating 处理 PUT /api/valuation/admin/condition-ratings/:id
// Body: {"label":"优秀","base_coefficient":1.00}
func (h *ConfigHandler) UpdateConditionRating(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}
	var body struct {
		Label          string  `json:"label" binding:"required"`
		BaseCoefficient float64 `json:"base_coefficient" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	if err := h.dictRepo.UpdateConditionRating(c.Request.Context(), id, body.Label, body.BaseCoefficient); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "车况评级不存在")
			return
		}
		h.logger.Error("更新车况评级失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "更新车况评级失败")
		return
	}
	OK(c, gin.H{"id": id, "label": body.Label, "base_coefficient": body.BaseCoefficient})
}

// DeleteConditionRating 处理 DELETE /api/valuation/admin/condition-ratings/:id
func (h *ConfigHandler) DeleteConditionRating(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}
	if err := h.dictRepo.DeleteConditionRating(c.Request.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "车况评级不存在")
			return
		}
		h.logger.Error("删除车况评级失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "删除车况评级失败")
		return
	}
	OK(c, gin.H{"id": id})
}

// --- region_coefficients ---

// CreateRegionCoefficient 处理 POST /api/valuation/admin/region-coefficients
// Body: {"province":"江苏","city":"苏州","coefficient":1.02}
func (h *ConfigHandler) CreateRegionCoefficient(c *gin.Context) {
	var body repository.RegionCoefficient
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	if body.Province == "" || body.City == "" {
		Error(c, http.StatusBadRequest, CodeBadRequest, "province 与 city 必填")
		return
	}
	rc, err := h.dictRepo.CreateRegionCoefficient(c.Request.Context(), body.Province, body.City, body.Coefficient)
	if err != nil {
		h.logger.Error("新增区域系数失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "新增区域系数失败")
		return
	}
	OK(c, rc)
}

// UpdateRegionCoefficient 处理 PUT /api/valuation/admin/region-coefficients/:id
// Body: {"coefficient":1.05}
func (h *ConfigHandler) UpdateRegionCoefficient(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}
	var body struct {
		Coefficient float64 `json:"coefficient" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	if err := h.dictRepo.UpdateRegionCoefficient(c.Request.Context(), id, body.Coefficient); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "区域系数不存在")
			return
		}
		h.logger.Error("更新区域系数失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "更新区域系数失败")
		return
	}
	OK(c, gin.H{"id": id, "coefficient": body.Coefficient})
}

// DeleteRegionCoefficient 处理 DELETE /api/valuation/admin/region-coefficients/:id
func (h *ConfigHandler) DeleteRegionCoefficient(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}
	if err := h.dictRepo.DeleteRegionCoefficient(c.Request.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "区域系数不存在")
			return
		}
		h.logger.Error("删除区域系数失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "删除区域系数失败")
		return
	}
	OK(c, gin.H{"id": id})
}

// --- original_prices ---

// CreateOriginalPrice 处理 POST /api/valuation/admin/original-prices
// Body: 完整 original_prices 行（不含 id 与 updated_at）
func (h *ConfigHandler) CreateOriginalPrice(c *gin.Context) {
	var body repository.OriginalPrice
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	if body.BrandType == "" || body.Brand == "" || body.VehicleType == "" || body.Series == "" {
		Error(c, http.StatusBadRequest, CodeBadRequest, "brand_type/brand/vehicle_type/series 必填")
		return
	}
	if body.OriginalPrice <= 0 {
		Error(c, http.StatusBadRequest, CodeBadRequest, "original_price 必须大于 0")
		return
	}
	id, err := h.dictRepo.CreateOriginalPrice(c.Request.Context(), &body)
	if err != nil {
		h.logger.Error("新增原价记录失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "新增原价记录失败")
		return
	}
	body.ID = id
	OK(c, body)
}

// UpdateOriginalPrice 处理 PUT /api/valuation/admin/original-prices/:id
// Body: {"original_price":28.50,"is_active":true}
func (h *ConfigHandler) UpdateOriginalPrice(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}
	var body struct {
		OriginalPrice float64 `json:"original_price" binding:"required"`
		IsActive      bool    `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	if err := h.dictRepo.UpdateOriginalPrice(c.Request.Context(), id, body.OriginalPrice, body.IsActive); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "原价记录不存在")
			return
		}
		h.logger.Error("更新原价记录失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "更新原价记录失败")
		return
	}
	OK(c, gin.H{"id": id, "original_price": body.OriginalPrice, "is_active": body.IsActive})
}

// DeleteOriginalPrice 处理 DELETE /api/valuation/admin/original-prices/:id
func (h *ConfigHandler) DeleteOriginalPrice(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}
	if err := h.dictRepo.DeleteOriginalPrice(c.Request.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "原价记录不存在")
			return
		}
		h.logger.Error("删除原价记录失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "删除原价记录失败")
		return
	}
	OK(c, gin.H{"id": id})
}

// --- coefficient_configs ---

// UpdateCoefficient 处理 PUT /api/valuation/admin/coefficient-configs/:key
// Body: {"value":0.15}
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
	cc, err := h.dictRepo.UpdateCoefficientByKey(c.Request.Context(), key, body.Value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(c, http.StatusNotFound, CodeNotFound, "系数 key 不存在: "+key)
			return
		}
		h.logger.Error("更新系数失败", zap.Error(err))
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "更新系数失败")
		return
	}
	OK(c, cc)
}
