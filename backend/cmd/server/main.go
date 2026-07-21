// Package main 是叉车维修培训与残值评估系统的服务入口。
// 单一进程、单一端口（默认 :8080），同时提供：
//   - 维修培训业务路由 /api/*
//   - 残值评估子模块路由 /api/valuation/*
package main

//nolint:gocritic // exitAfterDefer: os.Exit 在 defer cancel 之前，是预期的启动失败流程

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/api"
	"forklift-training/internal/cache"
	"forklift-training/internal/config"
	"forklift-training/internal/db"
	migratedb "forklift-training/internal/migrate"
	"forklift-training/internal/service"
	vconfig "forklift-training/internal/valuation/config"
	vhandler "forklift-training/internal/valuation/handler"
	vrepo "forklift-training/internal/valuation/repository"
	vservice "forklift-training/internal/valuation/service"
	"forklift-training/pkg/pdf"
)

//nolint:gocritic
func main() {
	// 开发环境自动加载 .env
	_ = godotenv.Load()

	// 1. 加载配置
	cfg, err := config.Load()
	if err != nil {
		slog.Error("加载配置失败", "error", err)
		os.Exit(1)
	}
	slog.Info("配置加载完成", "env", cfg.AppEnv, "port", cfg.Port)

	// 2. GORM 连接数据库
	gormDB, err := db.InitDB(cfg.DatabaseURL)
	if err != nil {
		slog.Error("数据库连接失败", "error", err)
		os.Exit(1)
	}
	slog.Info("数据库连接成功")

	// 2.5. 连接 Redis 缓存
	redisClient, err := cache.InitRedis(cfg.Redis)
	if err != nil {
		slog.Error("Redis 连接失败", "error", err)
		os.Exit(1)
	}

	// 3. 执行数据库迁移（000001~000017，覆盖维修培训初始化与残值评估全量表结构/种子/系数调整）
	if err := migratedb.RunMigrations(cfg.DatabaseURL, "up"); err != nil {
		slog.Error("数据库迁移失败", "error", err)
		os.Exit(1)
	}
	slog.Info("数据库迁移完成")

	// 4. 确保默认账号（密码由环境变量配置）
	authSvc := service.NewAuthService(gormDB, cfg.JWTSecretKey, cfg.JWTExpiry(),
		cfg.DefaultPasswords.Admin, cfg.DefaultPasswords.Tutor, cfg.DefaultPasswords.Student)
	if err := authSvc.EnsureDefaultUsers(); err != nil {
		slog.Error("默认用户创建失败", "error", err)
		os.Exit(1)
	}
	slog.Info("默认用户就绪")

	// 5. 确保上传/PDF 目录存在
	ensureUploadDirs(cfg)

	// 6. 创建路由（维修培训业务 + 静态资源 + 健康检查）
	router := api.NewRouter(cfg, gormDB)

	// 7. 装配残值评估子模块（注册 /api/valuation/* 路由）
	cleanup := setupValuation(router, cfg, gormDB)
	defer cleanup()

	// 8. 启动 HTTP 服务
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	go func() {
		slog.Info("HTTP 服务启动", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP 服务异常", "error", err)
			os.Exit(1)
		}
	}()

	// 9. 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("正在关闭服务...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("服务关闭异常", "error", err)
	}
	// 释放 GORM 连接池（此前仅关闭了 valuation 的 pgx 池，GORM 池会被泄漏）
	db.Close(gormDB)
	// 释放 Redis 连接池
	cache.CloseRedis(redisClient)
	slog.Info("服务已退出")
}

// setupValuation 装配残值评估子模块，注册 /api/valuation/* 路由。
// 返回 cleanup 函数用于释放 pgx 连接池和 zap 日志缓冲。
//
//nolint:gocritic
func setupValuation(r *gin.Engine, cfg *config.Config, gormDB *gorm.DB) func() {
	// 1. 初始化 zap 日志器
	vLogger, err := vconfig.NewLogger(vconfig.LogConfig{
		Level:  cfg.Valuation.LogLevel,
		Format: cfg.Valuation.LogFormat,
		Output: cfg.Valuation.LogOutput,
	})
	if err != nil {
		slog.Warn("valuation 日志初始化失败，降级到无操作日志", "error", err)
		vLogger = zap.NewNop()
	}

	// 2. 创建 pgx 连接池（与 GORM 共用 DATABASE_URL）
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := vconfig.NewPostgresPool(ctx, cfg.DatabaseURL,
		cfg.Valuation.DBMaxOpenConns, cfg.Valuation.DBMaxIdleConns, cfg.Valuation.DBConnMaxLifetime)
	if err != nil {
		slog.Error("valuation pgx 连接池创建失败", "error", err)
		os.Exit(1)
	}
	slog.Info("valuation pgx 连接池就绪")

	// 3. 装配数据访问层（手写 pgx 仓储）
	dictRepo := vrepo.NewDictionaryRepository(pool)
	evalRepo := vrepo.NewEvaluationRepository(pool)

	// 4. 装配业务服务（系数从 DB 实时查询，不再使用内存加载器）
	valuationSvc, err := vservice.NewValuationService(pool, dictRepo, evalRepo)
	if err != nil {
		slog.Error("valuation 服务初始化失败", "error", err)
		os.Exit(1)
	}
	batterySvc := vservice.NewBatteryRULService()

	// 5. 装配估值模块独立认证服务（独立 JWT secret + 独立用户表）
	valuationAuthSvc := vservice.NewValuationAuthService(gormDB, cfg.Valuation.JWTSecretKey, cfg.ValuationJWTExpiry())

	// 6. 装配 PDF 生成器
	pdfDir := cfg.Valuation.PDFOutputDir
	if pdfDir == "" {
		pdfDir = "storage/reports"
	}
	if err := os.MkdirAll(pdfDir, 0o755); err != nil {
		vLogger.Warn("创建 PDF 输出目录失败", zap.Error(err), zap.String("dir", pdfDir))
	}
	pdfGen := pdf.NewGenerator(pdfDir)

	// 7. 注册路由（/api/valuation/*，公开组 + 估值独立鉴权组 + admin 组）
	vhandler.RegisterRoutes(r, cfg, vLogger, pool, dictRepo, evalRepo, valuationSvc, batterySvc, pdfGen, pdfDir, valuationAuthSvc)
	slog.Info("valuation 路由注册完成", "prefix", "/api/valuation")

	return func() {
		pool.Close()
		_ = vLogger.Sync()
	}
}

// ensureUploadDirs 确保上传与静态资源目录存在。
func ensureUploadDirs(cfg *config.Config) {
	dirs := []string{
		cfg.UploadFolder,
		"static/uploads/chapters",
		"static/uploads/slides",
		cfg.Valuation.PDFOutputDir,
	}
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if err := os.MkdirAll(d, 0o755); err != nil {
			slog.Warn("创建目录失败", "dir", d, "error", err)
		}
	}
}
