// Package handler 提供残值评估子模块的路由注册入口。
// 路由结构：
//   /api/valuation                      启用 JWTAuth 中间件
//     ├── /evaluations                  评估 CRUD（学生端可访问）
//     ├── /evaluations/:id/report       PDF 报告生成与下载
//     ├── /battery/evaluations          电池 RUL 评估（保留不变）
//     ├── /dictionaries/*               字典查询（学生端只读 GET）
//     ├── /admin/*                      管理员 CRUD（要求 JWT role=admin）
//     └── /health                       健康检查
package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"forklift-training/internal/config"
	"forklift-training/internal/middleware"
	vrepo "forklift-training/internal/valuation/repository"
	vservice "forklift-training/internal/valuation/service"
	"forklift-training/pkg/pdf"
)

// RegisterRoutes 注册残值评估模块路由。
// 路由组 /api/valuation 启用 JWTAuth 中间件；admin 子组额外要求 RoleRequired("admin")。
func RegisterRoutes(
	r *gin.Engine,
	cfg *config.Config,
	logger *zap.Logger,
	pool *pgxpool.Pool,
	dictRepo *vrepo.DictionaryRepository,
	evalRepo *vrepo.EvaluationRepository,
	valuationSvc *vservice.ValuationService,
	batterySvc *vservice.BatteryRULService,
	pdfGen *pdf.Generator,
	pdfOutputDir string,
) {
	g := r.Group("/api/valuation")
	g.Use(middleware.JWTAuth(cfg))

	// === 评估接口（学生端可访问） ===
	evalHandler := NewEvaluationHandler(valuationSvc, evalRepo, logger)
	g.POST("/evaluations", evalHandler.Create)
	g.GET("/evaluations", evalHandler.List)
	g.GET("/evaluations/stats", evalHandler.Stats)
	g.GET("/evaluations/:id", evalHandler.Get)

	// === PDF 报告接口（学生端可访问） ===
	reportHandler := NewReportHandler(evalRepo, pdfGen, logger)
	g.POST("/evaluations/:id/report", reportHandler.Generate)
	g.GET("/evaluations/:id/report", reportHandler.Download)

	// === 字典查询接口（学生端只读 GET，无需 admin 权限） ===
	configHandler := NewConfigHandler(dictRepo, logger)
	dict := g.Group("/dictionaries")
	{
		dict.GET("/brands", configHandler.ListBrands)
		dict.GET("/vehicle-types", configHandler.ListVehicleTypes)
		dict.GET("/series", configHandler.ListSeries)
		dict.GET("/tonnages", configHandler.ListTonnages)
		dict.GET("/config-types", configHandler.ListConfigTypes)
		dict.GET("/mast-types", configHandler.ListMastTypes)
		dict.GET("/mast-heights", configHandler.ListMastHeights)
		dict.GET("/battery-types", configHandler.ListBatteryTypes)
		dict.GET("/transmission-types", configHandler.ListTransmissionTypes)
		dict.GET("/engine-types", configHandler.ListEngineTypes)
		dict.GET("/series-config-options", configHandler.ListSeriesConfigOptions)
		dict.GET("/condition-ratings", configHandler.ListConditionRatings)
		dict.GET("/region-coefficients", configHandler.ListRegionCoefficients)
		dict.GET("/provinces", configHandler.ListProvinces)
		dict.GET("/cities", configHandler.ListCities)
		dict.GET("/coefficient-configs", configHandler.ListCoefficientConfigs)
		dict.GET("/original-prices", configHandler.ListOriginalPrices)
		dict.GET("/algorithm-parameters", configHandler.ListAlgorithmParameters)
	}

	// === 管理员 CRUD 接口（要求 JWT role=admin） ===
	admin := g.Group("/admin")
	admin.Use(middleware.RoleRequired("admin"))
	{
		// brands
		admin.POST("/brands", configHandler.CreateBrand)
		admin.PUT("/brands/:id", configHandler.UpdateBrand)
		admin.DELETE("/brands/:id", configHandler.DeleteBrand)

		// vehicle_types
		admin.POST("/vehicle-types", configHandler.CreateVehicleType)
		admin.PUT("/vehicle-types/:id", configHandler.UpdateVehicleType)
		admin.DELETE("/vehicle-types/:id", configHandler.DeleteVehicleType)

		// series
		admin.POST("/series", configHandler.CreateSeries)
		admin.PUT("/series/:id", configHandler.UpdateSeries)
		admin.DELETE("/series/:id", configHandler.DeleteSeries)

		// tonnages
		admin.POST("/tonnages", configHandler.CreateTonnage)
		admin.DELETE("/tonnages/:id", configHandler.DeleteTonnage)

		// mast_types
		admin.POST("/mast-types", configHandler.CreateMastType)
		admin.DELETE("/mast-types/:id", configHandler.DeleteMastType)

		// mast_heights
		admin.POST("/mast-heights", configHandler.CreateMastHeight)
		admin.DELETE("/mast-heights/:id", configHandler.DeleteMastHeight)

		// battery_types
		admin.POST("/battery-types", configHandler.CreateBatteryType)
		admin.DELETE("/battery-types/:id", configHandler.DeleteBatteryType)

		// transmission_types
		admin.POST("/transmission-types", configHandler.CreateTransmissionType)
		admin.DELETE("/transmission-types/:id", configHandler.DeleteTransmissionType)

		// engine_types
		admin.POST("/engine-types", configHandler.CreateEngineType)
		admin.DELETE("/engine-types/:id", configHandler.DeleteEngineType)

		// condition_ratings
		admin.POST("/condition-ratings", configHandler.CreateConditionRating)
		admin.PUT("/condition-ratings/:id", configHandler.UpdateConditionRating)
		admin.DELETE("/condition-ratings/:id", configHandler.DeleteConditionRating)

		// region_coefficients
		admin.POST("/region-coefficients", configHandler.CreateRegionCoefficient)
		admin.PUT("/region-coefficients/:id", configHandler.UpdateRegionCoefficient)
		admin.DELETE("/region-coefficients/:id", configHandler.DeleteRegionCoefficient)

		// original_prices
		admin.POST("/original-prices", configHandler.CreateOriginalPrice)
		admin.PUT("/original-prices/:id", configHandler.UpdateOriginalPrice)
		admin.DELETE("/original-prices/:id", configHandler.DeleteOriginalPrice)

		// coefficient_configs（仅支持按 key 更新值，不允许新增/删除）
		admin.PUT("/coefficient-configs/:key", configHandler.UpdateCoefficient)
	}

	// === 电池 RUL 评估（保留不变） ===
	batteryRepo := vrepo.NewBatteryRepository(pool)
	batteryHandler := NewBatteryHandler(batteryRepo, batterySvc, logger, pdfOutputDir)
	g.POST("/battery/evaluations", batteryHandler.Create)
	g.GET("/battery/evaluations", batteryHandler.List)
	g.GET("/battery/evaluations/:id", batteryHandler.Get)
	g.POST("/battery/evaluations/:id/report", batteryHandler.GenerateReport)
	g.GET("/battery/evaluations/:id/report", batteryHandler.DownloadReport)

	// === 健康检查（valuation 子模块独立健康端点） ===
	healthHandler := NewHealthHandler()
	g.GET("/health", healthHandler.Check)
}
