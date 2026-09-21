// 验证码重置口令 = 全会话吊销（ADR-0062 票7；CONTEXT.md「会话（session）」口令族）。
// seam：POST /api/auth/phone/reset-password（匿名、凭验证码认领账号）与 POST /api/auth/refresh
// 共用同一 Session 单例（ADR-0011），吊销标记写在真链路上才会被轮换读到。
// 修复前：重置口令只写 password 列、一句吊销都没有 ⇒ 被盗号者最常见的自救动作
// （忘记密码 → 验证码重置）不会踢掉攻击者手上的 refresh 链，而 RotateRefresh 只看令牌与
// 吊销标记、不查账号是否仍在，最长 7 天仍可静默续登——用户以为改了密码就把人踢出去了。
//
// 第二条用例锁失败策略那一半：口令族**尽力而为**（标记写不进去也不回退口令），与注销族
// 「先写标记、失败即整体不生效」（session_termination_contract_test.go）有意不同，
// 两族策略不许互相顶替。
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/captcha"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

const (
	resetPhone       = "13900000009"
	resetAccount     = "resetme"
	resetOldPassword = "oldpass123"
	resetNewPassword = "newpass123"
)

// newPasswordFamilyRouter 装配最小真链路：手机号通道重置口令 + 双令牌轮换 + 账号密码登录，
// 三者共用一个会话实例（黑名单存储由参数注入）。
func newPasswordFamilyRouter(t *testing.T, bl security.BlacklistStore) (*gin.Engine, *security.Session, *memCodeStore, *fakeChannel, int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	sess := security.NewSessionWithBlacklistAndRefresh("test-secret", time.Hour, 7*time.Hour,
		security.CookieConfig{Name: "hrwai_token"}, bl)
	authSvc := service.NewAuthService(db, sess, service.NewForumCounter(), "admin", "tutor", "student", zap.NewNop())
	store := newMemCodeStore()
	codeSvc := service.NewVerifyCodeService(db, authSvc, 5*time.Minute, store, zap.NewNop())
	phoneCh := &fakeChannel{column: "phone", keyPref: "phone_code", noun: "手机号"}

	hashed, err := service.HashPassword(resetOldPassword)
	if err != nil {
		t.Fatalf("哈希种子口令失败: %v", err)
	}
	u := model.HrwaiUser{
		UID: 910001, Account: resetAccount, Username: "被盗学员",
		Password: hashed, Phone: resetPhone, Status: 1,
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("播种学员账号失败: %v", err)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	RegisterPhoneAuthRoutes(r.Group("/api"),
		RouterDeps{Session: sess, DB: db, Logger: zap.NewNop()}, codeSvc, phoneCh, captcha.NewService(store), false)
	authH := NewAuthHandler(sess, authSvc, nil, nil, nil, zap.NewNop())
	g := r.Group("/api/auth")
	g.POST("/refresh", authH.Refresh)
	g.POST("/login", authH.Login)
	return r, sess, store, phoneCh, u.ID
}

// resetPasswordViaCode 走真实两步：找回发码 → 带码重置，返回重置这一步的 recorder。
func resetPasswordViaCode(t *testing.T, r *gin.Engine, store *memCodeStore, ch *fakeChannel, newPassword string) *httptest.ResponseRecorder {
	t.Helper()
	if w := codeAuthRequest(r, http.MethodPost, "/api/auth/phone/send-code",
		map[string]interface{}{"phone": resetPhone, "purpose": "reset_password"}, ""); w.Code != http.StatusOK {
		t.Fatalf("找回发码应 200，实际 %d\nbody=%s", w.Code, w.Body.String())
	}
	code := extractStoredCode(t, store, ch, service.CodePurposeResetPassword, resetPhone)
	return codeAuthRequest(r, http.MethodPost, "/api/auth/phone/reset-password",
		map[string]interface{}{"phone": resetPhone, "code": code, "password": newPassword}, "")
}

// postPasswordLogin 账号密码登录端点（口令是否真的生效，以能否登录为准）。
func postPasswordLogin(r *gin.Engine, account, password string) int {
	return codeAuthRequest(r, http.MethodPost, "/api/auth/login",
		map[string]interface{}{"username": account, "password": password}, "").Code
}

// 重置口令后，手上那枚 refresh（重置前一刻刚轮换出来的、本身完全有效）换不出新令牌 ⇒ 401。
func TestResetPassword_旧refresh在重置后被拒(t *testing.T) {
	r, sess, store, ch, uid := newPasswordFamilyRouter(t, newValBlacklist())

	_, staleRefresh, err := sess.IssuePair(uid, resetAccount, service.HrwaiRole)
	if err != nil {
		t.Fatalf("签发 refresh 失败: %v", err)
	}
	// 对照组：重置前可正常轮换（被拒的原因必须是吊销，不是令牌本身无效）
	code, before := doRefresh(r, staleRefresh)
	if code != http.StatusOK || before.Data.RefreshToken == "" {
		t.Fatalf("重置前应可轮换：code=%d data=%+v", code, before)
	}
	inHand := before.Data.RefreshToken

	if w := resetPasswordViaCode(t, r, store, ch, resetNewPassword); w.Code != http.StatusOK {
		t.Fatalf("重置口令应 200，实际 %d\nbody=%s", w.Code, w.Body.String())
	}

	// 吊销作用于整条链：重置前那枚与手上刚轮换出来那枚，一律被拒
	for _, tok := range []struct {
		name string
		rt   string
	}{{"重置前签发的", staleRefresh}, {"重置前轮换出的", inHand}} {
		if got, resp := doRefresh(r, tok.rt); got != http.StatusUnauthorized {
			t.Errorf("%s refresh 在重置口令后应 401，实际 %d（data=%+v）", tok.name, got, resp)
		}
	}
}

// 吊销标记写不进去 ⇒ 口令仍然生效（尽力而为族：不能为吊销失败而拒绝落口令，
// 否则 Redis 抖动时用户找不回账号）。
func TestResetPassword_吊销写失败口令仍生效(t *testing.T) {
	r, _, store, ch, _ := newPasswordFamilyRouter(t, rejectBlacklist{})

	if w := resetPasswordViaCode(t, r, store, ch, resetNewPassword); w.Code != http.StatusOK {
		t.Fatalf("吊销标记写失败时重置口令仍应 200，实际 %d\nbody=%s", w.Code, w.Body.String())
	}
	if got := postPasswordLogin(r, resetAccount, resetNewPassword); got != http.StatusOK {
		t.Errorf("重置后新密码应可登录（口令不可因吊销失败而回退），实际 %d", got)
	}
	if got := postPasswordLogin(r, resetAccount, resetOldPassword); got == http.StatusOK {
		t.Error("重置后旧密码不应仍可登录")
	}
}
