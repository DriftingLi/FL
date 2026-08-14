// 验证码认证路由 handler 测试（#17 通道化收尾）：
// 邮箱/手机注册登录走同一份骨架（CodeChannel 驱动生成器），路由形状与行为不变。
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/cache"
	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

var errCodeNotFound = errors.New("code not found")

// =====================================================
// 测试替身：内存验证码存储 + 测试通道
// =====================================================

type memCodeStore struct {
	m map[string]string
}

func newMemCodeStore() *memCodeStore {
	return &memCodeStore{m: map[string]string{}}
}

func (s *memCodeStore) Get(_ context.Context, key string) (string, error) {
	v, ok := s.m[key]
	if !ok {
		return "", errCodeNotFound
	}
	return v, nil
}

func (s *memCodeStore) Set(_ context.Context, key, value string, ttl time.Duration) error {
	s.m[key] = value
	return nil
}

func (s *memCodeStore) Del(_ context.Context, keys ...string) error {
	for _, k := range keys {
		delete(s.m, k)
	}
	return nil
}

// fakeChannel 通用测试通道：按 column 查 hrwai_users（email / phone 两套）。
type fakeChannel struct {
	column  string
	keyPref string
	noun    string
}

func (c *fakeChannel) SenderReady() error { return nil }
func (c *fakeChannel) Normalize(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", errCodeNotFound
	}
	return target, nil
}
func (c *fakeChannel) Noun() string      { return c.noun }
func (c *fakeChannel) KeyPrefix() string { return c.keyPref }

func (c *fakeChannel) FindAccount(ctx context.Context, db *gorm.DB, target string, excludeUserID int) (int64, error) {
	var count int64
	q := db.WithContext(ctx).Model(&model.HrwaiUser{}).Where(c.column+" = ?", target)
	if excludeUserID > 0 {
		q = q.Where("id <> ?", excludeUserID)
	}
	if err := q.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (c *fakeChannel) FindUser(ctx context.Context, db *gorm.DB, target string) (*model.HrwaiUser, error) {
	var user model.HrwaiUser
	if err := db.WithContext(ctx).Where(c.column+" = ?", target).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *fakeChannel) Render(purpose service.CodePurpose, code string, ttl time.Duration) (string, string) {
	return "title", "code=" + code
}

func (c *fakeChannel) Send(target, title, body, _ string, _ time.Duration) error { return nil }

func (c *fakeChannel) ApplyTarget(user *model.HrwaiUser, target string) {
	switch c.column {
	case "email":
		user.Email = target
	case "phone":
		user.Phone = target
	}
}

func (c *fakeChannel) BindColumn() string { return c.column }

// =====================================================
// 路由装配 + 请求 helper
// =====================================================

func newCodeAuthTestRouter(t *testing.T) (*gin.Engine, *memCodeStore, *fakeChannel, *fakeChannel) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	authSvc := service.NewAuthService(db, security.NewSession("test-secret", time.Hour, security.CookieConfig{}), "admin", "tutor", "student", zap.NewNop())
	store := newMemCodeStore()
	codeSvc := service.NewVerifyCodeService(db, authSvc, 5*time.Minute, store, zap.NewNop())

	emailCh := &fakeChannel{column: "email", keyPref: "email_code", noun: "邮箱"}
	phoneCh := &fakeChannel{column: "phone", keyPref: "phone_code", noun: "手机号"}

	cfg := &config.Config{
		JWTSecretKey: "test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: false},
	}

	deps := &Deps{
		Cfg:     cfg,
		DB:      db,
		Logger:  zap.NewNop(),
		Session: security.SessionFromConfig(cfg),
		AuthSvc: authSvc,
		CodeSvc: codeSvc,
		EmailCh: emailCh,
		PhoneCh: phoneCh,
	}

	r := gin.New()
	r.Use(gin.Recovery())
	api := r.Group("/api")
	RegisterEmailAuthRoutes(api, deps.RouterDeps(), deps.CodeSvc, deps.EmailCh)
	RegisterPhoneAuthRoutes(api, deps.RouterDeps(), deps.CodeSvc, deps.PhoneCh)
	RegisterProfileBindRoutes(api, deps.RouterDeps(), deps.AuthSvc, deps.CodeSvc, deps.EmailCh, deps.PhoneCh)

	return r, store, emailCh, phoneCh
}

func codeAuthRequest(r *gin.Engine, method, path string, body map[string]interface{}, token string) *httptest.ResponseRecorder {
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

// extractStoredCode 按与生产一致的 key 公式从内存 store 取验证码（存储值为 authCodeValue JSON）。
func extractStoredCode(t *testing.T, store *memCodeStore, ch *fakeChannel, purpose service.CodePurpose, target string) string {
	t.Helper()
	raw, ok := store.m[cache.SafeKey(ch.KeyPrefix(), string(purpose), target)]
	if !ok {
		t.Fatalf("验证码未存储: channel=%s purpose=%s target=%s", ch.KeyPrefix(), purpose, target)
	}
	var v struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("验证码存储值解析失败: %s", raw)
	}
	return v.Code
}

// =====================================================
// 测试
// =====================================================

// TestCodeAuth_EmailRegisterLogin 邮箱通道：send-code → register → login 全流程。
func TestCodeAuth_EmailRegisterLogin(t *testing.T) {
	r, store, emailCh, _ := newCodeAuthTestRouter(t)

	// 1. send-code
	w := codeAuthRequest(r, http.MethodPost, "/api/auth/email/send-code",
		map[string]interface{}{"email": "a@example.com", "purpose": "register"}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("send-code 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "请查收邮箱") {
		t.Errorf("发送文案不符: %s", w.Body.String())
	}

	// 2. register（用 store 中的真实验证码）
	code := extractStoredCode(t, store, emailCh, service.CodePurposeRegister, "a@example.com")
	w = codeAuthRequest(r, http.MethodPost, "/api/auth/email/register",
		map[string]interface{}{"email": "a@example.com", "code": code, "nickname": "测试学员", "password": "pass123456"}, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("register 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"token"`) {
		t.Errorf("register 应返回 token: %s", w.Body.String())
	}

	// 3. login
	w = codeAuthRequest(r, http.MethodPost, "/api/auth/email/send-code",
		map[string]interface{}{"email": "a@example.com", "purpose": "login"}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("login send-code 状态码 = %d", w.Code)
	}
	code = extractStoredCode(t, store, emailCh, service.CodePurposeLogin, "a@example.com")
	w = codeAuthRequest(r, http.MethodPost, "/api/auth/email/login",
		map[string]interface{}{"email": "a@example.com", "code": code}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("login 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"token"`) {
		t.Errorf("login 应返回 token: %s", w.Body.String())
	}
}

// TestCodeAuth_PhoneRegisterLogin 手机通道：同一骨架另一 adapter，行为一致。
func TestCodeAuth_PhoneRegisterLogin(t *testing.T) {
	r, store, _, phoneCh := newCodeAuthTestRouter(t)

	w := codeAuthRequest(r, http.MethodPost, "/api/auth/phone/send-code",
		map[string]interface{}{"phone": "13800138000", "purpose": "register"}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("send-code 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "请查收手机短信") {
		t.Errorf("发送文案不符: %s", w.Body.String())
	}

	code := extractStoredCode(t, store, phoneCh, service.CodePurposeRegister, "13800138000")
	w = codeAuthRequest(r, http.MethodPost, "/api/auth/phone/register",
		map[string]interface{}{"phone": "13800138000", "code": code, "nickname": "手机学员", "password": "pass123456"}, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("register 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}

	w = codeAuthRequest(r, http.MethodPost, "/api/auth/phone/send-code",
		map[string]interface{}{"phone": "13800138000", "purpose": "login"}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("login send-code 状态码 = %d", w.Code)
	}
	code = extractStoredCode(t, store, phoneCh, service.CodePurposeLogin, "13800138000")
	w = codeAuthRequest(r, http.MethodPost, "/api/auth/phone/login",
		map[string]interface{}{"phone": "13800138000", "code": code}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("login 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
}

// TestCodeAuth_PhoneResetPassword 手机号忘记密码：发码→重置→新密码可登录。
func TestCodeAuth_PhoneResetPassword(t *testing.T) {
	r, store, _, phoneCh := newCodeAuthTestRouter(t)
	phone := "13800138001"

	// 注册手机号账号
	w := codeAuthRequest(r, http.MethodPost, "/api/auth/phone/send-code",
		map[string]interface{}{"phone": phone, "purpose": "register"}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("register send-code 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	code := extractStoredCode(t, store, phoneCh, service.CodePurposeRegister, phone)
	w = codeAuthRequest(r, http.MethodPost, "/api/auth/phone/register",
		map[string]interface{}{"phone": phone, "code": code, "nickname": "找回学员", "password": "oldpass123"}, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("register 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}

	// 未注册手机号找回 → 400
	w = codeAuthRequest(r, http.MethodPost, "/api/auth/phone/send-code",
		map[string]interface{}{"phone": "13700000000", "purpose": "reset_password"}, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("未注册找回应 400\nbody=%s", w.Body.String())
	}

	// 发找回密码验证码
	w = codeAuthRequest(r, http.MethodPost, "/api/auth/phone/send-code",
		map[string]interface{}{"phone": phone, "purpose": "reset_password"}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("reset send-code 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	code = extractStoredCode(t, store, phoneCh, service.CodePurposeResetPassword, phone)

	// 错误验证码 → 400
	w = codeAuthRequest(r, http.MethodPost, "/api/auth/phone/reset-password",
		map[string]interface{}{"phone": phone, "code": "000000", "password": "newpass123"}, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("错误验证码应 400\nbody=%s", w.Body.String())
	}

	// 正确验证码 + 合法密码 → 200
	w = codeAuthRequest(r, http.MethodPost, "/api/auth/phone/reset-password",
		map[string]interface{}{"phone": phone, "code": code, "password": "newpass123"}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("reset-password 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
}

// TestCodeAuth_InvalidPurpose 非法 purpose 返回 400（purpose 校验行为不变）。
func TestCodeAuth_InvalidPurpose(t *testing.T) {
	r, _, _, _ := newCodeAuthTestRouter(t)
	w := codeAuthRequest(r, http.MethodPost, "/api/auth/email/send-code",
		map[string]interface{}{"email": "a@example.com", "purpose": "whatever"}, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法 purpose 状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "purpose 必须为 register、login 或 reset_password") {
		t.Errorf("purpose 错误文案不符: %s", w.Body.String())
	}
}

// TestCodeAuth_ProfileBind 个人信息绑定：send-code channel 分发 + email/phone 绑定。
func TestCodeAuth_ProfileBind(t *testing.T) {
	r, store, emailCh, phoneCh := newCodeAuthTestRouter(t)

	// 先注册一个用户拿 token
	if w := codeAuthRequest(r, http.MethodPost, "/api/auth/email/send-code",
		map[string]interface{}{"email": "bind@example.com", "purpose": "register"}, ""); w.Code != http.StatusOK {
		t.Fatalf("send-code 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	code := extractStoredCode(t, store, emailCh, service.CodePurposeRegister, "bind@example.com")
	w := codeAuthRequest(r, http.MethodPost, "/api/auth/email/register",
		map[string]interface{}{"email": "bind@example.com", "code": code, "nickname": "绑定学员", "password": "pass123456"}, "")
	token := extractToken(t, w)

	// channel=phone → 手机通道发送
	w = codeAuthRequest(r, http.MethodPost, "/api/auth/profile/send-code",
		map[string]interface{}{"channel": "phone", "target": "13900139000"}, token)
	if w.Code != http.StatusOK {
		t.Fatalf("profile send-code 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	phoneCode := extractStoredCode(t, store, phoneCh, service.CodePurposeBind, "13900139000")

	// 非法 channel → 400
	w = codeAuthRequest(r, http.MethodPost, "/api/auth/profile/send-code",
		map[string]interface{}{"channel": "fax", "target": "x"}, token)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法 channel 状态码 = %d, 期望 400", w.Code)
	}

	// 绑定手机号
	w = codeAuthRequest(r, http.MethodPost, "/api/auth/profile/phone",
		map[string]interface{}{"phone": "13900139000", "code": phoneCode}, token)
	if w.Code != http.StatusOK {
		t.Fatalf("绑定手机状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "手机号修改成功") {
		t.Errorf("绑定文案不符: %s", w.Body.String())
	}
}

func extractToken(t *testing.T, w *httptest.ResponseRecorder) string {
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
