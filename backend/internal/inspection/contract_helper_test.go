package inspection

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/config"
	"forklift-training/internal/notification"
	"forklift-training/internal/points"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// newInspectionContractRouter 自装巡检域 HTTP 面，语义与 internal/api 的
// newContractDepsWithStorage + providers_contribution.go 那条链同义（pointsSvc 按
// providers_core.go 的实参构造），只是不经装配根（#1445 域包契约测试下沉）。
//
// 会话必须带内存黑名单（#1445 批 1 的教训，见审计 §8.6）。
func newInspectionContractRouter(t *testing.T, db *gorm.DB, cfg *config.Config) *gin.Engine {
	t.Helper()
	testutil.SetTestGinMode()
	sess := security.SessionFromConfigWithBlacklist(cfg, testutil.NewValueBlacklist())
	pointsSvc := points.NewService(db, zap.NewNop(), clock.Real(), notification.NewService(db, zap.NewNop()))
	r := gin.New()
	RegisterRoutes(r.Group("/api"), sess, NewService(db), pointsSvc)
	return r
}

// doWithToken 与 internal/api 的同名助手逐字同口径（body 非 nil 才带请求体；一律带 Bearer 头）。
func doWithToken(t *testing.T, r *gin.Engine, token, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		req, _ = http.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, path, nil)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
