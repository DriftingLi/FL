package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"forklift-training/internal/config"
	"forklift-training/internal/security"
)

func init() {
	gin.SetMode(gin.TestMode)
}

const testSecret = "test-jwt-secret"

// generateToken 生成测试用 JWT 令牌。
func generateToken(t *testing.T, userID int, username, role string) string {
	t.Helper()
	claims := &Claims{
		UserID:  userID,
		Account: username,
		Role:    role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// newTestRouter 创建带 JWTAuth + RoleRequired 的测试路由器。
func newTestRouter(cfg *config.Config, roles ...string) *gin.Engine {
	r := gin.New()
	sess := security.SessionFromConfig(cfg)
	protected := r.Group("/protected", JWTAuth(sess))
	if len(roles) > 0 {
		protected.Use(RoleRequired(roles...))
	}
	protected.GET("/endpoint", func(c *gin.Context) {
		uid, _ := c.Get(string(CtxUserID))
		c.JSON(200, gin.H{"user_id": uid})
	})
	return r
}

func TestJWTAuth_ValidToken(t *testing.T) {
	cfg := &config.Config{JWTSecretKey: testSecret}
	r := newTestRouter(cfg)

	token := generateToken(t, 42, "student01", "student")
	req, _ := http.NewRequest("GET", "/protected/endpoint", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("有效 token 应返回 200，得到 %d", w.Code)
	}
}

func TestJWTAuth_MissingToken(t *testing.T) {
	cfg := &config.Config{JWTSecretKey: testSecret}
	r := newTestRouter(cfg)

	req, _ := http.NewRequest("GET", "/protected/endpoint", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("无 token 应返回 401，得到 %d", w.Code)
	}
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	cfg := &config.Config{JWTSecretKey: testSecret}
	r := newTestRouter(cfg)

	req, _ := http.NewRequest("GET", "/protected/endpoint", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-string")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("无效 token 应返回 401，得到 %d", w.Code)
	}
}

func TestJWTAuth_WrongSecret(t *testing.T) {
	cfg := &config.Config{JWTSecretKey: "different-secret"}
	r := newTestRouter(cfg)

	token := generateToken(t, 1, "user", "hrwai_user")
	req, _ := http.NewRequest("GET", "/protected/endpoint", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("错误密钥应返回 401，得到 %d", w.Code)
	}
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	cfg := &config.Config{JWTSecretKey: testSecret}
	r := newTestRouter(cfg)

	// 生成已过期的 token
	claims := &Claims{
		UserID:  1,
		Account: "user",
		Role:    "student",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))

	req, _ := http.NewRequest("GET", "/protected/endpoint", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("过期 token 应返回 401，得到 %d", w.Code)
	}
}

func TestOptionalAuth_WithToken(t *testing.T) {
	cfg := &config.Config{JWTSecretKey: testSecret}
	sess := security.SessionFromConfig(cfg)
	r := gin.New()
	r.GET("/optional", OptionalAuth(sess), func(c *gin.Context) {
		uid, exists := c.Get(string(CtxUserID))
		c.JSON(200, gin.H{"user_id": uid, "exists": exists})
	})

	token := generateToken(t, 10, "optuser", "hrwai_user")
	req, _ := http.NewRequest("GET", "/optional", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("应返回 200，得到 %d", w.Code)
	}
}

func TestOptionalAuth_WithoutToken(t *testing.T) {
	cfg := &config.Config{JWTSecretKey: testSecret}
	sess := security.SessionFromConfig(cfg)
	r := gin.New()
	r.GET("/optional", OptionalAuth(sess), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("GET", "/optional", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("无 token 可选认证应放行，得到 %d", w.Code)
	}
}

func TestOptionalAuth_InvalidToken(t *testing.T) {
	cfg := &config.Config{JWTSecretKey: testSecret}
	sess := security.SessionFromConfig(cfg)
	r := gin.New()
	r.GET("/optional", OptionalAuth(sess), func(c *gin.Context) {
		_, exists := c.Get(string(CtxUserID))
		c.JSON(200, gin.H{"authenticated": exists})
	})

	req, _ := http.NewRequest("GET", "/optional", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-string")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("无效 token 可选认证应放行，得到 %d", w.Code)
	}
	if w.Body.String() != `{"authenticated":false}` {
		t.Fatalf("无效 token 不应写入用户上下文，得到 %s", w.Body.String())
	}
}

func TestOptionalAuth_RevokedToken(t *testing.T) {
	cfg := &config.Config{JWTSecretKey: testSecret}
	sess := security.SessionFromConfig(cfg)

	token := generateToken(t, 42, "optuser", "hrwai_user")

	// 依赖真实 Redis 黑名单（全局缓存）；无 Redis 时跳过
	ctx := context.Background()
	if err := sess.Revoke(ctx, token); err != nil {
		t.Skipf("Redis 不可用，跳过黑名单分支测试: %v", err)
	}
	revoked, _ := sess.IsRevoked(ctx, token)
	if !revoked {
		t.Skip("Redis 黑名单写入失败，跳过")
	}

	r := gin.New()
	r.GET("/optional", OptionalAuth(sess), func(c *gin.Context) {
		_, exists := c.Get(string(CtxUserID))
		c.JSON(200, gin.H{"authenticated": exists})
	})

	req, _ := http.NewRequest("GET", "/optional", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("已吊销 token 可选认证应放行，得到 %d", w.Code)
	}
	if w.Body.String() != `{"authenticated":false}` {
		t.Fatalf("已吊销 token 不应写入用户上下文，得到 %s", w.Body.String())
	}
}

func TestRoleRequired_Allowed(t *testing.T) {
	cfg := &config.Config{JWTSecretKey: testSecret}
	r := newTestRouter(cfg, "admin", "tutor")

	token := generateToken(t, 1, "admin01", "admin")
	req, _ := http.NewRequest("GET", "/protected/endpoint", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("admin 角色应被允许，得到 %d", w.Code)
	}
}

func TestRoleRequired_Denied(t *testing.T) {
	cfg := &config.Config{JWTSecretKey: testSecret}
	r := newTestRouter(cfg, "admin")

	token := generateToken(t, 1, "hrwai01", "hrwai_user")
	req, _ := http.NewRequest("GET", "/protected/endpoint", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Fatalf("hrwai_user 角色应被拒绝 (403)，得到 %d", w.Code)
	}
}

func TestJWTAuth_RevokedToken(t *testing.T) {
	cfg := &config.Config{JWTSecretKey: testSecret}
	r := newTestRouter(cfg)

	token := generateToken(t, 42, "hrwai01", "hrwai_user")

	// 依赖真实 Redis 黑名单（全局缓存）；无 Redis 时跳过
	sess := security.SessionFromConfig(cfg)
	ctx := context.Background()
	if err := sess.Revoke(ctx, token); err != nil {
		t.Skipf("Redis 不可用，跳过黑名单分支测试: %v", err)
	}
	revoked, _ := sess.IsRevoked(ctx, token)
	if !revoked {
		t.Skip("Redis 黑名单写入失败，跳过")
	}

	req, _ := http.NewRequest("GET", "/protected/endpoint", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("已吊销 token 应返回 401，得到 %d", w.Code)
	}
}

func TestRequestID(t *testing.T) {
	r := gin.New()
	r.Use(RequestID())
	r.GET("/", func(c *gin.Context) {
		rid, _ := c.Get(string(CtxRequestID))
		c.String(200, "%s", rid)
	})

	// 自动生成
	req, _ := http.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Body.String() == "" {
		t.Error("应自动生成 request ID")
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Error("应设置 X-Request-ID 响应头")
	}

	// 使用传入的 ID
	req, _ = http.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", "custom-rid")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Body.String() != "custom-rid" {
		t.Errorf("应使用传入的 ID，得到 %q", w.Body.String())
	}
}
