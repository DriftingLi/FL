package forum

import (
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/admincap"
	"forklift-training/internal/config"
	"forklift-training/internal/filestore"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// newForumContractEnv 是论坛域契约测试自装的 HTTP 面：在域内既有的 newForumTestEnv
// （service_test.go，注释写明「与 deps.go 的装配同形」）之上补「会话 + gin 引擎」。
//
// 不另起第二份构造链——域内测试装置只此一份，契约测试要的是它的 HTTP 面。
// 会话必须带内存黑名单：NewSession 装的是 Redis 存储，测试链路没有 Redis，
// 「注销先写吊销标记」这类端点会恒红（#1445 批 1 的教训，见
// docs/design/1445-api-contract-test-audit.md §8.6）。
//
// 返回值顺序：内存库、已挂 /api 分组的 gin 引擎、会话、config（被测文件在测试体里
// 沿用同一个 JWTSecretKey 签发 token，故一并返回）。
func newForumContractEnv(t *testing.T) (*gorm.DB, *gin.Engine, *security.Session, *config.Config) {
	t.Helper()
	testutil.SetTestGinMode()
	env := newForumTestEnv(t)
	cfg := &config.Config{
		JWTSecretKey: "contract-test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	sess := security.SessionFromConfigWithBlacklist(cfg, testutil.NewValueBlacklist())
	imageSvc := NewImageService(env.db, filestore.NewFileStore("", env.st, zap.NewNop()), zap.NewNop())
	r := gin.New()
	grp := r.Group("/api")
	// 管理端能力解析源（#1618 段1）：域包契约测试自建路由，与装配根 NewRouter 挂同一份实现。
	// 缺了它管理端端点一律 403 —— admin 的能力已改由数据层回答，静态表不再回答它。
	grp.Use(middleware.AdminCapabilityResolver(admincap.New(env.db, zap.NewNop())))
	RegisterAdminRoutes(grp, sess, env.svc, env.mod)
	RegisterRoutes(grp, sess, env.svc, env.mod, imageSvc)
	return env.db, r, sess, cfg
}