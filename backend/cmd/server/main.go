// Package main 是叉车维修培训与残值评估系统的服务入口。
// 单一进程、单一端口（默认 :8080），同时提供：
//   - 维修培训业务路由 /api/*
//   - 残值评估子模块路由 /api/valuation/*
package main

//nolint:gocritic // exitAfterDefer: os.Exit 在 defer cancel 之前，是预期的启动失败流程

import (
	"context"
	"fmt"
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
	applogger "forklift-training/internal/logger"
	migratedb "forklift-training/internal/migrate"
	"forklift-training/internal/service"
	"forklift-training/internal/storage"
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
		fmt.Fprintln(os.Stderr, "加载配置失败:", err)
		os.Exit(1)
	}

	// 1.5 初始化统一 zap 日志栈（启动最早阶段，后续全部日志走 logger）
	logger, err := applogger.New(applogger.Config{
		Level:      cfg.Log.Level,
		Format:     cfg.Log.Format,
		OutputDir:  cfg.Log.OutputDir,
		MaxSizeMB:  cfg.Log.MaxSizeMB,
		MaxBackups: cfg.Log.MaxBackups,
		MaxAgeDays: cfg.Log.MaxAgeDays,
		Compress:   cfg.Log.Compress,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "日志初始化失败:", err)
		os.Exit(1)
	}
	defer func() { _ = logger.Sync() }()
	logger.Info("配置加载完成", zap.String("env", cfg.AppEnv), zap.String("port", cfg.Port))
	for _, w := range cfg.CORSConfigWarnings() {
		logger.Warn(w)
	}

	// 2. GORM 连接数据库
	gormDB, err := db.InitDB(cfg.DatabaseURL, logger)
	if err != nil {
		logger.Error("数据库连接失败", zap.Error(err))
		os.Exit(1)
	}

	// 2.5. 连接 Redis 缓存
	redisClient, err := cache.InitRedis(cfg.Redis, logger)
	if err != nil {
		logger.Error("Redis 连接失败", zap.Error(err))
		os.Exit(1)
	}

	// 3. 执行数据库迁移
	if err := migratedb.RunMigrations(cfg.DatabaseURL, "up", logger); err != nil {
		logger.Error("数据库迁移失败", zap.Error(err))
		os.Exit(1)
	}

	// 4. 确保默认账号（密码由环境变量配置）
	authSvc := service.NewAuthService(gormDB, cfg.JWTSecretKey, cfg.JWTExpiry(),
		cfg.DefaultPasswords.Admin, cfg.DefaultPasswords.Tutor, cfg.DefaultPasswords.Student, logger)
	if err := authSvc.EnsureDefaultUsers(); err != nil {
		logger.Error("默认用户创建失败", zap.Error(err))
		os.Exit(1)
	}
	logger.Info("默认用户就绪")

	// 4.5 清理上次进程遗留的异步生成任务（避免重启后前端一直显示「生成中」）
	genSvc := service.NewContentGenerateService(gormDB, nil, logger)
	genSvc.CleanupInterruptedTasks()

	// 4.6 检查 AI 配置：未配置任何启用模型时告警（简答题评分等 AI 功能将走导师人工评分）
	aiConfigSvc := service.NewAIConfigService(gormDB, cfg.SecretKey, logger)
	if !aiConfigSvc.HasActiveConfigs(context.Background()) {
		logger.Warn("未检测到已启用的 AI 模型配置：简答题评分将走导师人工评分，AI 助手/课程内容生成不可用；请在管理端「AI 设置」中配置模型")
	}

	// 5. 确保上传/PDF 目录存在
	ensureUploadDirs(cfg, logger)

	// 5.5 创建文件存储实例（local 本地磁盘 / r2 Cloudflare R2 对象存储）
	st, err := createStorage(cfg)
	if err != nil {
		logger.Error("创建文件存储实例失败", zap.Error(err))
		os.Exit(1)
	}
	logger.Info("文件存储就绪", zap.String("driver", cfg.Storage.Driver))

	// 6. 创建路由（维修培训业务 + 静态资源 + 健康检查）
	router := api.NewRouter(cfg, gormDB, st, logger)

	// 6.5 论坛悬空图片定期清理（每 6 小时扫描 images/forum/ 前缀，删除超 24h 未引用的图片）
	startForumImageCleanup(gormDB, cfg, st, logger)

	// 7. 装配残值评估子模块（注册 /api/valuation/* 路由）
	cleanup := setupValuation(router, cfg, gormDB, authSvc, st, logger)
	defer cleanup()

	// 8. 启动 HTTP 服务
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	go func() {
		logger.Info("HTTP 服务启动", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP 服务异常", zap.Error(err))
			os.Exit(1)
		}
	}()

	// 9. 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("正在关闭服务...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("服务关闭异常", zap.Error(err))
	}
	// 释放 GORM 连接池（此前仅关闭了 valuation 的 pgx 池，GORM 池会被泄漏）
	db.Close(gormDB, logger)
	// 释放 Redis 连接池
	cache.CloseRedis(redisClient, logger)
	logger.Info("服务已退出")
}

// setupValuation 装配残值评估子模块，注册 /api/valuation/* 路由。
// 返回 cleanup 函数用于释放 pgx 连接池。
//
//nolint:gocritic
func setupValuation(r *gin.Engine, cfg *config.Config, gormDB *gorm.DB, authSvc *service.AuthService, st storage.Storage, logger *zap.Logger) func() {
	// 1. 创建 pgx 连接池（与 GORM 共用 DATABASE_URL）
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := vconfig.NewPostgresPool(ctx, cfg.DatabaseURL,
		cfg.Valuation.DBMaxOpenConns, cfg.Valuation.DBMaxIdleConns, cfg.Valuation.DBConnMaxLifetime)
	if err != nil {
		logger.Error("valuation pgx 连接池创建失败", zap.Error(err))
		os.Exit(1)
	}
	logger.Info("valuation pgx 连接池就绪")

	// 2. 装配数据访问层（手写 pgx 仓储）
	dictRepo := vrepo.NewDictionaryRepository(pool)
	evalRepo := vrepo.NewEvaluationRepository(pool)

	// 3. 装配业务服务（系数从 DB 实时查询，不再使用内存加载器）
	valuationSvc, err := vservice.NewValuationService(dictRepo, evalRepo)
	if err != nil {
		logger.Error("valuation 服务初始化失败", zap.Error(err))
		os.Exit(1)
	}
	batterySvc := vservice.NewBatteryRULService()
	batteryRepo := vrepo.NewBatteryRepository(pool)

	// 4. 装配估值模块认证服务(已统一到主体系 AuthService,薄包装保留旧签名)
	// 内部代理到主体系 AuthService,使用统一 JWT_SECRET_KEY 与 hrwai_users 表
	valuationAuthSvc := vservice.WrapValuationAuthService(authSvc)

	// 5. 装配 PDF 生成器（outputDir 仅用于本地缓存/兼容旧路径，R2 模式下不写入）
	pdfDir := cfg.Valuation.PDFOutputDir
	if pdfDir == "" {
		pdfDir = "storage/reports"
	}
	if err := os.MkdirAll(pdfDir, 0o755); err != nil {
		logger.Warn("创建 PDF 输出目录失败", zap.Error(err), zap.String("dir", pdfDir))
	}
	pdfGen := pdf.NewGenerator(pdfDir)

	// 6. 注册路由（/api/valuation/*，公开组 + 估值独立鉴权组 + admin 组）
	// PDF 报告通过 storage 抽象层上传（local=本地磁盘 / r2=Cloudflare R2 对象存储）
	vhandler.RegisterRoutes(r, cfg, logger, dictRepo, evalRepo, batteryRepo, valuationSvc, batterySvc, pdfGen, st, valuationAuthSvc)
	logger.Info("valuation 路由注册完成", zap.String("prefix", "/api/valuation"))

	return func() {
		pool.Close()
	}
}

// ensureUploadDirs 确保上传与静态资源目录存在。
func ensureUploadDirs(cfg *config.Config, logger *zap.Logger) {
	dirs := []string{
		cfg.UploadFolder,
		"static/uploads/chapters",
		"static/uploads/slides",
		"static/uploads/images",
		cfg.Valuation.PDFOutputDir,
	}
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if err := os.MkdirAll(d, 0o755); err != nil {
			logger.Warn("创建目录失败", zap.String("dir", d), zap.Error(err))
		}
	}
}

// startForumImageCleanup 启动论坛悬空图片清理定时任务：
// 每 6 小时对 images/forum/ 前缀 List，与全量引用集做差集，删除超过 24h 未被引用的图片。
// 进程内 goroutine（与限流池清理同模式），进程退出自然终止。
func startForumImageCleanup(gormDB *gorm.DB, cfg *config.Config, st storage.Storage, logger *zap.Logger) {
	fileSvc := service.NewFileService(cfg.LibreOfficeSidecarURL, st, logger)
	forumSvc := service.NewForumService(gormDB, fileSvc, logger)
	const interval = 6 * time.Hour
	go func() {
		for range time.Tick(interval) {
			cleaned := forumSvc.CleanupOrphanImages()
			if cleaned > 0 {
				logger.Info("论坛悬空图片清理完成", zap.Int("cleaned", cleaned))
			}
		}
	}()
	logger.Info("论坛悬空图片清理任务已启动", zap.String("interval", interval.String()))
}

// createStorage 根据配置创建文件存储实例。
// driver=local 时使用本地磁盘，driver=r2 时使用 Cloudflare R2 对象存储。
func createStorage(cfg *config.Config) (storage.Storage, error) {
	if cfg.Storage.Driver == "r2" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return storage.NewR2Storage(ctx,
			cfg.Storage.R2AccountID,
			cfg.Storage.R2AccessKeyID,
			cfg.Storage.R2SecretAccessKey,
			cfg.Storage.R2Bucket,
			cfg.Storage.R2PublicDomain,
		)
	}
	// 默认本地磁盘
	return storage.NewLocalStorage(cfg.UploadFolder), nil
}
