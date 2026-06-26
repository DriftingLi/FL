package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// NewRouter 创建并配置 Gin 引擎，注册全部路由与中间件。
func NewRouter(cfg *config.Config, db *gorm.DB) *gin.Engine {
	if cfg.IsProd() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())
	r.Use(middleware.CORS(cfg.CORSOrigins))

	// 健康检查与根路由（无需鉴权）
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "message": "backend is running"})
	})
	r.GET("/api", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "Forklift Training System API", "version": "1.0.0"})
	})

	// 静态资源：等价 Python Flask 的 static_folder + VOLUME_MOUNT_PATH 行为
	// /static/uploads/* 优先从 VOLUME_MOUNT_PATH/uploads 提供，否则本地 UploadFolder
	// /static/*         其他静态资源从本地 static/ 目录提供
	registerStaticRoutes(r, cfg)

	// 初始化服务
	authSvc := service.NewAuthService(db, cfg.JWTSecretKey, cfg.JWTExpiry())
	authH := NewAuthHandler(authSvc)

	// ===== API 路由组 =====
	api := r.Group("/api")

	// 认证蓝图 /api/auth/*
	auth := api.Group("/auth")
	{
		auth.POST("/login", authH.Login)
		auth.POST("/register", authH.Register)
		auth.POST("/admin-login", authH.AdminLogin)
		auth.POST("/tutor-login", authH.TutorLogin)
		auth.POST("/logout", middleware.JWTAuth(cfg), authH.Logout)
		auth.GET("/me", middleware.JWTAuth(cfg), authH.Me)
	}

	// 注册全部 14 个业务蓝图：
	//   auth/courses/exam/student/practice/practice-mode/question-bank/
	//   level-exam/grading/ai/tutor/wrong-questions/mock-exam/admin
	RegisterCoursesRoutes(api, cfg, db)
	RegisterExamRoutes(api, cfg, db)
	RegisterStudentRoutes(api, cfg, db)
	RegisterPracticeRoutes(api, cfg, db)
	RegisterPracticeModeRoutes(api, cfg, db)
	RegisterQuestionBankRoutes(api, cfg, db)
	RegisterLevelExamRoutes(api, cfg, db)
	RegisterGradingRoutes(api, cfg, db)
	RegisterAIRoutes(api, cfg, db)
	RegisterAdminRoutes(api, cfg, db)
	RegisterTutorRoutes(api, cfg, db)
	RegisterWrongQuestionRoutes(api, cfg, db)
	RegisterMockExamRoutes(api, cfg, db)

	_ = response.Success // 确保包引用
	return r
}

// registerStaticRoutes 注册 /static/* 静态资源路由，等价 Python Flask 行为。
//
// /static/uploads/<path> 优先从 VOLUME_MOUNT_PATH/uploads 提供，否则本地 UploadFolder/static/uploads
// /static/<path>          其他静态资源从本地 static/ 目录提供
func registerStaticRoutes(r *gin.Engine, cfg *config.Config) {
	uploadDir := resolveUploadDir(cfg)
	_ = os.MkdirAll(uploadDir, 0o755)

	// 预计算 static 目录的绝对路径，避免依赖进程工作目录
	staticDir := resolveStaticDir()

	r.GET("/static/*filepath", func(c *gin.Context) {
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
	})
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
	// 2. 可执行文件上级目录下的 static（本地开发 cwd 可能不是 backend-go/）
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)        // backend-go/bin
		projectDir := filepath.Dir(exeDir) // backend-go
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
