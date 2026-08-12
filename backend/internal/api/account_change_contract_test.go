// Ticket #137（T2 修改账号）契约测试：
// PUT /api/auth/account 行为锁定——无登录态被拒、验证码错误被拒、格式非法被拒、
// 账号被占用被拒、成功后 account 变更且可用新账号+密码登录。
package api

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/cache"
	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

// newAccountChangeTestRouter 装配手机注册 + profile 绑定 + 修改账号路由（内存库 + 内存验证码存储）。
func newAccountChangeTestRouter(t *testing.T) (*gin.Engine, *memCodeStore, *fakeChannel, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	authSvc := service.NewAuthService(db, security.NewSession("test-secret", time.Hour, security.CookieConfig{}), "admin", "tutor", "student", zap.NewNop())
	store := newMemCodeStore()
	codeSvc := service.NewVerifyCodeService(db, authSvc, 5*time.Minute, store, zap.NewNop())

	phoneCh := &fakeChannel{column: "phone", keyPref: "phone_code", noun: "手机号"}
	emailCh := &fakeChannel{column: "email", keyPref: "email_code", noun: "邮箱"}

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
	// /auth/login（新账号+密码登录断言用）
	authH := NewAuthHandler(security.SessionFromConfig(cfg), authSvc, nil, nil, nil, zap.NewNop())
	auth := api.Group("/auth")
	auth.POST("/login", authH.Login)
	RegisterEmailAuthRoutes(api, deps.Session, deps.CodeSvc, deps.EmailCh)
	RegisterPhoneAuthRoutes(api, deps.Session, deps.CodeSvc, deps.PhoneCh)
	RegisterProfileBindRoutes(api, deps.Session, deps.AuthSvc, deps.CodeSvc, deps.EmailCh, deps.PhoneCh)

	return r, store, phoneCh, db
}

// TestAuthAccountChange_NoAuthRejected 无登录态调用发送验证码/修改账号均被拒。
func TestAuthAccountChange_NoAuthRejected(t *testing.T) {
	r, _, _, _ := newAccountChangeTestRouter(t)

	for _, tc := range []struct {
		method, path string
	}{
		{http.MethodPost, "/api/auth/account/send-code"},
		{http.MethodPut, "/api/auth/account"},
	} {
		w := codeAuthRequest(r, tc.method, tc.path, map[string]interface{}{"account": "new_acct"}, "")
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s 无登录态状态码 = %d, 期望 401", tc.method, tc.path, w.Code)
		}
	}
}

// TestAuthAccountChange_FullFlow 端到端：注册（绑定手机号）→ 发送验证码 → 各拒绝分支 → 成功 → 新账号登录。
func TestAuthAccountChange_FullFlow(t *testing.T) {
	r, store, phoneCh, db := newAccountChangeTestRouter(t)

	// 1. 手机验证码注册（绑定手机号 + 密码）
	if w := codeAuthRequest(r, http.MethodPost, "/api/auth/phone/send-code",
		map[string]interface{}{"phone": "13800138000", "purpose": "register"}, ""); w.Code != http.StatusOK {
		t.Fatalf("send-code 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	regCode := extractStoredCode(t, store, phoneCh, service.CodePurposeRegister, "13800138000")
	w := codeAuthRequest(r, http.MethodPost, "/api/auth/phone/register",
		map[string]interface{}{"phone": "13800138000", "code": regCode, "nickname": "改号学员", "password": "pass123456"}, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("register 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	token := extractToken(t, w)

	// 2. 发送修改账号验证码（发往已绑定手机号）
	w = codeAuthRequest(r, http.MethodPost, "/api/auth/account/send-code", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("send-code 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	code := extractStoredCode(t, store, phoneCh, service.CodePurposeAccountChange, "13800138000")

	// 3. 验证码错误被拒
	w = codeAuthRequest(r, http.MethodPut, "/api/auth/account",
		map[string]interface{}{"account": "new_acct_1", "code": "000000"}, token)
	if w.Code != http.StatusBadRequest || !bytes.Contains(w.Body.Bytes(), []byte("验证码错误")) {
		t.Errorf("验证码错误应被拒: 状态码=%d body=%s", w.Code, w.Body.String())
	}

	// 4. 格式非法被拒（<4 位）
	w = codeAuthRequest(r, http.MethodPut, "/api/auth/account",
		map[string]interface{}{"account": "ab", "code": code}, token)
	if w.Code != http.StatusBadRequest || !bytes.Contains(w.Body.Bytes(), []byte("4-20 位")) {
		t.Errorf("格式非法应被拒: 状态码=%d body=%s", w.Code, w.Body.String())
	}

	// 5. 账号已被占用被拒（另一账号持有 new_acct_1）
	seedOccupiedAccount(t, db, "new_acct_1")
	w = codeAuthRequest(r, http.MethodPut, "/api/auth/account",
		map[string]interface{}{"account": "new_acct_1", "code": code}, token)
	if w.Code != http.StatusBadRequest || !bytes.Contains(w.Body.Bytes(), []byte("已被占用")) {
		t.Errorf("账号占用应被拒: 状态码=%d body=%s", w.Code, w.Body.String())
	}

	// 5b. 验证码校验成功后即失效（被占用分支已消费），成功分支需重新获取；
	// 先清掉发送节流 key（真实场景等待 60s 即可）
	_ = store.Del(context.Background(), cache.SafeKey("phone_code_send", "account_change", "13800138000"))
	if w := codeAuthRequest(r, http.MethodPost, "/api/auth/account/send-code", nil, token); w.Code != http.StatusOK {
		t.Fatalf("重发 send-code 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	code = extractStoredCode(t, store, phoneCh, service.CodePurposeAccountChange, "13800138000")

	// 6. 成功：账号变更为 new_acct_2
	w = codeAuthRequest(r, http.MethodPut, "/api/auth/account",
		map[string]interface{}{"account": "new_acct_2", "code": code}, token)
	if w.Code != http.StatusOK {
		t.Fatalf("修改账号状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("账号修改成功")) {
		t.Errorf("修改成功文案不符: %s", w.Body.String())
	}

	// 7. 数据库 account 已变更为新值（/me 响应形状由 auth_me 契约测试锁定）
	var changed model.HrwaiUser
	if err := db.Where("phone = ?", "13800138000").First(&changed).Error; err != nil {
		t.Fatalf("查询用户失败: %v", err)
	}
	if changed.Account != "new_acct_2" {
		t.Errorf("account 未变更: got=%s", changed.Account)
	}

	// 8. 新账号 + 密码可登录，原手机号登录仍可用
	w = codeAuthRequest(r, http.MethodPost, "/api/auth/login",
		map[string]interface{}{"username": "new_acct_2", "password": "pass123456", "role": "hrwai_user"}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("新账号登录状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	w = codeAuthRequest(r, http.MethodPost, "/api/auth/login",
		map[string]interface{}{"username": "13800138000", "password": "pass123456", "role": "hrwai_user"}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("手机号登录状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
}

// TestAuthAccountChange_UnboundPhone 未绑定手机号（邮箱注册占位）发送验证码被拒。
func TestAuthAccountChange_UnboundPhone(t *testing.T) {
	r, store, _, _ := newAccountChangeTestRouter(t)

	// 邮箱注册：phone 为 email_ 占位值
	emailCh := &fakeChannel{column: "email", keyPref: "email_code", noun: "邮箱"}
	if w := codeAuthRequest(r, http.MethodPost, "/api/auth/email/send-code",
		map[string]interface{}{"email": "acct@example.com", "purpose": "register"}, ""); w.Code != http.StatusOK {
		t.Fatalf("send-code 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	regCode := extractStoredCode(t, store, emailCh, service.CodePurposeRegister, "acct@example.com")
	w := codeAuthRequest(r, http.MethodPost, "/api/auth/email/register",
		map[string]interface{}{"email": "acct@example.com", "code": regCode, "nickname": "邮箱学员", "password": "pass123456"}, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("register 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	token := extractToken(t, w)

	w = codeAuthRequest(r, http.MethodPost, "/api/auth/account/send-code", nil, token)
	if w.Code != http.StatusBadRequest || !bytes.Contains(w.Body.Bytes(), []byte("请先绑定手机号")) {
		t.Errorf("未绑定手机号应被拒: 状态码=%d body=%s", w.Code, w.Body.String())
	}
}

// seedOccupiedAccount 直接插入一个占用指定登录账号的用户（唯一性校验测试种子）。
func seedOccupiedAccount(t *testing.T, db *gorm.DB, account string) {
	t.Helper()
	u := &model.HrwaiUser{
		UID:       1000000000000000099,
		Account:   account,
		Username:  "占用账号用户",
		Password:  "hash",
		Phone:     "13900009999",
		Status:    1,
		CreatedAt: testutil.Now(),
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("种子占用账号失败: %v", err)
	}
}
