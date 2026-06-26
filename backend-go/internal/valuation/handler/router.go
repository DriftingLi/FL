// Package handler 提供残值评估子模块的路由注册入口。
// 所有路由挂在 /api/valuation/* 下，启用 JWTAuth 中间件。
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
// 路由组 /api/valuation 启用 JWTAuth 中间件。
func RegisterRoutes(
	r *gin.Engine,
	cfg *config.Config,
	logger *zap.Logger,
	pool *pgxpool.Pool,
	queries *vrepo.Queries,
	valuationSvc *vservice.ValuationService,
	brandLoader *vservice.BrandLoader,
	batterySvc *vservice.BatteryRULService,
	pdfGen *pdf.Generator,
	pdfOutputDir string,
) {
	g := r.Group("/api/valuation")
	g.Use(middleware.JWTAuth(cfg))

	// 评估接口
	evalHandler := NewEvaluationHandler(queries, valuationSvc, logger)
	g.POST("/evaluations", evalHandler.Create)
	g.GET("/evaluations", evalHandler.List)
	g.GET("/evaluations/:id", evalHandler.Get)

	// PDF 报告接口
	reportHandler := NewReportHandler(queries, pdfGen, logger)
	g.POST("/evaluations/:id/report", reportHandler.Generate)
	g.GET("/evaluations/:id/report", reportHandler.Download)

	// 配置类接口
	configHandler := NewConfigHandler(queries, brandLoader, logger)
	g.GET("/part-configs", configHandler.ListPartConfigs)
	g.GET("/brands", configHandler.ListBrands)
	g.GET("/coefficients", configHandler.ListCoefficients)
	g.PUT("/coefficients/:key", configHandler.UpdateCoefficient)

	// 历史成交数据导入
	histHandler := NewHistoricalHandler(queries, logger)
	g.POST("/historical-sales/import", histHandler.Import)

	// 电池 RUL 评估
	batteryRepo := vrepo.NewBatteryRepository(pool)
	batteryHandler := NewBatteryHandler(batteryRepo, batterySvc, logger, pdfOutputDir)
	g.POST("/battery/evaluations", batteryHandler.Create)
	g.GET("/battery/evaluations", batteryHandler.List)
	g.GET("/battery/evaluations/:id", batteryHandler.Get)
	g.POST("/battery/evaluations/:id/report", batteryHandler.GenerateReport)
	g.GET("/battery/evaluations/:id/report", batteryHandler.DownloadReport)

	// 健康检查（valuation 子模块独立健康端点）
	healthHandler := NewHealthHandler()
	g.GET("/health", healthHandler.Check)
}
