package favorite

import (
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
	"forklift-training/internal/training"
)

// newFavoriteContractRouter 自装收藏路由（域内契约测试用）。
//
// 与装配根的装配链同义：会话走**内存黑名单**（§8.6）；CredentialScope 用 training.Service
// （装配根 RouterDeps 的 CredentialScope 就是它 —— 它只实现 CurrentCredentialID 一个方法）；
// service 按 providers_training.go 的实参构造。域包不得 import internal/api，故不经 newContractDeps。
func newFavoriteContractRouter(t *testing.T, db *gorm.DB, cfg *config.Config) *gin.Engine {
	t.Helper()
	sess := security.SessionFromConfigWithBlacklist(cfg, testutil.NewValueBlacklist())
	credRes := training.NewService(db, zap.NewNop())
	svc := NewService(db, zap.NewNop())
	r := gin.New()
	RegisterRoutes(r.Group("/api"), sess, credRes, svc)
	return r
}
