package course

import (
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// courseContractCredResolver 是课程域契约测试的「当前证件」替身。
//
// 为什么不用 internal/training 的真实实现：course 在 internal/core 的传递依赖闭包内
// （core → … → course），而 training 依赖 course ⇒ course 的包内测试 import 任一者都会
// 撞 `import cycle not allowed in test`。本用例的学员没有 current_credential_id，
// 恒返 (0,false) 与 training.Service.CurrentCredentialID 的真实行为同义
// （见 backend/internal/middleware/credential_scope.go:17-20 的窄接口）。
type courseContractCredResolver struct{}

// CurrentCredentialID 恒报「未选证件」。
func (courseContractCredResolver) CurrentCredentialID(int) (int, bool) { return 0, false }

// newCourseContractRouter 自装课程域 HTTP 面，语义与 internal/api 的
// newContractDepsWithStorage + providers_training.go 那条链同义（st 同样传 nil），
// 只是不经装配根（#1445 域包契约测试下沉）。
//
// 会话必须带内存黑名单：NewSession 装的是 Redis 存储，测试链路没有 Redis，
// 「注销先写吊销标记」这类端点会恒红（#1445 批 1 的教训，见
// docs/design/1445-api-contract-test-audit.md §8.6）。
func newCourseContractRouter(t *testing.T, db *gorm.DB, cfg *config.Config) *gin.Engine {
	t.Helper()
	testutil.SetTestGinMode()
	sess := security.SessionFromConfigWithBlacklist(cfg, testutil.NewValueBlacklist())
	svc := NewService(db, NewSlideRenderer(cfg.LibreOfficeSidecarURL, nil, zap.NewNop()), zap.NewNop())
	r := gin.New()
	RegisterRoutes(r.Group("/api"), sess, courseContractCredResolver{}, svc)
	return r
}
