package points

import (
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/config"
	"forklift-training/internal/notification"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// newPointsContractRouter 自装管理端扣罚路由（域内契约测试用）。
//
// 与装配根的装配链同义：会话取装配链那份（SessionFromConfigWithBlacklist + **内存黑名单** ——
// NewSession 装的是 Redis 存储，测试链路没有 Redis，见 docs/design/1445-api-contract-test-audit.md §8.6），
// service 按 providers_core.go 的实参构造（clock.Real() + notification 单点）。
// 域包不得 import internal/api，故不经 newContractDeps（#1445 批 4）。
func newPointsContractRouter(t *testing.T, db *gorm.DB, cfg *config.Config) *gin.Engine {
	t.Helper()
	sess := security.SessionFromConfigWithBlacklist(cfg, testutil.NewValueBlacklist())
	svc := NewService(db, zap.NewNop(), clock.Real(), notification.NewService(db, zap.NewNop()))
	r := gin.New()
	RegisterAdminRoutes(r.Group("/api"), sess, svc)
	return r
}
