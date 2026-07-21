// Package handler 提供残值评估子模块的路由注册入口。
// 路由结构：
//
//	/api/valuation                      公开组（无需登录）
//	  ├── POST /evaluations             评估提交（匿名存储）
//	  ├── GET  /evaluations/stats       评估统计
//	  ├── POST /evaluations/:id/report  生成 PDF 报告
//	  ├── GET  /evaluations/:id/report  下载 PDF 报告
//	  ├── POST /battery/evaluations/:id/report   生成电池报告
//	  ├── GET  /battery/evaluations/:id/report   下载电池报告
//	  ├── POST /auth/login              估值模块独立登录
//	  ├── POST /auth/register           估值模块独立注册
//	  ├── /dictionaries/*               字典查询（只读 GET）
//	  └── /health                       健康检查
//
//	/api/valuation                      估值鉴权组（ValuationJWTAuth，独立 JWT secret）
//	  ├── GET  /evaluations             评估历史/详情（需登录）
//	  ├── GET  /evaluations/:id
//	  ├── /battery/evaluations          电池 RUL 评估 CRUD（需登录）
//	  ├── GET  /auth/me                 获取当前估值用户
//	  └── POST /auth/logout             估值用户登出
//
//	/api/valuation/admin                管理员组（主体系 JWTAuth + role=admin）
//	  └── /admin/*                      管理员 CRUD（仍走主体系 admin JWT）
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
// 路由分四组：
//   - 公开组 /api/valuation：字典查询、评估提交、统计、健康检查、报告生成/下载、登录/注册（匿名可访问）
//   - 估值鉴权组 /api/valuation：评估历史/详情、电池 RUL CRUD、/auth/me、/auth/logout（需估值专属 ValuationJWTAuth）
//   - 管理员组 /api/valuation/admin：字典 CRUD（需主体系 admin JWT）
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
	valuationAuthSvc *vservice.ValuationAuthService,
) {
	evalHandler := NewEvaluationHandler(valuationSvc, evalRepo, logger)
	configHandler := NewConfigHandler(dictRepo, logger)
	reportHandler := NewReportHandler(evalRepo, pdfGen, logger)
	batteryRepo := vrepo.NewBatteryRepository(pool)
	batteryHandler := NewBatteryHandler(batteryRepo, batterySvc, logger, pdfOutputDir)
	healthHandler := NewHealthHandler()
	valuationAuthHandler := NewValuationAuthHandler(valuationAuthSvc)

	// === 公开组（无需登录）：字典查询 + 评估提交 + 统计 + 健康检查 + 报告生成/下载 + 登录/注册 ===
	// 未登录用户可提交评估并被计数（evaluations 表无 user_id，记录匿名存储）
	// 报告生成/下载也改为公开：未登录用户可下载已生成的评估报告
	public := r.Group("/api/valuation")
	{
		public.POST("/evaluations", evalHandler.Create)
		public.GET("/evaluations/stats", evalHandler.Stats)
		public.GET("/health", healthHandler.Check)

		// 估值模块独立登录/注册（公开接口）
		public.POST("/auth/login", valuationAuthHandler.Login)
		public.POST("/auth/register", valuationAuthHandler.Register)

		// 报告生成与下载（无需登录）
		public.POST("/evaluations/:id/report", reportHandler.Generate)
		public.GET("/evaluations/:id/report", reportHandler.Download)
		public.POST("/battery/evaluations/:id/report", batteryHandler.GenerateReport)
		public.GET("/battery/evaluations/:id/report", batteryHandler.DownloadReport)

		dict := public.Group("/dictionaries")
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
			dict.GET("/earliest-factory-year", configHandler.GetEarliestFactoryYear)
			dict.GET("/algorithm-parameters", configHandler.ListAlgorithmParameters)
		}
	}

	// === 估值独立鉴权组（需估值专属 ValuationJWTAuth） ===
	// 评估历史/详情 + 电池 RUL CRUD + /auth/me + /auth/logout
	// 使用独立 JWT secret，与主体系 token 互不兼容
	valAuth := r.Group("/api/valuation")
	valAuth.Use(ValuationJWTAuth(cfg.Valuation.JWTSecretKey))
	{
		valAuth.GET("/evaluations", evalHandler.List)
		valAuth.GET("/evaluations/:id", evalHandler.Get)

		valAuth.POST("/battery/evaluations", batteryHandler.Create)
		valAuth.GET("/battery/evaluations", batteryHandler.List)
		valAuth.GET("/battery/evaluations/:id", batteryHandler.Get)

		valAuth.GET("/auth/me", valuationAuthHandler.Me)
		valAuth.POST("/auth/logout", valuationAuthHandler.Logout)
	}

	// === 管理员 CRUD 接口（要求主体系 JWT role=admin） ===
	// 残值配置管理仍走主体系 admin JWT，不参与此次独立化
	admin := r.Group("/api/valuation/admin")
	admin.Use(middleware.JWTAuth(cfg))
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
}
