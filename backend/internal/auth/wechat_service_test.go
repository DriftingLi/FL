// Package auth 微信登录服务测试。
// code2session 外呼用 httptest server 注入 apiBase 模拟（同包测试可直接改私有字段）。
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	observer "go.uber.org/zap/zaptest/observer"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/core"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// newWxSvcOnDB 在**既有库**上装一个注入 mock code2session 端点的小程序登录服务。
// 允许传库与 logger 是为了两类用例：跨服务链路（小程序回填 unionid ⇒ App 按 unionid 认人）、
// 以及「外呼失败的日志里不许出现凭据」那条需要观察日志输出的回归锁。
func newWxSvcOnDB(t *testing.T, db *gorm.DB, openID, unionID string, errCode int, lastQuery *url.Values, logger *zap.Logger) *WechatService {
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

	authSvc := NewService(db, security.NewSession(testJWTSecret, time.Hour, security.CookieConfig{}), core.NewForumCounter(),
		"admin123", "tutor123", "student123", logger)
	svc := NewWechatService(config.WechatAppConfig{AppID: "wx-appid", AppSecret: "wx-secret"}, db, authSvc, logger)
	svc.apiBase = ts.URL
	return svc
}

// newWxSvc 自建内存库的常用形态（连带返回 db 供落库断言）。
func newWxSvc(t *testing.T, openID, unionID string, errCode int, lastQuery *url.Values) (*WechatService, *gorm.DB) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	return newWxSvcOnDB(t, db, openID, unionID, errCode, lastQuery, zap.NewNop()), db
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
	authSvc := NewService(db, security.NewSession(testJWTSecret, time.Hour, security.CookieConfig{}), core.NewForumCounter(),
		"admin123", "tutor123", "student123", zap.NewNop())
	svc := NewWechatService(config.WechatAppConfig{}, db, authSvc, zap.NewNop())
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
	if res.Role != core.HrwaiRole {
		t.Fatalf("角色应为 %s, got %s", core.HrwaiRole, res.Role)
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
	if info.Enabled != false {
		t.Errorf("未配置授权时应 enabled=false: %+v", info)
	}
}

func TestWechatLogin_NotImplemented(t *testing.T) {
	svc, _ := newWxSvc(t, "", "", 0, nil)
	if _, err := svc.LoginWithQRCode("code"); err == nil {
		t.Error("扫码登录占位应报错")
	}
}

// --- unionid 回填（#1482：App 端按 unionid 认人的前提）---

// TestWechatMiniProgramLogin_BackfillsMissingUnionID 覆盖的是存量形状：账号在开放平台绑定之前
// 注册，wechat_unionid 是空串。绑定之后 code2session 开始返回 unionid，登录时必须把它补回那行——
// 不补，App 端那条「先按 unionid 认人」的链路对老用户永远命不中，结果是给他们各开一个账号。
func TestWechatMiniProgramLogin_BackfillsMissingUnionID(t *testing.T) {
	const openID = "oBACKFILL_target"
	db := testutil.NewMemoryDB(t)
	if err := db.Create(&model.HrwaiUser{UID: 7401, Account: "wx_before_bind", Username: "绑定前注册",
		WechatOpenID: openID, Status: 1, CreatedAt: time.Now()}).Error; err != nil {
		t.Fatalf("预插失败: %v", err)
	}
	svc := newWxSvcOnDB(t, db, openID, "uni-now-available", 0, nil, zap.NewNop())

	res, err := svc.MiniProgramLogin(context.Background(), "js-code")
	if err != nil {
		t.Fatalf("老账号登录失败: %v", err)
	}
	if res.IsNew {
		t.Fatal("已有 openid 不该建新号")
	}
	var back model.HrwaiUser
	if err := db.Where("wechat_openid = ?", openID).First(&back).Error; err != nil {
		t.Fatalf("回查失败: %v", err)
	}
	if back.WechatUnionID != "uni-now-available" {
		t.Fatalf("应把换取到的 unionid 回填进空位: got %q", back.WechatUnionID)
	}
}

// TestWechatMiniProgramLogin_DoesNotOverwriteExistingUnionID 是上一道的另一半：
// 「只补空位」不等于「覆盖已有值」——已有值要么是同一人（那无需改），要么来源不同（那更不能改）。
// 把 backfillUnionID 的 `user.WechatUnionID != ""` 判空去掉，这条必红。
func TestWechatMiniProgramLogin_DoesNotOverwriteExistingUnionID(t *testing.T) {
	const openID = "oBACKFILL_keep"
	db := testutil.NewMemoryDB(t)
	if err := db.Create(&model.HrwaiUser{UID: 7402, Account: "wx_already_bound", Username: "已有unionid",
		WechatOpenID: openID, WechatUnionID: "uni-existing", Status: 1, CreatedAt: time.Now()}).Error; err != nil {
		t.Fatalf("预插失败: %v", err)
	}
	svc := newWxSvcOnDB(t, db, openID, "uni-incoming", 0, nil, zap.NewNop())

	if _, err := svc.MiniProgramLogin(context.Background(), "js-code"); err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	var back model.HrwaiUser
	if err := db.Where("wechat_openid = ?", openID).First(&back).Error; err != nil {
		t.Fatalf("回查失败: %v", err)
	}
	if back.WechatUnionID != "uni-existing" {
		t.Fatalf("已有 unionid 不该被覆盖: got %q", back.WechatUnionID)
	}
}

// TestWechatMiniProgramLogin_FailureLogCarriesNoSecret 与 App 那条同判据：
// 传输层失败时 *url.Error 的字符串含完整请求 URL，而 code2session 的查询串里就带 secret=。
// 把 wechat_service.go 的 zap.Error(outboundCause(err)) 改回 zap.Error(err) 即红。
func TestWechatMiniProgramLogin_FailureLogCarriesNoSecret(t *testing.T) {
	zapCore, logs := observer.New(zap.WarnLevel)
	db := testutil.NewMemoryDB(t)
	svc := newWxSvcOnDB(t, db, "oANY", "", 0, nil, zap.New(zapCore))
	svc.apiBase = "http://127.0.0.1:1/unreachable"

	if _, err := svc.MiniProgramLogin(context.Background(), "code"); err == nil {
		t.Fatal("不可达端点应报错")
	}
	entries := logs.All()
	if len(entries) == 0 {
		t.Fatal("外呼失败一条日志都没写——本用例就成了空断言")
	}
	for _, e := range entries {
		line := e.Message + fmt.Sprintf("%v", e.ContextMap())
		if strings.Contains(line, "wx-secret") {
			t.Fatalf("日志泄漏小程序 AppSecret: %s", line)
		}
	}
}
