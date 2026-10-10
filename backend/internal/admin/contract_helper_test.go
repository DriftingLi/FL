package admin

import (
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/auth"
	"forklift-training/internal/config"
	"forklift-training/internal/core"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// newAdminRecruiterContractEnv 装配管理端招聘者面（随域下沉，#1445 批 4）。
//
// 与 internal/api 的 newContractDeps + providers_core.go 同义：会话走内存黑名单
// （测试链路没有 Redis，审计 §8.6），authSvc 用同一份构造实参（providers_core.go:65）；
// 返回的引擎已挂好 /api 分组，调用方只管发请求。
func newAdminRecruiterContractEnv(t *testing.T, db *gorm.DB, cfg *config.Config) *gin.Engine {
	t.Helper()
	if cfg == nil {
		cfg = &config.Config{}
	}
	sess := security.SessionFromConfigWithBlacklist(cfg, testutil.NewValueBlacklist())
	authSvc := auth.NewService(db, sess, core.NewForumCounter(),
		cfg.DefaultPasswords.Admin, cfg.DefaultPasswords.Tutor, cfg.DefaultPasswords.Student, zap.NewNop())
	r := gin.New()
	apiGroup := r.Group("/api")
	// 管理端能力解析源（#1618 段1）：与生产装配根 NewRouter 同一条挂法。自建路由的测试也必须注入 ——
	// 管理端能力改为数据层回答后，缺了它管理端端点一律 403（静态表不再回答 admin）。
	apiGroup.Use(middleware.AdminCapabilityResolver(NewService(db, sess, zap.NewNop())))
	RegisterAdminRecruiterRoutes(apiGroup, sess, authSvc)
	return r
}
