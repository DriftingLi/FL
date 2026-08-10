package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/cache"
	"forklift-training/internal/config"
	applogger "forklift-training/internal/logger"
	"forklift-training/internal/middleware"
)

// NewRouter 创建并配置 Gin 引擎，注册全部路由与中间件。
// service 实例统一由 NewDeps 构建（装配根），本函数只做路由装配。
func NewRouter(deps *Deps) *gin.Engine {
	cfg := deps.Cfg
	if cfg.IsProd() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(applogger.AccessLog(deps.Logger))
	r.Use(middleware.Recovery(deps.Logger))
	r.Use(middleware.CORS(cfg.CORSOrigins, cfg.IsProd()))
	// 限流：基于客户端 IP 的 token bucket，防暴力枚举/撞库/爬虫
	// 健康检查 /api/health 在中间件内放行，不受限流影响
	r.Use(middleware.RateLimit(cfg, deps.Logger))

	// 健康检查与根路由（无需鉴权）
	// 探测 Redis 连通性，异常时返回 503 便于容器编排重启
	r.GET("/api/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := cache.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "degraded",
				"redis":  "unreachable",
				"error":  err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "backend is running"})
	})
	// 存活探针（liveness）：仅表示进程存活，不依赖外部组件。
	// 容器编排探活应使用本端点，避免 Redis 抖动导致容器被重启。
	r.GET("/api/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/api", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "Forklift Training System API", "version": "1.0.0"})
	})

	// 静态资源：等价 static_folder + VOLUME_MOUNT_PATH 行为
	// /static/uploads/* 优先从 VOLUME_MOUNT_PATH/uploads 提供，否则本地 UploadFolder
	// /static/*         其他静态资源从本地 static/ 目录提供
	registerStaticRoutes(r, cfg)

	authH := deps.AuthH

	// ===== API 路由组 =====
	api := r.Group("/api")
	// 审计日志：记录管理员/讲师写操作（不依赖中间件顺序，见 middleware.AuditLog）
	api.Use(middleware.AuditLog(cfg, deps.DB, deps.Logger))

	// 认证蓝图 /api/auth/*
	auth := api.Group("/auth")
	{
		auth.POST("/login", authH.Login)
		auth.POST("/register", authH.Register)
		auth.POST("/admin-login", authH.AdminLogin)
		auth.POST("/tutor-login", authH.TutorLogin)
		auth.POST("/logout", middleware.JWTAuth(deps.Session), authH.Logout)
		auth.GET("/me", middleware.JWTAuth(deps.Session), authH.Me)
		// 个人资料：昵称 / 头像
		auth.PUT("/profile", middleware.JWTAuth(deps.Session), authH.UpdateProfile)
		auth.POST("/avatar", middleware.JWTAuth(deps.Session), authH.UploadAvatar)
	}

	// 邮箱验证码注册/登录
	RegisterEmailAuthRoutes(api, deps)
	// 手机号验证码注册/登录
	RegisterPhoneAuthRoutes(api, deps)
	// 微信扫码登录（框架占位）
	RegisterWechatAuthRoutes(api, deps)
	// 个人信息页：手机号/邮箱绑定修改
	RegisterProfileBindRoutes(api, deps)

	// 注册全部 12 个业务蓝图：
	//   auth/courses/student/question-bank/
	//   level-exam/grading/tutor/wrong-questions/mock-exam/admin
	//   practice-mode（题库练习模式：自由刷题/知识点专项，对应 question_practice_record）
	RegisterCoursesRoutes(api, deps)
	RegisterStudentRoutes(api, deps)
	RegisterQuestionBankRoutes(api, deps)
	RegisterPracticeModeRoutes(api, deps)
	RegisterLevelExamRoutes(api, deps)
	RegisterGradingRoutes(api, deps)
	RegisterAdminRoutes(api, deps)
	RegisterTutorRoutes(api, deps)
	RegisterWrongQuestionRoutes(api, deps)
	RegisterMockExamRoutes(api, deps)
	RegisterFeaturedRoutes(api, deps)
	RegisterAIAssistantRoutes(api, deps)
	RegisterForumRoutes(api, deps)
	RegisterProfileReviewRoutes(api, deps)
	RegisterNotificationRoutes(api, deps)
	RegisterAuditRoutes(api, deps)
	RegisterExportRoutes(api, deps)
	RegisterTrainingCatalogRoutes(api, deps)

	return r
}

// registerStaticRoutes 注册 /static/* 静态资源路由。
//
// /static/uploads/<path> 优先从 VOLUME_MOUNT_PATH/uploads 提供，否则本地 UploadFolder/static/uploads
// /static/<path>          其他静态资源从本地 static/ 目录提供
func registerStaticRoutes(r *gin.Engine, cfg *config.Config) {
	uploadDir := resolveUploadDir(cfg)
	_ = os.MkdirAll(uploadDir, 0o755)

	// 预计算 static 目录的绝对路径，避免依赖进程工作目录
	staticDir := resolveStaticDir()

	// 静态文件 handler：同时支持 GET 和 HEAD（前端 DocumentViewer/ImageViewer 用 HEAD 检查文件存在性）
	staticHandler := func(c *gin.Context) {
		reqPath := c.Param("filepath") // 含前导 /

		// 防止路径穿越攻击
		if strings.Contains(reqPath, "..") {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		var fullPath string
		if strings.HasPrefix(reqPath, "/uploads/") || reqPath == "/uploads" {
			// 上传文件：从 uploadDir 提供
			rel := strings.TrimPrefix(reqPath, "/uploads")
			fullPath = filepath.Join(uploadDir, rel)
		} else {
			// 其他静态资源：从本地 static/ 目录提供
			fullPath = filepath.Join(staticDir, reqPath)
		}

		// 校验文件存在且不是目录
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.File(fullPath)
	}
	r.GET("/static/*filepath", staticHandler)
	r.HEAD("/static/*filepath", staticHandler)
}

// resolveStaticDir 解析静态资源目录（返回绝对路径）。
// 依次尝试：工作目录下的 static → 可执行文件上级目录下的 static。
func resolveStaticDir() string {
	// 1. 工作目录下的 static（生产环境通常 cwd 正确）
	if abs, err := filepath.Abs("static"); err == nil {
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			return abs
		}
	}
	// 2. 可执行文件上级目录下的 static（本地开发 cwd 可能不是 backend/）
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)        // backend/bin
		projectDir := filepath.Dir(exeDir) // backend
		candidate := filepath.Join(projectDir, "static")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	// 3. 回退
	abs, _ := filepath.Abs("static")
	return abs
}

// resolveUploadDir 解析静态上传目录（返回绝对路径）。
func resolveUploadDir(cfg *config.Config) string {
	if cfg.VolumeMountPath != "" {
		if info, err := os.Stat(cfg.VolumeMountPath); err == nil && info.IsDir() {
			return filepath.Join(cfg.VolumeMountPath, "uploads")
		}
	}
	baseDir := resolveStaticDir()
	if cfg.UploadFolder != "" {
		abs, err := filepath.Abs(cfg.UploadFolder)
		if err == nil {
			return abs
		}
		return cfg.UploadFolder
	}
	return filepath.Join(baseDir, "uploads")
}
