// 微信小程序登录端点契约测试：路由路径、信封结构与错误分支。
// 成功路径（code2session 外呼）由 service 层测试以 httptest server 覆盖，
// 此处覆盖无需外呼的两条分支：缺 code、未配置 AppID/Secret。
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/testutil"
)

func newWxLoginEnv(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	return NewRouter(newContractDeps(t, db, nil))
}

// wxLoginRequest 模拟前端 uni.login 后携带 code 的请求体。
func wxLoginRequest(t *testing.T, r *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, "/api/auth/wx-login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestWxLogin_MissingCode(t *testing.T) {
	w := wxLoginRequest(t, newWxLoginEnv(t), "{}")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺 code 应 400, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Code    int
		Message string
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("信封解析失败: %v", err)
	}
	if body.Code != http.StatusBadRequest || !strings.Contains(body.Message, "缺少") {
		t.Fatalf("缺 code 文案不符: %+v", body)
	}
}

func TestWxLogin_NotConfigured(t *testing.T) {
	w := wxLoginRequest(t, newWxLoginEnv(t), "{\"code\":\"js-code\"}")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("未配置应 400, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Code    int
		Message string
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("信封解析失败: %v", err)
	}
	if body.Code != http.StatusBadRequest || !strings.Contains(body.Message, "未配置") {
		t.Fatalf("未配置文案不符: %+v", body)
	}
}
