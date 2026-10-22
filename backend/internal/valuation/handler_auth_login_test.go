// 估值族登录端点（#1756 第二批）的外部行为锁：
//  1. 它签出的令牌带估值族 —— 主站入口必须拒绝它（会话隔离的落点）；
//  2. 它**不下发 Cookie** —— 令牌由 BFF 持有，浏览器侧不出现跨站 Cookie；
//  3. 验密失败一律 401 且不泄露账号是否存在（与主站同口径）。
package valuation

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/security"
)

func newValuationLoginRouter(t *testing.T, sess *security.Session) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	login := func(family security.Family, account, password string) (ValuationLoginResult, error) {
		if account != "acct_alice" || password != "pw123456" {
			return ValuationLoginResult{}, errors.New("账号或密码错误")
		}
		access, refresh, err := sess.IssuePairFor(family, 1, account, "hrwai_user")
		if err != nil {
			return ValuationLoginResult{}, err
		}
		return ValuationLoginResult{
			Token: access, RefreshToken: refresh, UserID: 1,
			Account: account, Username: "alice", Role: "hrwai_user",
		}, nil
	}
	r := gin.New()
	r.POST("/api/valuation/auth/login", NewValuationAuthHandlerWithLogin(nil, sess, login).Login)
	return r
}

func postValuationLogin(r *gin.Engine, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/valuation/auth/login", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestValuationLoginIssuesValuationFamily(t *testing.T) {
	sess := security.NewSessionWithBlacklistAndRefresh("test-secret", 2*time.Hour, 7*time.Hour,
		security.CookieConfig{Name: "hrwai_token"}, newMemBlacklistStore())
	r := newValuationLoginRouter(t, sess)

	w := postValuationLogin(r, `{"account":"acct_alice","password":"pw123456"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("登录失败：HTTP %d %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			Token        string `json:"token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应不是预期 JSON: %v", err)
	}
	if resp.Data.Token == "" || resp.Data.RefreshToken == "" {
		t.Fatal("登录成功却没有返回双令牌")
	}

	if _, err := sess.VerifyAccessFor(security.FamilyValuation, resp.Data.Token); err != nil {
		t.Errorf("签出的令牌不是估值族: %v", err)
	}
	if _, err := sess.VerifyAccessFor(security.FamilyWeb, resp.Data.Token); err == nil {
		t.Error("估值登录签出的令牌被主站入口接受了：会话隔离失效")
	}
	// 不下发 Cookie：BFF 持有会话，浏览器侧不该收到任何登录态 Cookie。
	if got := w.Header().Values("Set-Cookie"); len(got) != 0 {
		t.Errorf("估值登录不应下发 Cookie，实得 %v", got)
	}
}

func TestValuationLoginRejectsBadCredentials(t *testing.T) {
	sess := security.NewSessionWithBlacklistAndRefresh("test-secret", 2*time.Hour, 7*time.Hour,
		security.CookieConfig{Name: "hrwai_token"}, newMemBlacklistStore())
	r := newValuationLoginRouter(t, sess)

	w := postValuationLogin(r, `{"account":"acct_alice","password":"wrong"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("错误口令应 401，实得 %d", w.Code)
	}
	if bytes.Contains(w.Body.Bytes(), []byte("token")) {
		t.Error("失败响应里不该出现任何令牌字段")
	}
}
