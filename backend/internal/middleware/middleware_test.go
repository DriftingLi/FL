package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
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
		UserID:    userID,
		Account:   username,
		Role:      role,
		TokenType: security.TokenTypeAccess,
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

// memBlacklistStore 内存版黑名单存储（测试注入双，替代真实 Redis，断言确定性执行）。
type memBlacklistStore struct {
	mu sync.Mutex
	m  map[string]string
}

func newMemBlacklistStore() *memBlacklistStore {
	return &memBlacklistStore{m: map[string]string{}}
}

func (s *memBlacklistStore) Get(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[key]; !ok {
		return "", errors.New("not found")
	}
	return "1", nil
}

func (s *memBlacklistStore) Set(_ context.Context, key, value string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
	return nil
}

// PutIfAbsent 原子抢占（SETNX 语义），与生产 Redis 行为对齐。
func (s *memBlacklistStore) PutIfAbsent(_ context.Context, key, value string, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[key]; ok {
		return false, nil
	}
	s.m[key] = value
	return true, nil
}

// TestOptionalAuth_LogoutRevokesRefreshOnly 新世界（ADR-0016）：黑名单只管理 refresh。
// 登出（吊销 refresh）后未过期的旧 access 照常通过可选认证；refresh 传入鉴权端点被类型分流拒绝。
func TestOptionalAuth_LogoutRevokesRefreshOnly(t *testing.T) {
	store := newMemBlacklistStore()
	sess := security.NewSessionWithBlacklistAndRefresh(testSecret, time.Hour, time.Hour, security.CookieConfig{Name: "hrwai_token"}, store)

	access, refresh, err := sess.IssuePair(42, "optuser", "hrwai_user")
	if err != nil {
		t.Fatalf("签发双令牌失败: %v", err)
	}
	// 登出语义：仅吊销 refresh（access 不入黑名单，自然过期）
	if err := sess.RevokeRefresh(context.Background(), refresh); err != nil {
		t.Fatalf("登出吊销 refresh 失败: %v", err)
	}

	r := gin.New()
	r.GET("/optional", OptionalAuth(sess), func(c *gin.Context) {
		_, exists := c.Get(string(CtxUserID))
		c.JSON(200, gin.H{"authenticated": exists})
	})

	// 未过期的旧 access 不受登出影响
	req, _ := http.NewRequest("GET", "/optional", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 || w.Body.String() != `{"authenticated":true}` {
		t.Fatalf("登出仅吊销 refresh, access 应照常认证: code=%d body=%s", w.Code, w.Body.String())
	}

	// refresh 传入鉴权端点被拒绝（token_type 分流）
	req2, _ := http.NewRequest("GET", "/optional", nil)
	req2.Header.Set("Authorization", "Bearer "+refresh)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != 200 || w2.Body.String() != `{"authenticated":false}` {
		t.Fatalf("refresh 不应写入用户上下文: code=%d body=%s", w2.Code, w2.Body.String())
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

// TestJWTAuth_LogoutRevokesRefreshOnly 新世界（ADR-0016）：JWTAuth 只认 access、不读黑名单。
// 登出（吊销 refresh）后旧 access 在有效期内仍可访问；refresh 作为 Bearer 传入返回 401。
func TestJWTAuth_LogoutRevokesRefreshOnly(t *testing.T) {
	store := newMemBlacklistStore()
	sess := security.NewSessionWithBlacklistAndRefresh(testSecret, time.Hour, time.Hour, security.CookieConfig{Name: "hrwai_token"}, store)

	access, refresh, err := sess.IssuePair(42, "hrwai01", "hrwai_user")
	if err != nil {
		t.Fatalf("签发双令牌失败: %v", err)
	}
	if err := sess.RevokeRefresh(context.Background(), refresh); err != nil {
		t.Fatalf("登出吊销 refresh 失败: %v", err)
	}

	r := gin.New()
	r.GET("/protected", JWTAuth(sess), func(c *gin.Context) {
		c.JSON(200, gin.H{"user_id": 42})
	})

	// access 吊销不影响访问（登出只吊销 refresh）
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("登出后未过期 access 应照常访问，得到 %d", w.Code)
	}

	// refresh 传入鉴权端点被拒（VerifyAccess 类型分流）
	req2, _ := http.NewRequest("GET", "/protected", nil)
	req2.Header.Set("Authorization", "Bearer "+refresh)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != 401 {
		t.Fatalf("refresh 传入鉴权端点应 401，得到 %d", w2.Code)
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
