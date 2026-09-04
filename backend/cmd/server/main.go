// Package main 是叉车维修培训与残值评估系统的服务入口。
// 单一进程、单一端口（默认 :8080），同时提供：
//   - 维修培训业务路由 /api/*
//   - 残值评估子模块路由 /api/valuation/*
//
// @title 叉车维修培训系统-学员端 API
// @version 1.0
// @description 学员端与公开端点：认证/验证码/微信登录/学员学习中心/课程/练习/考试/收藏/搜索/资料（课程附件 + 学员投稿）/论坛/通知/精选/AI助手/积分/培训目录；鉴权：`Authorization: Bearer <access JWT>`（access 2h，`POST /api/auth/refresh` 换新双令牌）。响应统一 `{code,message,data}`（见 ADR-0005）。不含导师端、管理端与残值评估模块。
// @BasePath /api
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Bearer <access JWT>，示例：Bearer eyJhbGciOi...
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

	"forklift-training/internal/api"
	"forklift-training/internal/cache"
	"forklift-training/internal/config"
	"forklift-training/internal/daemon"
	"forklift-training/internal/db"
	applogger "forklift-training/internal/logger"
	migratedb "forklift-training/internal/migrate"
	"forklift-training/internal/security"
	svc "forklift-training/internal/service"
	"forklift-training/internal/storage"
	vconfig "forklift-training/internal/valuation/config"
	vhandler "forklift-training/internal/valuation/handler"
	"forklift-training/internal/valuation/pdf"
	vrepo "forklift-training/internal/valuation/repository"
	vservice "forklift-training/internal/valuation/service"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
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

	// 4. 创建文件存储实例（local 本地磁盘 / r2 Cloudflare R2 对象存储，装配根依赖）
	st, err := createStorage(cfg)
	if err != nil {
		logger.Error("创建文件存储实例失败", zap.Error(err))
		os.Exit(1)
	}
	logger.Info("文件存储就绪", zap.String("driver", cfg.Storage.Driver))

	// 4.5 估值子模块 pgx 连接池（装配根与 setupValuation 共用同一池）
	vpool, err := createValuationPool(cfg, logger)
	if err != nil {
		logger.Error("valuation pgx 连接池创建失败", zap.Error(err))
		os.Exit(1)
	}
	defer vpool.Close()

	// 5. 装配根：全部 service 在此构建一次（单一装配根，见 spec #75 D9）
	// 导出数据访问经 ExportStore seam 注入估值模块 adapter（spec #75 D4）
	deps := api.NewDeps(cfg, gormDB, st, logger, vrepo.NewExportStore(vpool))

	// 5.1 确保默认账号（密码由环境变量配置）
	if err := deps.AuthSvc.EnsureDefaultUsers(); err != nil {
		logger.Error("默认用户创建失败", zap.Error(err))
		os.Exit(1)
	}
	logger.Info("默认用户就绪")

	// 5.5 清理上次进程遗留的异步生成任务（避免重启后前端一直显示「生成中」）
	deps.ContentGenSvc.CleanupInterruptedTasks()

	// 5.6 检查 AI 配置：未配置任何启用模型时告警（简答题评分等 AI 功能将走导师人工评分）
	if !deps.AIConfigSvc.HasActiveConfigs(context.Background()) {
		logger.Warn("未检测到已启用的 AI 模型配置：简答题评分将走导师人工评分，AI 助手/课程内容生成不可用；请在管理端「AI 设置」中配置模型")
	}

	// 5.7 检查腾讯云短信签名/模板审核状态（已配置时自检，失败仅告警不阻断启动）
	if cfg.SMS.Configured() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if smsCh, ok := deps.PhoneCh.(*svc.SmsChannel); ok {
			if err := smsCh.ValidateReady(ctx); err != nil {
				logger.Warn("腾讯云短信签名/模板自检未通过", zap.Error(err))
			} else {
				logger.Info("腾讯云短信签名/模板自检通过")
			}
		}
		cancel()
	}

	// 6. 确保上传/PDF 目录存在
	ensureUploadDirs(cfg, logger)

	// 6.5 创建路由（维修培训业务 + 静态资源 + 健康检查）
	router := api.NewRouter(deps)

	// 7. 论坛悬空图片定期清理（每 6 小时扫描 images/forum/ 前缀，删除超 24h 未引用的图片）
	daemonCtx, daemonCancel := context.WithCancel(context.Background())
	defer daemonCancel()
	startForumImageCleanup(daemonCtx, deps, logger)
	startContributionCleanup(daemonCtx, deps, logger)

	// 7.5 装配残值评估子模块（注册 /api/valuation/* 路由）
	cleanup := setupValuation(router, cfg, deps.AuthSvc, deps.Session, vpool, st, logger, deps.AuditSvc)
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
	daemonCancel()

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

// createValuationPool 创建估值子模块 pgx 连接池（与 GORM 共用 DATABASE_URL）。
func createValuationPool(cfg *config.Config, logger *zap.Logger) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := vconfig.NewPostgresPool(ctx, cfg.DatabaseURL,
		cfg.Valuation.DBMaxOpenConns, cfg.Valuation.DBMaxIdleConns, cfg.Valuation.DBConnMaxLifetime)
	if err != nil {
		return nil, err
	}
	logger.Info("valuation pgx 连接池就绪")
	return pool, nil
}

// setupValuation 装配残值评估子模块，注册 /api/valuation/* 路由。
// 返回 cleanup 函数用于释放 pgx 连接池（pool 由调用方创建并共用）。
//
//nolint:gocritic
func setupValuation(r *gin.Engine, cfg *config.Config, authSvc vhandler.ValuationAuth, sess *security.Session, pool *pgxpool.Pool, st storage.Storage, logger *zap.Logger, auditSvc *svc.AuditService) func() {
	// 1. 装配数据访问层（手写 pgx 仓储）
	dictRepo := vrepo.NewDictionaryRepository(pool)
	evalRepo := vrepo.NewEvaluationRepository(pool)

	// 2. 装配业务服务（系数从 DB 实时查询，不再使用内存加载器）
	valuationSvc, err := vservice.NewValuationService(dictRepo, evalRepo)
	if err != nil {
		logger.Error("valuation 服务初始化失败", zap.Error(err))
		os.Exit(1)
	}
	batterySvc := vservice.NewBatteryRULService()
	batteryRepo := vrepo.NewBatteryRepository(pool)

	// 3. 装配 PDF 生成器（字节输出，不落盘；存储经 storage 抽象层）
	pdfGen := pdf.NewGenerator()

	// 4. 注册路由（/api/valuation/*，公开组 + 估值独立鉴权组 + admin 组）
	// 认证经 ValuationAuth 窄接口注入主体系 AuthService（spec #75 D4）
	// PDF 报告通过 storage 抽象层上传（local=本地磁盘 / r2=Cloudflare R2 对象存储）
	// Session 单例：与主体系共用同一实例（装配一次，B2 D4；现由 NewDeps 创建）
	vhandler.RegisterRoutes(r, sess, logger, auditSvc, dictRepo, evalRepo, batteryRepo, valuationSvc, batterySvc, pdfGen, st, authSvc)
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
// 由通用守护 runner 托管（panic 恢复 + jitter 错峰 + 可注入 ticker + context 取消贯穿存储）。
func startForumImageCleanup(ctx context.Context, deps *api.Deps, logger *zap.Logger) {
	const interval = 6 * time.Hour
	runner := daemon.NewRunner("forum-image-cleanup", interval, logger, func(runCtx context.Context) {
		cleaned := deps.ForumImageSvc.CleanupOrphans(runCtx)
		if cleaned > 0 {
			logger.Info("论坛悬空图片清理完成", zap.Int("cleaned", cleaned))
		}
	})
	runner.Start(ctx)
	logger.Info("论坛悬空图片清理任务已启动", zap.String("interval", interval.String()))
}

// startContributionCleanup 启动投稿悬空文件清理定时任务：
// 每 6 小时对 contributions/ 前缀 List，与全量引用集做差集，删除超过 24h 未被引用的文件。
// 由通用守护 runner 托管（与论坛图片清理同一模式）。
func startContributionCleanup(ctx context.Context, deps *api.Deps, logger *zap.Logger) {
	const interval = 6 * time.Hour
	runner := daemon.NewRunner("contribution-file-cleanup", interval, logger, func(runCtx context.Context) {
		cleaned := deps.ContributionSvc.CleanupOrphanFiles(runCtx)
		if cleaned > 0 {
			logger.Info("投稿悬空文件清理完成", zap.Int("cleaned", cleaned))
		}
	})
	runner.Start(ctx)
	logger.Info("投稿悬空文件清理任务已启动", zap.String("interval", interval.String()))
}

// createStorage 根据配置创建文件存储实例。
// driver=local 时使用本地磁盘，driver=r2 时使用 Cloudflare R2 对象存储。
func createStorage(cfg *config.Config) (storage.Storage, error) {
	if cfg.Storage.Driver == "r2" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return storage.NewR2Storage(ctx,
			cfg.Storage.R2Endpoint,
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
