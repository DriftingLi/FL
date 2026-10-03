package questionbank

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/filestore"
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
	RegisterRoutes(r.Group("/api"), sess, training.NewService(db, zap.NewNop()), svc, fileSvc)
	return r
}

// doWithToken 与 internal/api 的同名助手逐字同口径：body 非 nil 才带请求体（JSON），
// 一律带 Bearer 头。搬进域包后仍需要它 —— testutil.PerformRequest 不带 token，
// 而 testutil.CodeAuthRequest 在 body=nil 时会发 {}（与「不带请求体」不等价）。
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
