package contribution

import (
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/admincap"
	"forklift-training/internal/clock"
	"forklift-training/internal/config"
	"forklift-training/internal/filestore"
	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/notification"
	"forklift-training/internal/points"
	"forklift-training/internal/security"
	"forklift-training/internal/storage"
	"forklift-training/internal/testutil"
	"forklift-training/internal/training"
)

// contributionContractDeps 契约测试用的依赖投影。
//
// 原夹具走 internal/api 的 newContractDepsWithStorage 拿 *Deps，域包不得 import 装配根
// （ADR-0070），而这两个用例只用到 `.DB` 一个字段 ⇒ 按调用点裁剪成这一枚，测试体的
// `deps.DB.Create(...)` 因此逐字不动（#1445 批 4）。
type contributionContractDeps struct {
	DB *gorm.DB
}

// newContributionRouter 装配投稿蓝图测试路由器（随域下沉，装配与 internal/api 的
// newContractDepsWithStorage + providers_core.go / providers_contribution.go 同义）。
//
// 第五个返回值是投稿暂存位的本地存储：#1361 之后 Create 要问它「那个暂存文件在不在」，
// 用例必须先往本人的分区 contributions/<uid>/ 里种一个真文件，才谈得上提交成功。
// 注意 storage 传的是**这份本地存储**（不是 nil）——投稿蓝图的第四校验要读它。
func newContributionRouter(t *testing.T) (*gin.Engine, *contributionContractDeps, *model.HrwaiUser, *model.Credential, *storage.LocalStorage) {
	t.Helper()
	testutil.SetTestGinMode()
	db := testutil.NewFileDB(t)
	cfg := &config.Config{
		JWTSecretKey: "contract-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	st := storage.NewLocalStorage(t.TempDir())
	logger := zap.NewNop()
	// 会话黑名单走内存实现：测试链路没有 Redis（审计 §8.6 的教训——
	// security.NewSession 装的是 Redis 存储，「注销先写吊销标记」这类端点会恒红）。
	sess := security.SessionFromConfigWithBlacklist(cfg, testutil.NewValueBlacklist())
	fileSvc := filestore.NewFileStore(cfg.LibreOfficeSidecarURL, st, logger)
	notifSvc := notification.NewService(db, logger)
	pointsSvc := points.NewService(db, logger, clock.Real(), notifSvc)
	svc := NewService(db, fileSvc, notifSvc, pointsSvc, logger, clock.Real())
	// CredentialScope：装配根给的是 TrainingCatalogSvc（deps.go 的 RouterDeps()）。
	credRes := training.NewService(db, logger)

	r := gin.New()
	api := r.Group("/api")
	// 管理端能力解析源（#1618 段1）：自建路由与装配根 NewRouter 挂同一份实现，缺了它管理端端点一律 403。
	api.Use(middleware.AdminCapabilityResolver(admincap.New(db, zap.NewNop())))
	RegisterRoutes(api, sess, credRes, svc)
	RegisterAdminRoutes(api, sess, svc)

	// 学员（已选证件）
	cred := &model.Credential{Code: "N1", Name: "叉车司机"}
	if err := db.Create(cred).Error; err != nil {
		t.Fatalf("建证件失败: %v", err)
	}
	cid := cred.ID
	stu := &model.HrwaiUser{Account: "acct_c", Phone: "13800000001", Username: "学员丙", Status: 1, CurrentCredentialID: &cid, CreatedAt: testutil.Now()}
	if err := db.Create(stu).Error; err != nil {
		t.Fatalf("建学员失败: %v", err)
	}
	return r, &contributionContractDeps{DB: db}, stu, cred, st
}
