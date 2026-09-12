package api

// 可信取 IP 口径的路由装配契约（ticket #888）：断言的不是「调用过 SetTrustedProxies」，
// 而是真实装配出来的路由器在收到伪造 X-Forwarded-For 时，服务端认定的客户端 IP 仍是 TCP 对端。
// 观察点用访问日志的 ip 字段（与审计日志、限流键同源：middleware.ClientIP）。
import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"forklift-training/internal/config"
	"forklift-training/internal/testutil"
)

// newTrustedProxyRouter 用给定可信代理装配真实路由器，并捕获访问日志。
func newTrustedProxyRouter(t *testing.T, trusted []string) (*gin.Engine, *strings.Builder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var buf strings.Builder
	enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	logger := zap.New(zapcore.NewCore(enc, zapcore.AddSync(&buf), zapcore.InfoLevel))

	cfg := &config.Config{TrustedProxies: trusted}
	deps := newContractDeps(t, testutil.NewMemoryDB(t), cfg)
	deps.Logger = logger
	return NewRouter(deps), &buf
}

// accessLogIP 发起一次请求，返回访问日志最后一条记录的 ip 字段。
func accessLogIP(t *testing.T, r *gin.Engine, buf *strings.Builder, remoteAddr, xff string) string {
	t.Helper()
	buf.Reset()
	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api 应返回 200: got %d, body %s", w.Code, w.Body.String())
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry); err != nil {
		t.Fatalf("访问日志应为 JSON: %v (%s)", err, buf.String())
	}
	ip, _ := entry["ip"].(string)
	if ip == "" {
		t.Fatalf("访问日志缺少 ip 字段: %s", buf.String())
	}
	return ip
}

const proxyPeerAddr = "192.0.2.10:41234"

func TestNewRouter_SpoofedXForwardedForIsNotTheClientIP(t *testing.T) {
	r, buf := newTrustedProxyRouter(t, []string{"192.0.2.10"})
	got := accessLogIP(t, r, buf, "203.0.113.7:51820", "114.114.114.114")
	if got != "203.0.113.7" {
		t.Errorf("伪造的 X-Forwarded-For 不应成为客户端 IP: got %q, want %q", got, "203.0.113.7")
	}
}

func TestNewRouter_TrustedProxyResolvesForwardedClientIP(t *testing.T) {
	r, buf := newTrustedProxyRouter(t, []string{"192.0.2.10"})
	got := accessLogIP(t, r, buf, proxyPeerAddr, "114.114.114.114, 198.51.100.9")
	if got != "198.51.100.9" {
		t.Errorf("可信代理转发的真实客户端 IP 应被采信: got %q, want %q", got, "198.51.100.9")
	}
}

// 未配置可信代理（默认）时，任何对端都不被当作代理——包括看起来像代理的地址。
func TestNewRouter_DefaultsToTrustingNoProxy(t *testing.T) {
	r, buf := newTrustedProxyRouter(t, nil)
	got := accessLogIP(t, r, buf, proxyPeerAddr, "114.114.114.114, 198.51.100.9")
	if got != "192.0.2.10" {
		t.Errorf("默认应不信任任何代理，取 TCP 对端: got %q, want %q", got, "192.0.2.10")
	}
}
