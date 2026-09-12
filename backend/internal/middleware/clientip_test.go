package middleware

// 可信取 IP 口径（ticket #888）的安全闸门：断言的不是「配了可信代理」，
// 而是「伪造的 X-Forwarded-For 不会成为服务端认定的客户端 IP」。
import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// newClientIPRouter 构造只回显「服务端认定的客户端 IP」的最小路由，
// 可信代理按 trusted 配置——与 api.NewRouter 的装配方式一致。
func newClientIPRouter(t *testing.T, trusted []string) *gin.Engine {
	t.Helper()
	r := gin.New()
	if err := r.SetTrustedProxies(trusted); err != nil {
		t.Fatalf("SetTrustedProxies(%v) 失败: %v", trusted, err)
	}
	r.GET("/ip", func(c *gin.Context) { c.String(http.StatusOK, "%s", ClientIP(c)) })
	return r
}

// clientIPFor 以指定 TCP 对端与请求头发起一次请求，返回服务端认定的客户端 IP。
func clientIPFor(t *testing.T, r *gin.Engine, remoteAddr string, headers map[string]string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Body.String()
}

const (
	// 模拟部署里的 nginx：直连后端，转发时把真实客户端追加在 XFF 末尾。
	proxyPeer  = "192.0.2.10:41234"
	clientPeer = "203.0.113.7:51820"
	realClient = "198.51.100.9"
	spoofed    = "114.114.114.114"
)

// 直连形态（对端不在可信网段）：请求头一律不采信，伪造 XFF 不生效。
func TestClientIP_UntrustedPeerCannotSpoofXForwardedFor(t *testing.T) {
	r := newClientIPRouter(t, []string{"192.0.2.10"})
	got := clientIPFor(t, r, clientPeer, map[string]string{"X-Forwarded-For": spoofed})
	if got != "203.0.113.7" {
		t.Errorf("不可信对端的 X-Forwarded-For 不应生效: got %q, want %q", got, "203.0.113.7")
	}
}

// X-Real-IP 是 gin 的第二个候选头（RemoteIPHeaders），同样必须受可信代理约束。
func TestClientIP_UntrustedPeerCannotSpoofXRealIP(t *testing.T) {
	r := newClientIPRouter(t, []string{"192.0.2.10"})
	got := clientIPFor(t, r, clientPeer, map[string]string{"X-Real-IP": spoofed})
	if got != "203.0.113.7" {
		t.Errorf("不可信对端的 X-Real-IP 不应生效: got %q, want %q", got, "203.0.113.7")
	}
}

// 走可信代理：从右往左跳过可信代理，取第一个不可信地址——
// nginx 追加在末尾的真实客户端胜出，客户端自带的伪造值被忽略。
func TestClientIP_TrustedProxyUsesRightmostUntrustedHop(t *testing.T) {
	r := newClientIPRouter(t, []string{"192.0.2.10"})
	got := clientIPFor(t, r, proxyPeer, map[string]string{
		"X-Forwarded-For": spoofed + ", " + realClient,
	})
	if got != "198.51.100.9" {
		t.Errorf("可信代理链应取最右侧不可信地址: got %q, want %q", got, "198.51.100.9")
	}
}

// 没有伪造头时，可信代理转发的单个地址就是真实客户端（限流/审计的正常路径）。
func TestClientIP_TrustedProxyWithoutSpoofedHeader(t *testing.T) {
	r := newClientIPRouter(t, []string{"192.0.2.10"})
	got := clientIPFor(t, r, proxyPeer, map[string]string{"X-Forwarded-For": realClient})
	if got != "198.51.100.9" {
		t.Errorf("可信代理转发的客户端 IP 应被采信: got %q, want %q", got, "198.51.100.9")
	}
}

// 默认口径：不信任任何代理（gin 的默认值是信任所有）。
// 即使对端地址本身出现在可信网段之外，也不会因此被当成代理。
func TestClientIP_NoTrustedProxyIgnoresForwardedHeaders(t *testing.T) {
	r := newClientIPRouter(t, nil)
	got := clientIPFor(t, r, proxyPeer, map[string]string{"X-Forwarded-For": spoofed + ", " + realClient})
	if got != "192.0.2.10" {
		t.Errorf("未配置可信代理时应取 TCP 对端: got %q, want %q", got, "192.0.2.10")
	}
}

// 信任网段过宽的真实后果：真实客户端自己落在可信网段里时，伪造值会赢。
// 这条锁定 gin 的既有语义，也是「只信任部署网段、不要把私网整段写进去」的依据。
func TestClientIP_TrustedClientAddressLetsSpoofWin(t *testing.T) {
	r := newClientIPRouter(t, []string{"192.0.2.10", realClient})
	got := clientIPFor(t, r, proxyPeer, map[string]string{
		"X-Forwarded-For": spoofed + ", " + realClient,
	})
	if got != spoofed {
		t.Errorf("真实客户端也在可信网段时，伪造值会胜出（这正是不能整段信任私网的原因）: got %q", got)
	}
}

// 同一个客户端的不同写法必须映射到同一个键，否则限流键会被别名绕过。
func TestClientIP_NormalizesIPv4MappedIPv6(t *testing.T) {
	r := newClientIPRouter(t, []string{"192.0.2.10"})
	plain := clientIPFor(t, r, proxyPeer, map[string]string{"X-Forwarded-For": realClient})
	mapped := clientIPFor(t, r, proxyPeer, map[string]string{"X-Forwarded-For": "::ffff:" + realClient})
	if mapped != "198.51.100.9" {
		t.Errorf("IPv4-mapped IPv6 应归一成 IPv4: got %q, want %q", mapped, "198.51.100.9")
	}
	if mapped != plain {
		t.Errorf("同一客户端的两种写法应归一成同一个键: %q vs %q", mapped, plain)
	}
}

// 请求头损坏时退回 TCP 对端，而不是把不可信值当成客户端。
func TestClientIP_MalformedForwardedHeaderFallsBackToPeer(t *testing.T) {
	r := newClientIPRouter(t, []string{"192.0.2.10"})
	got := clientIPFor(t, r, proxyPeer, map[string]string{"X-Forwarded-For": "not-an-ip"})
	if got != "192.0.2.10" {
		t.Errorf("损坏的 XFF 应退回 TCP 对端: got %q, want %q", got, "192.0.2.10")
	}
}

// 对端地址无法解析（非 host:port）时返回空串：限流键退化为同一个桶（fail closed），
// 不会伪造出一个凭空的客户端身份。
func TestClientIP_UnparsableRemoteAddrReturnsEmpty(t *testing.T) {
	r := newClientIPRouter(t, []string{"192.0.2.10"})
	got := clientIPFor(t, r, "not-a-host-port", map[string]string{"X-Forwarded-For": spoofed})
	if got != "" {
		t.Errorf("无法解析的对端地址应返回空串: got %q", got)
	}
}
