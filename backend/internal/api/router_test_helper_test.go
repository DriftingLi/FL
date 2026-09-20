package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/security"
)

// performRequest 向测试路由器发起 HTTP 请求并返回响应记录器。
func performRequest(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, nil)
	r.ServeHTTP(w, req)
	return w
}

// newContractDeps 构建契约测试用的完整装配根（storage 与导出 store 传 nil，被测蓝图不使用）。
func newContractDeps(t *testing.T, db *gorm.DB, cfg *config.Config) *Deps {
	t.Helper()
	if cfg == nil {
		cfg = &config.Config{}
	}
	d := NewDeps(cfg, db, nil, zap.NewNop(), nil)
	// 测试链路没有 Redis：会话模块的黑名单存储换成内存实现，否则「注销先写吊销标记」
	// 这类要真实写凭证状态的端点只能判红（ADR-0060 票2）。Cookie 与有效期口径不变。
	d.Session = security.SessionFromConfigWithBlacklist(cfg, newValBlacklist())
	// NewDeps 在装配根内就把 Session 注给了各 handler（ADR-0047 手写装配），故一并改指。
	d.AuthH.session = d.Session
	return d
}
