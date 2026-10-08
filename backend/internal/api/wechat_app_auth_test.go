// App 端微信登录端点契约测试（#1482）：路由路径、信封结构与无需外呼的两条错误分支。
// 换取成功路径由 service 层测试以 httptest server 覆盖（wechat_app_service_test.go）。
//
// 本文件存在的理由不是再测一遍 service，而是钉住**端点归属**：
// 测试链路里的 cfg.Wechat.Mobile 是零值，所以命中本端点必然报「未配置」而不是「js_code 无效」——
// 一旦有人把 App 的 code 重新指回 /auth/wx-login（那条只认小程序凭证），这两条用例立刻红。
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

func newAppWxLoginEnv(t *testing.T) *gin.Engine {
	t.Helper()
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)
	return NewRouter(newContractDeps(t, db, nil))
}

func appWxLoginRequest(t *testing.T, r *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, "/api/auth/app-wx-login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAppWxLogin_MissingCode(t *testing.T) {
	t.Parallel()
	w := appWxLoginRequest(t, newAppWxLoginEnv(t), "{}")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺 code 应 400, got %d: %s", w.Code, w.Body.String())
	}
	var envelope struct {
		Code    int
		Message string
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("信封解析失败: %v", err)
	}
	if envelope.Code != http.StatusBadRequest || !strings.Contains(envelope.Message, "缺少") {
		t.Fatalf("缺 code 文案不符: %+v", envelope)
	}
}

func TestAppWxLogin_NotConfigured(t *testing.T) {
	t.Parallel()
	w := appWxLoginRequest(t, newAppWxLoginEnv(t), "{\"code\":\"app-code\"}")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("未配置移动应用凭证应 400, got %d: %s", w.Code, w.Body.String())
	}
	var envelope struct {
		Code    int
		Message string
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("信封解析失败: %v", err)
	}
	if envelope.Code != http.StatusBadRequest || !strings.Contains(envelope.Message, "未配置") {
		t.Fatalf("未配置文案不符: %+v", envelope)
	}
}
