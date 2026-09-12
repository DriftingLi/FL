package api

// 限流键的装配层契约（ticket #888）：走真实 NewRouter——限流中间件挂在引擎上，
// 若装配时没把可信代理交给 gin，伪造的 X-Forwarded-For 就能换来新的限流桶。
import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/testutil"
)

// newRateLimitedRouter 装配真实路由器，并把限流收紧到「第二个请求必被拒」。
func newRateLimitedRouter(t *testing.T, trusted []string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		TrustedProxies: trusted,
		RateLimit:      config.RateLimitConfig{Enabled: true, RPS: 0.0001, Burst: 1},
	}
	return NewRouter(newContractDeps(t, testutil.NewMemoryDB(t), cfg))
}

// limiterCodes 依次发起请求，返回每个请求的状态码。
func limiterCodes(t *testing.T, r *gin.Engine, remoteAddr string, headers []map[string]string) []int {
	t.Helper()
	codes := make([]int, 0, len(headers))
	for _, hs := range headers {
		req := httptest.NewRequest(http.MethodGet, "/api", nil)
		req.RemoteAddr = remoteAddr
		for k, v := range hs {
			req.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		codes = append(codes, w.Code)
	}
	return codes
}

func TestNewRouter_SpoofedXForwardedForCannotRotateLimiterBucket(t *testing.T) {
	r := newRateLimitedRouter(t, []string{"192.0.2.10"})
	codes := limiterCodes(t, r, "203.0.113.7:51820", []map[string]string{
		{"X-Forwarded-For": "1.1.1.1"},
		{"X-Forwarded-For": "2.2.2.2"},
		{"X-Forwarded-For": "3.3.3.3"},
	})
	if codes[0] != http.StatusOK {
		t.Fatalf("首个请求应通过: got %d", codes[0])
	}
	for i, code := range codes[1:] {
		if code != http.StatusTooManyRequests {
			t.Errorf("伪造头不应换来新的限流桶（第 %d 个请求）: got %d, want 429", i+2, code)
		}
	}
}

// 反向断言：可信代理转发的不同客户端各自成桶——证明键确实来自转发头，而不是固定取对端。
func TestNewRouter_TrustedProxyKeepsClientsInSeparateLimiterBuckets(t *testing.T) {
	r := newRateLimitedRouter(t, []string{"192.0.2.10"})
	codes := limiterCodes(t, r, "192.0.2.10:41234", []map[string]string{
		{"X-Forwarded-For": "198.51.100.1"},
		{"X-Forwarded-For": "198.51.100.2"},
	})
	for i, code := range codes {
		if code != http.StatusOK {
			t.Errorf("不同真实客户端应有独立限流桶（第 %d 个请求）: got %d, want 200", i+1, code)
		}
	}
}
