package middleware

// 限流键口径（ticket #888）：伪造请求头不能换来新的限流桶，
// 否则「按 IP 限流」形同虚设。
import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"forklift-training/internal/config"
)

// newLimitedRouter 构造一个 burst=1 的限流路由：第一个请求通过，紧随其后的请求被限流。
func newLimitedRouter(t *testing.T, trusted []string) *gin.Engine {
	t.Helper()
	cfg := &config.Config{RateLimit: config.RateLimitConfig{Enabled: true, RPS: 0.0001, Burst: 1}}
	r := gin.New()
	if err := r.SetTrustedProxies(trusted); err != nil {
		t.Fatalf("SetTrustedProxies(%v) 失败: %v", trusted, err)
	}
	r.Use(RateLimit(cfg, zap.NewNop()))
	r.GET("/api/x", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	return r
}

// limitedRequests 依次发起请求，返回每个请求的状态码。
func limitedRequests(t *testing.T, r *gin.Engine, remoteAddr string, xffs []string) []int {
	t.Helper()
	codes := make([]int, 0, len(xffs))
	for _, xff := range xffs {
		req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
		req.RemoteAddr = remoteAddr
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		codes = append(codes, w.Code)
	}
	return codes
}

// 不可信对端轮换伪造头：仍是同一个限流桶，第二个请求即被限流。
func TestRateLimit_SpoofedXForwardedForCannotRotateLimiterKey(t *testing.T) {
	r := newLimitedRouter(t, []string{"192.0.2.10"})
	codes := limitedRequests(t, r, "203.0.113.7:51820", []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"})
	if codes[0] != http.StatusOK {
		t.Fatalf("首个请求应通过: got %d", codes[0])
	}
	for i, code := range codes[1:] {
		if code != http.StatusTooManyRequests {
			t.Errorf("伪造头不应换来新的限流桶（第 %d 个请求）: got %d, want 429", i+2, code)
		}
	}
}

// 可信代理转发的不同客户端：各自独立限流桶，互不影响。
func TestRateLimit_TrustedProxyKeepsClientsInSeparateBuckets(t *testing.T) {
	r := newLimitedRouter(t, []string{"192.0.2.10"})
	codes := limitedRequests(t, r, "192.0.2.10:41234", []string{"198.51.100.1", "198.51.100.2"})
	for i, code := range codes {
		if code != http.StatusOK {
			t.Errorf("不同真实客户端应有独立限流桶（第 %d 个请求）: got %d, want 200", i+1, code)
		}
	}
}

// 可信代理转发的同一个客户端：共享限流桶（证明键确实来自转发头，而非固定取代理地址）。
func TestRateLimit_TrustedProxySharesBucketForSameClient(t *testing.T) {
	r := newLimitedRouter(t, []string{"192.0.2.10"})
	codes := limitedRequests(t, r, "192.0.2.10:41234", []string{"198.51.100.9", "198.51.100.9"})
	if codes[0] != http.StatusOK || codes[1] != http.StatusTooManyRequests {
		t.Errorf("同一真实客户端应共享限流桶: got %v, want [200 429]", codes)
	}
}
