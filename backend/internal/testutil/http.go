// 测试用 HTTP / 会话脚手架：跨包共用的请求构造、断言与内存黑名单。
//
// 为什么住在这里：域包（internal/<域>）的契约测试要与装配根（internal/api）的测试用
// **同一份**请求构造与断言，而 testutil 是两边都能 import 的普通包（域包不得 import
// internal/api —— 反向依赖成环）。这里只放**无域语义**的件：请求构造、字典键集断言、
// gin 测试模式与黑名单内存实现；域内夹具（路由装配、测试通道替身）仍各自住在域包里。
package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// SetTestGinMode 幂等地把 gin 切到 TestMode（#1366 测试面瘦身）。
//
// 契约测试改用 t.Parallel 后，若每个并行用例各自直接调 gin.SetMode(gin.TestMode)，
// 会并发写 gin 的包级全局（运行模式位与错误 writer），在 CI 的 -race 下即数据竞态。
// 用 sync.Once 把整个测试进程里的这次设置收敛成「只真正发生一次」：Once 让各调用点
// 相互串行并建立 happens-before，模式位一旦落定不再被并发改写。它不引入第二份事实源——
// 只是把「每个文件各写一遍 SetMode」收成「同一条链上设置一次」。
//
// #1445 测试面下沉：本件原住 internal/api/router_test_helper_test.go，域包测试同样需要它；
// 域包不得 import internal/api，故提到这里由两侧共用（仍只有一处实现）。
var SetTestGinMode = sync.OnceFunc(func() { gin.SetMode(gin.TestMode) })

// PerformRequest 向测试路由器发起一次无正文 HTTP 请求并返回响应记录器。
func PerformRequest(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, nil)
	r.ServeHTTP(w, req)
	return w
}

// CodeAuthRequest 发一枚 JSON 请求（body 为 nil 时发空对象），token 非空则带 Bearer 头。
func CodeAuthRequest(r *gin.Engine, method, path string, body map[string]interface{}, token string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body == nil {
		body = map[string]interface{}{}
	}
	_ = json.NewEncoder(&buf).Encode(body)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r.ServeHTTP(rec, req)
	return rec
}

// ExtractToken 从响应正文里取 "token" 字段值（登录/注册响应的取值口径）。
func ExtractToken(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	body := w.Body.String()
	idx := strings.Index(body, `"token":"`)
	if idx < 0 {
		t.Fatalf("响应缺少 token: %s", body)
	}
	rest := body[idx+len(`"token":"`):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		t.Fatalf("token 解析失败: %s", body)
	}
	return rest[:end]
}

// AssertDictKeys 断言字典对象键集与期望集合完全一致（map 序列化按键排序，
// 反序列化后键集一致 + service 层键序测试 → 字节级契约锁定）。
func AssertDictKeys(t *testing.T, got map[string]any, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("字典键数不符: got %d keys %v, want %d", len(got), KeysOf(got), len(want))
	}
	for _, k := range want {
		if _, ok := got[k]; !ok {
			t.Fatalf("缺少字段 %q: %v", k, KeysOf(got))
		}
	}
}

// KeysOf 取字典对象的键集（失败信息里展示用）。
func KeysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// ValueBlacklist 保留写入值的内存黑名单存储（生产走 Redis）。
//
// 它区别于只记 key 存在的「存在性」替身：用户级吊销标记的**值**是时间戳，
// 只记 key 的实现会把标记读成 1970 年、判成「未吊销」。
// PutIfAbsent 实现 SETNX 语义（原子抢占），与生产 Redis 行为对齐。
type ValueBlacklist struct {
	mu sync.Mutex
	m  map[string]string
}

// NewValueBlacklist 新建一份空的保留值内存黑名单。
func NewValueBlacklist() *ValueBlacklist { return &ValueBlacklist{m: map[string]string{}} }

func (s *ValueBlacklist) Get(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[key]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}

func (s *ValueBlacklist) Set(_ context.Context, key, value string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
	return nil
}

func (s *ValueBlacklist) PutIfAbsent(_ context.Context, key, value string, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[key]; ok {
		return false, nil
	}
	s.m[key] = value
	return true, nil
}
