package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/config"
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
	return NewDeps(cfg, db, nil, zap.NewNop(), nil)
}
