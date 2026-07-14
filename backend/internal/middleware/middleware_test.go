package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"forklift-training/internal/config"
)

func init() {
	gin.SetMode(gin.TestMode)
}

const testSecret = "test-jwt-secret"

// generateToken 生成测试用 JWT 令牌。
func generateToken(t *testing.T, userID int, username, role string) string {
	t.Helper()
	claims := &Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
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
	protected := r.Group("/protected", JWTAuth(cfg))
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

	token := generateToken(t, 1, "user", "student")
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
		UserID:   1,
		Username: "user",
		Role:     "student",
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
	r := gin.New()
	r.GET("/optional", OptionalAuth(cfg), func(c *gin.Context) {
		uid, exists := c.Get(string(CtxUserID))
		c.JSON(200, gin.H{"user_id": uid, "exists": exists})
	})

	token := generateToken(t, 10, "optuser", "student")
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
	r := gin.New()
	r.GET("/optional", OptionalAuth(cfg), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("GET", "/optional", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("无 token 可选认证应放行，得到 %d", w.Code)
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

	token := generateToken(t, 1, "student01", "student")
	req, _ := http.NewRequest("GET", "/protected/endpoint", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Fatalf("student 角色应被拒绝 (403)，得到 %d", w.Code)
	}
}

func TestExtractToken(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("GET", "/", nil)

	// 无 Authorization 头
	c.Request.Header.Del("Authorization")
	if got := extractToken(c); got != "" {
		t.Errorf("无 Authorization 头应返回 ''，得到 %q", got)
	}

	// 有 Bearer token
	c.Request.Header.Set("Authorization", "Bearer abc123")
	if got := extractToken(c); got != "abc123" {
		t.Errorf("应返回 'abc123'，得到 %q", got)
	}

	// 非 Bearer 前缀
	c.Request.Header.Set("Authorization", "Basic abc123")
	if got := extractToken(c); got != "" {
		t.Errorf("非 Bearer 应返回 ''，得到 %q", got)
	}
}

func TestParseToken_Valid(t *testing.T) {
	token := generateToken(t, 5, "parser", "tutor")
	claims, err := parseToken(testSecret, token)
	if err != nil {
		t.Fatalf("有效 token 不应报错: %v", err)
	}
	if claims.UserID != 5 {
		t.Errorf("UserID = %d，期望 5", claims.UserID)
	}
	if claims.Username != "parser" {
		t.Errorf("Username = %q", claims.Username)
	}
	if claims.Role != "tutor" {
		t.Errorf("Role = %q", claims.Role)
	}
}

func TestParseToken_Invalid(t *testing.T) {
	if _, err := parseToken(testSecret, "invalid"); err == nil {
		t.Error("无效 token 应报错")
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
