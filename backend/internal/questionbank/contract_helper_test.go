package questionbank

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
	"forklift-training/internal/training"
)

// newQuestionBankContractRouter 自装题库域写面，语义与 internal/api 的
// newContractDepsWithStorage + providers_exam.go 那条链同义（fileSvc 同样用 nil 存储、
// CredentialScope 同样取 training.Service），只是不经装配根（#1445 域包契约测试下沉）。
//
// 会话必须带内存黑名单（#1445 批 1 的教训，见审计 §8.6）。
func newQuestionBankContractRouter(t *testing.T, db *gorm.DB, cfg *config.Config) *gin.Engine {
	t.Helper()
	testutil.SetTestGinMode()
	sess := security.SessionFromConfigWithBlacklist(cfg, testutil.NewValueBlacklist())
	fileSvc := filestore.NewFileStore(cfg.LibreOfficeSidecarURL, nil, zap.NewNop())
	svc := NewService(db, fileSvc, zap.NewNop())
	r := gin.New()
	api := r.Group("/api")
	// 管理端能力解析源（#1618 段1）：自建路由与装配根 NewRouter 挂同一份实现，缺了它管理端端点一律 403。
	api.Use(middleware.AdminCapabilityResolver(admincap.New(db, zap.NewNop())))
	RegisterRoutes(api, sess, training.NewService(db, zap.NewNop()), svc, fileSvc)
	return r
}