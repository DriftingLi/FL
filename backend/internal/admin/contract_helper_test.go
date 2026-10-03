package admin

import (
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/auth"
	"forklift-training/internal/config"
	"forklift-training/internal/core"
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
	RegisterAdminRecruiterRoutes(r.Group("/api"), sess, authSvc)
	return r
}
