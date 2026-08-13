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
//	/api/valuation                      可选认证组（登录与否都能用，登录则记录 user_id）
//	  ├── POST /evaluations             评估提交
//	  └── POST /battery/evaluations     电池 RUL 评估提交
//
//	/api/valuation                      估值鉴权组（统一主体系 JWT）
//	  ├── GET  /evaluations             评估历史/详情（需登录）
//	  ├── GET  /evaluations/:id
//	  ├── /battery/evaluations          电池 RUL 评估历史（需登录）
//	  ├── GET  /auth/me                 获取当前估值用户
//	  └── POST /auth/logout             估值用户登出
//
//	/api/valuation/admin                管理员组（主体系 JWTAuth + role=admin）
//	  └── /admin/*                      管理员 CRUD（仍走主体系 admin JWT）
package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/internal/storage"
	"forklift-training/internal/valuation/dictcrud"
	vservice "forklift-training/internal/valuation/service"
)

// RegisterRoutes 注册残值评估模块路由。
// 路由分五组：
//   - 公开组 /api/valuation：字典查询、统计、健康检查、报告生成/下载、登录/注册（匿名可访问）
//   - 可选认证组 /api/valuation：评估提交、电池评估提交（匿名可提交，登录则记录 user_id）
//   - 估值鉴权组 /api/valuation：评估历史/详情、电池历史、/auth/me、/auth/logout（需登录）
//   - 管理员组 /api/valuation/admin：字典 CRUD（需主体系 admin JWT）
func RegisterRoutes(
	r *gin.Engine,
	sess *security.Session,
	logger *zap.Logger,
	auditCfg *config.Config,
	auditDB *gorm.DB,
	dictRepo DictionaryConfigStore,
	evalRepo EvaluationStore,
	batteryRepo BatteryStore,
	valuationSvc *vservice.ValuationService,
	batterySvc *vservice.BatteryRULService,
	pdfGen ReportGenerator,
	st storage.Storage,
	valuationAuthSvc ValuationAuth,
) {
	evalHandler := NewEvaluationHandler(valuationSvc, evalRepo, logger)
	configHandler := NewConfigHandler(dictRepo, logger)
	reportHandler := NewReportHandler(evalRepo, pdfGen, logger, st, vservice.NewCoefficientProvider(dictRepo))
	batteryHandler := NewBatteryHandler(batteryRepo, batterySvc, logger, st)
	healthHandler := NewHealthHandler()
	valuationAuthHandler := NewValuationAuthHandler(valuationAuthSvc, sess)

	// === 公开组（无需登录）：字典查询 + 统计 + 健康检查 + 报告生成/下载 + 登录/注册 ===
	// 评估提交（POST /evaluations）已移至"可选认证组"，登录用户提交时记录 user_id
	// 报告生成/下载公开：未登录用户可下载已生成的评估报告
	public := r.Group("/api/valuation")
	{
		public.GET("/evaluations/stats", evalHandler.Stats)
		public.GET("/health", healthHandler.Check)

		// 估值模块独立登录/注册（公开接口）
		public.POST("/auth/login", valuationAuthHandler.Login)

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

	// === 可选认证组（登录与否都能用，登录则记录 user_id） ===
	// 评估提交/电池评估：未登录可提交（user_id 落 NULL），登录用户提交则归属到自己
	optional := r.Group("/api/valuation")
	optional.Use(middleware.OptionalAuth(sess))
	{
		optional.POST("/evaluations", evalHandler.Create)
		optional.POST("/battery/evaluations", batteryHandler.Create)
	}

	// === HRWAI 账号鉴权组（需 middleware.JWTAuth + role=hrwai_user） ===
	// 评估历史/详情 + 电池 RUL CRUD + /auth/me + /auth/logout
	// 已统一到主体系 JWT,与培训学员端共用同一 token
	valAuth := r.Group("/api/valuation")
	valAuth.Use(middleware.JWTAuth(sess), middleware.RoleRequired("hrwai_user"))
	{
		valAuth.GET("/evaluations", evalHandler.List)
		valAuth.GET("/evaluations/:id", evalHandler.Get)

		valAuth.GET("/battery/evaluations", batteryHandler.List)
		valAuth.GET("/battery/evaluations/:id", batteryHandler.Get)

		valAuth.GET("/auth/me", valuationAuthHandler.Me)
		valAuth.POST("/auth/logout", valuationAuthHandler.Logout)
	}

	// === 管理员 CRUD 接口（要求主体系 JWT role=admin） ===
	// 残值配置管理仍走主体系 admin JWT，不参与此次独立化
	// 全部字典写面由描述符注册表驱动（ADR-0008）：POST/PUT/DELETE 按描述符声明注册，
	// 不再逐实体手写路由。失效 pattern 来自 repository 缓存契约单点（PatternsOf）。
	admin := r.Group("/api/valuation/admin")
	admin.Use(middleware.JWTAuth(sess))
	admin.Use(middleware.RoleRequired("admin"))
	// 管理员写操作审计：与主体系同一留痕口径（合规用途，ADR-0012 §7）
	admin.Use(middleware.AuditLog(auditCfg, auditDB, logger))
	{
		configHandler.registerDictCRUDRoutes(admin, dictcrud.NewRegistry(dictcrud.AllDescriptors()...))
	}
}
