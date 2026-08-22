// Package service 微信登录服务测试。
// code2session 外呼用 httptest server 注入 apiBase 模拟（同包测试可直接改私有字段）。
package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// newWxSvc 构建注入 mock code2session 端点的微信登录服务。
// mock 端点按入参返回 openID/unionID/errCode；lastQuery 记录最近一次请求参数供断言。
func newWxSvc(t *testing.T, openID, unionID string, errCode int, lastQuery *url.Values) (*WechatAuthService, *gorm.DB) {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if lastQuery != nil {
			*lastQuery = r.URL.Query()
		}
		w.Header().Set("Content-Type", "application/json")
		body, _ := json.Marshal(map[string]any{
			"openid":      openID,
			"session_key": "sk-mock",
			"unionid":     unionID,
			"errcode":     errCode,
			"errmsg":      "ok",
		})
		_, _ = w.Write(body)
	}))
	t.Cleanup(ts.Close)

	db := testutil.NewMemoryDB(t)
	authSvc := NewAuthService(db, security.NewSession(testJWTSecret, time.Hour, security.CookieConfig{}),
		"admin123", "tutor123", "student123", zap.NewNop())
	svc := NewWechatAuthService(config.WechatAppConfig{AppID: "wx-appid", AppSecret: "wx-secret"}, db, authSvc, zap.NewNop())
	svc.apiBase = ts.URL
	return svc, db
}

// --- 小程序登录：入参与配置校验 ---

func TestWechatMiniProgramLogin_MissingCode(t *testing.T) {
	svc, _ := newWxSvc(t, "oABC", "", 0, nil)
	if _, err := svc.MiniProgramLogin(context.Background(), "  "); err == nil {
		t.Fatal("空 code 应报错")
	}
}

func TestWechatMiniProgramLogin_NotConfigured(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	authSvc := NewAuthService(db, security.NewSession(testJWTSecret, time.Hour, security.CookieConfig{}),
		"admin123", "tutor123", "student123", zap.NewNop())
	svc := NewWechatAuthService(config.WechatAppConfig{}, db, authSvc, zap.NewNop())
	_, err := svc.MiniProgramLogin(context.Background(), "code")
	if err == nil || !strings.Contains(err.Error(), "未配置") {
		t.Fatalf("未配置 AppID/Secret 应明确报错, got: %v", err)
	}
}

// --- 小程序登录：主流程 ---

func TestWechatMiniProgramLogin_NewUser(t *testing.T) {
	const openID = "oABC_123456789"
	var q url.Values
	svc, db := newWxSvc(t, openID, "uni-1", 0, &q)

	res, err := svc.MiniProgramLogin(context.Background(), "the-js-code")
	if err != nil {
		t.Fatalf("新用户登录失败: %v", err)
	}
	// 请求参数契约：appid/secret/js_code/grant_type
	if q.Get("appid") != "wx-appid" || q.Get("secret") != "wx-secret" ||
		q.Get("js_code") != "the-js-code" || q.Get("grant_type") != "authorization_code" {
		t.Fatalf("code2session 请求参数不符契约: %+v", q)
	}
	if !res.IsNew {
		t.Fatal("新 openid 应标记 isNew=true")
	}
	if res.Token == "" || res.RefreshToken == "" {
		t.Fatal("应签发双令牌（access + refresh）")
	}
	if res.Role != HrwaiRole {
		t.Fatalf("角色应为 %s, got %s", HrwaiRole, res.Role)
	}
	if res.Name != res.Username {
		t.Fatalf("平铺契约 name 取 username: name=%s username=%s", res.Name, res.Username)
	}
	// account/username 派生：wx_+前12位 / 微信学员+后6位
	if want := "wx_" + openID[:12]; res.Account != want {
		t.Fatalf("account 派生不符: want %s got %s", want, res.Account)
	}
	if want := "微信学员" + openID[len(openID)-6:]; res.Username != want {
		t.Fatalf("username 派生不符: want %s got %s", want, res.Username)
	}
	// 落库验证：openid/unionid 绑定、状态启用
	var u model.HrwaiUser
	if err := db.Where("wechat_openid = ?", openID).First(&u).Error; err != nil {
		t.Fatalf("自动注册用户应落库: %v", err)
	}
	if u.WechatUnionID != "uni-1" {
		t.Fatalf("unionid 应落库: got %q", u.WechatUnionID)
	}
	if u.Status != 1 || u.UID == 0 {
		t.Fatalf("新用户应启用且有 uid: %+v", u)
	}
	if u.ID != res.UserID {
		t.Fatalf("返回 user_id 与落库不一致: %d vs %d", res.UserID, u.ID)
	}
}

func TestWechatMiniProgramLogin_ExistingUser(t *testing.T) {
	const openID = "oEXIST_987654"
	svc, db := newWxSvc(t, openID, "", 0, nil)

	first, err := svc.MiniProgramLogin(context.Background(), "code-1")
	if err != nil {
		t.Fatalf("首次登录失败: %v", err)
	}
	second, err := svc.MiniProgramLogin(context.Background(), "code-2")
	if err != nil {
		t.Fatalf("二次登录失败: %v", err)
	}
	if second.IsNew {
		t.Fatal("老用户 isNew 应为 false")
	}
	if second.UserID != first.UserID || second.Account != first.Account {
		t.Fatalf("同 openid 应命中同一账号: %+v vs %+v", first, second)
	}
	var count int64
	db.Model(&model.HrwaiUser{}).Where("wechat_openid = ?", openID).Count(&count)
	if count != 1 {
		t.Fatalf("同 openid 重复登录不应重复建号: count=%d", count)
	}
}

func TestWechatMiniProgramLogin_DisabledUser(t *testing.T) {
	const openID = "oDISABLED_1"
	svc, db := newWxSvc(t, openID, "", 0, nil)
	// 预插一个绑定该 openid 的用户（Status 带 gorm default:1，零值创建会被跳过）
	if err := db.Create(&model.HrwaiUser{
		UID: 99, Account: "acct_wx_disabled", Username: "被禁微信用户",
		WechatOpenID: openID, CreatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("预插用户失败: %v", err)
	}
	if err := db.Model(&model.HrwaiUser{}).Where("wechat_openid = ?", openID).Update("status", 0).Error; err != nil {
		t.Fatalf("显式置禁用失败: %v", err)
	}
	_, err := svc.MiniProgramLogin(context.Background(), "code")
	if err == nil || !strings.Contains(err.Error(), "禁用") {
		t.Fatalf("禁用用户应被登录骨架拦截: got %v", err)
	}
}

// --- 小程序登录：code2session 错误码 ---

func TestWechatMiniProgramLogin_BadCode(t *testing.T) {
	svc, _ := newWxSvc(t, "", "", 40029, nil)
	_, err := svc.MiniProgramLogin(context.Background(), "bad-code")
	if err == nil || !strings.Contains(err.Error(), "失效") {
		t.Fatalf("40029 应提示凭证失效: got %v", err)
	}
}

func TestWechatMiniProgramLogin_RateLimit(t *testing.T) {
	svc, _ := newWxSvc(t, "", "", 45011, nil)
	_, err := svc.MiniProgramLogin(context.Background(), "code")
	if err == nil || !strings.Contains(err.Error(), "频繁") {
		t.Fatalf("45011 应提示操作频繁: got %v", err)
	}
}

func TestWechatMiniProgramLogin_ServiceUnavailable(t *testing.T) {
	svc, _ := newWxSvc(t, "oX", "", 0, nil)
	svc.apiBase = "http://127.0.0.1:1/unreachable" // 指向不可达端点
	if _, err := svc.MiniProgramLogin(context.Background(), "code"); err == nil {
		t.Fatal("外呼失败应报服务不可用")
	}
}

// --- 扫码登录占位（保留原有行为） ---

func TestWechatQRCodeInfo_NotConfigured(t *testing.T) {
	svc, _ := newWxSvc(t, "", "", 0, nil)
	svc.cfg = config.WechatAppConfig{}
	info := svc.QRCodeInfo()
	if info["enabled"] != false {
		t.Errorf("未配置授权时应 enabled=false: %+v", info)
	}
}

func TestWechatLogin_NotImplemented(t *testing.T) {
	svc, _ := newWxSvc(t, "", "", 0, nil)
	if _, err := svc.LoginWithQRCode("code"); err == nil {
		t.Error("扫码登录占位应报错")
	}
}
