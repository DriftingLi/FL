package featured

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/filestore"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// newFeaturedContractEnv 装配内容精选测试路由（随域下沉，#1445 批 4）。
//
// 与 internal/api 的 newContractDeps(t, db, nil) + providers_training.go 同义：
// cfg 走零值（原链传的就是 nil ⇒ &config.Config{}）、fileSvc 传 st=nil、
// 会话用内存黑名单构造（测试链路没有 Redis，审计 §8.6）。
func newFeaturedContractEnv(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()
	cfg := &config.Config{}
	logger := zap.NewNop()
	sess := security.SessionFromConfigWithBlacklist(cfg, testutil.NewValueBlacklist())
	fileSvc := filestore.NewFileStore(cfg.LibreOfficeSidecarURL, nil, logger)
	svc := NewService(db, fileSvc, logger)

	r := gin.New()
	// 上传适配器传替身：这两个用例只打 GET 详情 / POST view，从不走上传路由。
	// 真实现是信封单点 backend/internal/api/vditor_upload.go（tutor 与 featured 共用），
	// 不复制进域包；万一将来有人真打通上传面，这里立刻报错而不是静默通过。
	uploader := func(c *gin.Context, _ *filestore.FileStore, _ func([]byte, string) (string, error)) {
		t.Errorf("featured 契约测试不打上传面，却被调用：%s %s", c.Request.Method, c.Request.URL.Path)
		c.Status(http.StatusInternalServerError)
	}
	RegisterRoutes(r.Group("/api"), sess, svc, fileSvc, uploader)
	return r
}
