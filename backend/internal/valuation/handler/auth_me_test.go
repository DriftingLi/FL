package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	mainmodel "forklift-training/internal/model"
	"forklift-training/internal/security"
	vmain "forklift-training/internal/service"
	vservice "forklift-training/internal/valuation/service"
)

// fakeValuationAuth 实现 ValuationAuth 窄接口（/auth/me 测试用）。
type fakeValuationAuth struct {
	user *mainmodel.HrwaiUser
}

func (f *fakeValuationAuth) HrwaiLogin(account, password string) (*vmain.LoginResult, error) {
	return nil, nil
}

func (f *fakeValuationAuth) GetHrwaiUserByID(id int) (*mainmodel.HrwaiUser, error) {
	return f.user, nil
}

// TestValuationAuthMe_MasksEmailPlaceholderPhone 锁定 /api/valuation/auth/me
// 对 email_ 占位手机号源头过滤为空串（与主站 /auth/me 同口径）。
func TestValuationAuthMe_MasksEmailPlaceholderPhone(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sess := security.NewSession("test-secret", time.Hour, security.CookieConfig{Name: "hrwai_token"})
	authSvc := &fakeValuationAuth{
		user: &mainmodel.HrwaiUser{
			ID:       1,
			UID:      1000000000000000001,
			Account:  "acct_alice",
			Username: "alice",
			Phone:    "email_0123456789abcdef0123456789abcdef",
			Email:    "alice@example.com",
		},
	}

	r := gin.New()
	dict := newSeedMemDict()
	evalStore := newMemEvalStore()
	batteryStore := &memBatteryStore{}
	valuationSvc, err := vservice.NewValuationService(dict, evalStore)
	if err != nil {
		t.Fatalf("构造估值服务失败: %v", err)
	}
	RegisterRoutes(r, sess, zap.NewNop(), nil,
		dict, evalStore, batteryStore,
		valuationSvc, vservice.NewBatteryRULService(),
		&memReportGenerator{}, &memStorage{},
		authSvc)

	token, err := sess.Issue(1, "acct_alice", "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}
	req := httptest.NewRequest("GET", "/api/valuation/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200, got %d: %s", w.Code, w.Body.String())
	}
	_, _, data := decodeBody(t, w)
	if got := data["phone"]; got != "" {
		t.Fatalf("期望 email_ 占位手机号被过滤为空串, got %v", got)
	}
	if got := data["email"]; got != "alice@example.com" {
		t.Fatalf("邮箱字段不应受影响, got %v", got)
	}
}
