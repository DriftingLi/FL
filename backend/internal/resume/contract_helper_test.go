package resume_test

import (
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/auth"
	"forklift-training/internal/config"
	"forklift-training/internal/core"
	"forklift-training/internal/filestore"
	"forklift-training/internal/resume"
	"forklift-training/internal/security"
	"forklift-training/internal/storage"
	"forklift-training/internal/testutil"
)

// resumeContractDeps 保留装配根字段名（AuthSvc），让用例体里的
// `deps.AuthSvc.DeleteAccount(...)` 那行逐字不动。
type resumeContractDeps struct {
	AuthSvc *auth.Service
}

// newResumeContractDeps 自装简历域 HTTP 面，语义与 internal/api 的
// newContractDepsWithStorage + providers_jobs.go / providers_core.go 那条链同义
// （fileSvc 用 TempDir 支持的本地存储 —— 原用例显式覆写过，PDF/图片端点在用例里真写文件）。
//
// **本文件是 `package resume_test`（外部测试包）**：用例末尾要调
// `auth.Service.DeleteAccount` 验级联删除，而 auth 的传递闭包包含 resume
// （auth → core → … → resume）⇒ 包内测试（package resume）import auth 会
// `import cycle not allowed in test`；外部测试包住在被测包之外，两条边都能拿
// （先例：backend/internal/middleware/audit_ip_test.go，见 internal/layers 的说明）。
//
// 会话必须带内存黑名单（#1445 批 1 的教训，见审计 §8.6）。
func newResumeContractDeps(t *testing.T, db *gorm.DB, cfg *config.Config) (*gin.Engine, *resumeContractDeps) {
	t.Helper()
	testutil.SetTestGinMode()
	sess := security.SessionFromConfigWithBlacklist(cfg, testutil.NewValueBlacklist())
	st := storage.NewLocalStorage(t.TempDir())
	fileSvc := filestore.NewFileStore("", st, zap.NewNop())
	jobSvc := resume.NewService(db, fileSvc, zap.NewNop())
	authSvc := auth.NewService(db, sess, core.NewForumCounter(),
		cfg.DefaultPasswords.Admin, cfg.DefaultPasswords.Tutor, cfg.DefaultPasswords.Student, zap.NewNop())
	r := gin.New()
	resume.RegisterRoutes(r.Group("/api"), sess, jobSvc, fileSvc)
	return r, &resumeContractDeps{AuthSvc: authSvc}
}
