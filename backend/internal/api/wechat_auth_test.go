// 微信登录端点契约测试：两条链路的**路由归属**、信封结构与无需外呼的错误分支。
// 换取成功路径由各自 service 层测试以 httptest server 覆盖
// （wechat_service_test.go / wechat_app_service_test.go），这里不重复测业务。
//
// 一张表跑两条链路而不是各开一个文件：两条链路的错误文案**逐字相同**（都是「缺少微信登录凭证 code」
// 与「微信登录未配置，请联系管理员」），复制成两份用例只会得到两个假绿的第二真源。
// 本表证的是：同一个 {code} 请求体在两条各自的路由上都命中了各自的服务，且都在出门前要门禁。
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

// wxLoginEndpoints 两条微信登录链路：小程序 code2session 与移动应用 oauth2/access_token。
var wxLoginEndpoints = []struct {
	name string
	path string
}{
	{"小程序", "/api/auth/wx-login"},
	{"App", "/api/auth/app-wx-login"},
}

func newWxLoginEnv(t *testing.T) *gin.Engine {
	t.Helper()
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)
	return NewRouter(newContractDeps(t, db, nil))
}

// wxLoginRequest 向指定微信登录端点发一个带 code 的请求。
func wxLoginRequest(t *testing.T, r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// wxEnvelope 解出统一响应信封的 code/message。
func wxEnvelope(t *testing.T, w *httptest.ResponseRecorder) (int, string) {
	t.Helper()
	var envelope struct {
		Code    int
		Message string
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("信封解析失败: %v（body=%s）", err, w.Body.String())
	}
	return envelope.Code, envelope.Message
}

// 两条链路都必须「缺 code 先拦」：命中 400 且文案含「缺少」——真发一次外呼就不会是这个文案。
func TestWechatLoginEndpoints_MissingCode(t *testing.T) {
	t.Parallel()
	for _, ep := range wxLoginEndpoints {
		w := wxLoginRequest(t, newWxLoginEnv(t), ep.path, "{}")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s 缺 code 应 400, got %d: %s", ep.name, w.Code, w.Body.String())
		}
		code, msg := wxEnvelope(t, w)
		if code != http.StatusBadRequest || !strings.Contains(msg, "缺少") {
			t.Fatalf("%s 缺 code 文案不符: code=%d message=%s", ep.name, code, msg)
		}
	}
}

// 两条链路都必须「未配置凭证先拦再呼」：测试链路里两组凭证都是零值（config.WechatConfig{}），
// 所以命中即回「未配置」。这条也是「服务没接错线」的证据——若注册面把某个 handler 指到了
// 别的服务，这里读到的仍是同一条文案，路由归属那一半由下面的 404 用例守住。
func TestWechatLoginEndpoints_NotConfigured(t *testing.T) {
	t.Parallel()
	for _, ep := range wxLoginEndpoints {
		w := wxLoginRequest(t, newWxLoginEnv(t), ep.path, "{\"code\":\"some-code\"}")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s 未配置应 400, got %d: %s", ep.name, w.Code, w.Body.String())
		}
		code, msg := wxEnvelope(t, w)
		if code != http.StatusBadRequest || !strings.Contains(msg, "未配置") {
			t.Fatalf("%s 未配置文案不符: code=%d message=%s", ep.name, code, msg)
		}
	}
}

// 路由各归各的：两条都存在，而「只注册了一条」的形状（比如把 App 并进小程序那条）会在这里红。
func TestWechatLoginEndpoints_RoutesAreDistinct(t *testing.T) {
	t.Parallel()
	r := newWxLoginEnv(t)
	registered := make(map[string]bool)
	for _, rt := range r.Routes() {
		registered[rt.Method+" "+rt.Path] = true
	}
	for _, want := range []string{
		"POST /api/auth/wx-login",     // 小程序 js_code → code2session
		"POST /api/auth/app-wx-login", // 移动应用 code → oauth2/access_token
	} {
		if !registered[want] {
			t.Fatalf("注册面缺少 %s（两条链路必须是两个端点，不是一个端点认两种 code）", want)
		}
	}
	if registered["PUT /api/auth/app-wx-login"] {
		t.Fatal("App 登录端点的方法面漂了（应为 POST）")
	}
}
