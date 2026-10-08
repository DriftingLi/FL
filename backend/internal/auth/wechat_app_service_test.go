// Package auth App 端微信登录（#1482）服务测试。
// /sns/oauth2/access_token 外呼用 httptest server 注入 accessTokenBase 模拟（同包测试可直接改私有字段）。
// 每条判据都成对取证：既有「改坏了会红」的断言，也有「这条分支本身在测」的负样本。
package auth

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
	"forklift-training/internal/core"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// newAppSvc 构建注入 mock 换取端点的 App 端微信登录服务。
// resp 是端点返回的 JSON 字段集；lastQuery 记录最近一次请求参数，calls 记录外呼次数（可为 nil）。
func newAppSvc(t *testing.T, cfg config.WechatAppConfig, resp map[string]any, lastQuery *url.Values, calls *int) (*WechatAppService, *gorm.DB) {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls != nil {
			*calls++
		}
		if lastQuery != nil {
			*lastQuery = r.URL.Query()
		}
		w.Header().Set("Content-Type", "application/json")
		body, _ := json.Marshal(resp)
		_, _ = w.Write(body)
	}))
	t.Cleanup(ts.Close)

	db := testutil.NewMemoryDB(t)
	authSvc := NewService(db, security.NewSession(testJWTSecret, time.Hour, security.CookieConfig{}), core.NewForumCounter(),
		"admin123", "tutor123", "student123", zap.NewNop())
	svc := NewWechatAppService(cfg, db, authSvc, zap.NewNop())
	svc.accessTokenBase = ts.URL
	return svc, db
}

// configuredAppCfg 是「凭证已配好」的默认样本；不配置的用例显式传零值。
func configuredAppCfg() config.WechatAppConfig {
	return config.WechatAppConfig{AppID: "wx-mobile-appid", AppSecret: "wx-mobile-secret"}
}

func appOK(openID, unionID string) map[string]any {
	return map[string]any{
		"access_token":  "at-mock",
		"expires_in":    7200,
		"refresh_token": "rt-mock",
		"openid":        openID,
		"scope":         "snsapi_userinfo",
		"unionid":       unionID,
	}
}

// --- 入参与配置门禁 ---

func TestWechatAppLogin_MissingCode(t *testing.T) {
	calls := 0
	svc, _ := newAppSvc(t, configuredAppCfg(), appOK("oAPP", "uAPP"), nil, &calls)
	if _, err := svc.AppLogin(context.Background(), "  "); err == nil {
		t.Fatal("空 code 应报错")
	}
	if calls != 0 {
		t.Fatalf("空 code 不该发起外呼, calls=%d", calls)
	}
}

// TestWechatAppLogin_NotConfigured 未配置凭证时必须**先拦再呼**：
// 断言外呼次数为 0，才挡住「把 gate 挪到换取之后」这种走偏（那种写法会真的把 AppID 空串发出去）。
func TestWechatAppLogin_NotConfigured(t *testing.T) {
	calls := 0
	svc, _ := newAppSvc(t, config.WechatAppConfig{}, appOK("oAPP", ""), nil, &calls)
	_, err := svc.AppLogin(context.Background(), "the-code")
	if err == nil || !strings.Contains(err.Error(), "未配置") {
		t.Fatalf("未配置移动应用凭证应明确报错, got: %v", err)
	}
	if calls != 0 {
		t.Fatalf("未配置凭证不该发起外呼, calls=%d", calls)
	}
}

// TestWechatAppLogin_RequestContract 钉住移动应用端点的参数形状：
// appid/secret/code/grant_type，且**没有** js_code——js_code 是小程序 code2session 的参数名，
// 出现它就把「两条链路可互换」这个被文档判死的假设写进了代码。
func TestWechatAppLogin_RequestContract(t *testing.T) {
	var q url.Values
	svc, _ := newAppSvc(t, configuredAppCfg(), appOK("oAPP_123456789", ""), &q, nil)
	if _, err := svc.AppLogin(context.Background(), "the-app-code"); err != nil {
		t.Fatalf("换取失败: %v", err)
	}
	if q.Get("appid") != "wx-mobile-appid" || q.Get("secret") != "wx-mobile-secret" ||
		q.Get("code") != "the-app-code" || q.Get("grant_type") != "authorization_code" {
		t.Fatalf("oauth2/access_token 请求参数不符契约: %+v", q)
	}
	if q.Get("js_code") != "" {
		t.Fatal("移动应用换取不得带 js_code（那是小程序 code2session 的参数）")
	}
}

// --- 建号与身份定位 ---

func TestWechatAppLogin_NewUser(t *testing.T) {
	const openID = "oAPP_123456789"
	svc, db := newAppSvc(t, configuredAppCfg(), appOK(openID, "uni-app-1"), nil, nil)

	res, err := svc.AppLogin(context.Background(), "code-1")
	if err != nil {
		t.Fatalf("新用户登录失败: %v", err)
	}
	if !res.IsNew {
		t.Fatal("首次出现的 App openid 应 isNew=true")
	}
	if res.Token == "" || res.RefreshToken == "" {
		t.Fatal("应签发双令牌（与小程序登录同构）")
	}
	if res.Role != core.HrwaiRole {
		t.Fatalf("角色应为 %s, got %s", core.HrwaiRole, res.Role)
	}
	if want := "wx_" + openID[:12]; res.Account != want {
		t.Fatalf("account 派生应与小程序同源: want %s got %s", want, res.Account)
	}
	var u model.HrwaiUser
	if err := db.Where("wechat_openid = ?", openID).First(&u).Error; err != nil {
		t.Fatalf("自动建号应落库: %v", err)
	}
	if u.WechatUnionID != "uni-app-1" {
		t.Fatalf("unionid 应落库: got %q", u.WechatUnionID)
	}
}

// TestWechatAppLogin_UnionIDReusesMiniProgramAccount 是 #1482 决策 2 的落地判据：
// 同一自然人已在小程序端建过号（库里那行的 openid 是小程序的），App 端换取到的是**另一个** openid，
// 但 unionid 相同 ⇒ 必须复用那个账号，且不把库里的 openid 覆盖成 App 的。
func TestWechatAppLogin_UnionIDReusesMiniProgramAccount(t *testing.T) {
	const mpOpenID = "oMINI_program_side_1"
	const appOpenID = "oAPP_mobile_side_9"
	svc, db := newAppSvc(t, configuredAppCfg(), appOK(appOpenID, "uni-shared"), nil, nil)

	// 预插小程序账号（unionid 由小程序 code2session 在绑定开放平台后写入）
	mp := model.HrwaiUser{UID: 7001, Account: "wx_mini_existing", Username: "小程序那侧",
		WechatOpenID: mpOpenID, WechatUnionID: "uni-shared", Status: 1, CreatedAt: time.Now()}
	if err := db.Create(&mp).Error; err != nil {
		t.Fatalf("预插小程序账号失败: %v", err)
	}

	res, err := svc.AppLogin(context.Background(), "code-app")
	if err != nil {
		t.Fatalf("App 端登录失败: %v", err)
	}
	if res.IsNew {
		t.Fatal("unionid 命中已有账号时不该建新号")
	}
	if res.UserID != mp.ID {
		t.Fatalf("应复用小程序那行的账号: want id=%d got id=%d", mp.ID, res.UserID)
	}
	var count int64
	db.Model(&model.HrwaiUser{}).Count(&count)
	if count != 1 {
		t.Fatalf("unionid 同人时不该多出一行: count=%d", count)
	}
	var back model.HrwaiUser
	if err := db.First(&back, mp.ID).Error; err != nil {
		t.Fatalf("回查失败: %v", err)
	}
	if back.WechatOpenID != mpOpenID {
		t.Fatalf("库里 openid 不该被 App 那侧覆盖: want %q got %q", mpOpenID, back.WechatOpenID)
	}
}

// TestWechatAppLogin_NoUnionIDDegrades 取的是票面那条「三条件缺一即退化」的另一支：
// 换取响应没有 unionid（移动应用与小程序不在同一开放平台账号下 / 未授权 userinfo）⇒ 独立建号。
// 删掉 resolveAccount 里的 `unionID != ""` 判空，这条会连带上一道用例一起走偏，故它不是冗余样本。
func TestWechatAppLogin_NoUnionIDDegrades(t *testing.T) {
	const mpOpenID = "oMINI_no_union_1"
	const appOpenID = "oAPP_no_union_2"
	svc, db := newAppSvc(t, configuredAppCfg(), appOK(appOpenID, ""), nil, nil)

	if err := db.Create(&model.HrwaiUser{UID: 7002, Account: "wx_mini_nounion", Username: "无unionid那侧",
		WechatOpenID: mpOpenID, Status: 1, CreatedAt: time.Now()}).Error; err != nil {
		t.Fatalf("预插小程序账号失败: %v", err)
	}

	res, err := svc.AppLogin(context.Background(), "code-app")
	if err != nil {
		t.Fatalf("无 unionid 时应按 App openid 独立建号: %v", err)
	}
	if !res.IsNew {
		t.Fatal("退化臂应建新账号（isNew=true）")
	}
	var count int64
	db.Model(&model.HrwaiUser{}).Count(&count)
	if count != 2 {
		t.Fatalf("退化臂应留下两行独立账号: count=%d", count)
	}
}

// TestWechatAppLogin_SameOpenIDIsIdempotent 同 App openid 重复登录不重复建号。
func TestWechatAppLogin_SameOpenIDIsIdempotent(t *testing.T) {
	svc, db := newAppSvc(t, configuredAppCfg(), appOK("oAPP_same_1", ""), nil, nil)
	first, err := svc.AppLogin(context.Background(), "code-1")
	if err != nil {
		t.Fatalf("首次登录失败: %v", err)
	}
	second, err := svc.AppLogin(context.Background(), "code-2")
	if err != nil {
		t.Fatalf("二次登录失败: %v", err)
	}
	if second.IsNew {
		t.Fatal("老用户 isNew 应为 false")
	}
	if second.UserID != first.UserID {
		t.Fatalf("同 openid 应命中同一账号: %d vs %d", first.UserID, second.UserID)
	}
	var count int64
	db.Model(&model.HrwaiUser{}).Where("wechat_openid = ?", "oAPP_same_1").Count(&count)
	if count != 1 {
		t.Fatalf("重复登录不该重复建号: count=%d", count)
	}
}

// TestWechatAppLogin_DuplicateUnionIDPicksOldest 钉住重复 unionid 下的定序口径（id 最小者）。
// 这也是 000041 迁移「为什么不上唯一约束」的行为前提：删掉 Order("id ASC") 这条就失去确定性。
func TestWechatAppLogin_DuplicateUnionIDPicksOldest(t *testing.T) {
	svc, db := newAppSvc(t, configuredAppCfg(), appOK("oAPP_dup", "uni-dup"), nil, nil)
	old := model.HrwaiUser{UID: 7101, Account: "wx_dup_old", Username: "最早那个",
		WechatOpenID: "oMINI_dup_old", WechatUnionID: "uni-dup", Status: 1, CreatedAt: time.Now()}
	later := model.HrwaiUser{UID: 7102, Account: "wx_dup_new", Username: "后来那个",
		WechatOpenID: "oMINI_dup_new", WechatUnionID: "uni-dup", Status: 1, CreatedAt: time.Now()}
	if err := db.Create(&old).Error; err != nil {
		t.Fatalf("预插失败: %v", err)
	}
	if err := db.Create(&later).Error; err != nil {
		t.Fatalf("预插失败: %v", err)
	}
	res, err := svc.AppLogin(context.Background(), "code")
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if res.UserID != old.ID {
		t.Fatalf("重复 unionid 应认 id 最小者: want %d got %d", old.ID, res.UserID)
	}
}

func TestWechatAppLogin_DisabledUser(t *testing.T) {
	const openID = "oAPP_disabled"
	svc, db := newAppSvc(t, configuredAppCfg(), appOK(openID, ""), nil, nil)
	if err := db.Create(&model.HrwaiUser{UID: 7201, Account: "acct_wx_app_disabled", Username: "被禁App用户",
		WechatOpenID: openID, CreatedAt: time.Now()}).Error; err != nil {
		t.Fatalf("预插失败: %v", err)
	}
	if err := db.Model(&model.HrwaiUser{}).Where("wechat_openid = ?", openID).Update("status", 0).Error; err != nil {
		t.Fatalf("显式置禁用失败: %v", err)
	}
	if _, err := svc.AppLogin(context.Background(), "code"); err == nil || !strings.Contains(err.Error(), "禁用") {
		t.Fatalf("禁用用户应被登录骨架拦截: got %v", err)
	}
}

// --- 换取端点的错误面（一手依据：开放平台《移动应用 / 微信登录 / 开发流程》）---

func TestWechatAppLogin_BadCode(t *testing.T) {
	svc, _ := newAppSvc(t, configuredAppCfg(), map[string]any{"errcode": 40029, "errmsg": "invalid code"}, nil, nil)
	_, err := svc.AppLogin(context.Background(), "bad")
	if err == nil || !strings.Contains(err.Error(), "失效") {
		t.Fatalf("40029 应提示凭证失效: got %v", err)
	}
}

func TestWechatAppLogin_NotLaunchedQuota(t *testing.T) {
	svc, _ := newAppSvc(t, configuredAppCfg(), map[string]any{"errcode": 10060, "errmsg": "system error"}, nil, nil)
	_, err := svc.AppLogin(context.Background(), "code")
	if err == nil || !strings.Contains(err.Error(), "上限") {
		t.Fatalf("10060（未上架应用 100 次/天）应给次数上限文案: got %v", err)
	}
}

// TestWechatAppLogin_UnknownErrCodeIsNotSilentlyMappedToBadCode 未登记的错误码只能走通用文案：
// 把 42001/40163 这类没有一手依据的码顺手翻译成「已失效」，就是在代码里冒充文档结论。
func TestWechatAppLogin_UnknownErrCodeIsNotSilentlyMapped(t *testing.T) {
	svc, _ := newAppSvc(t, configuredAppCfg(), map[string]any{"errcode": 42001, "errmsg": "code timeout"}, nil, nil)
	_, err := svc.AppLogin(context.Background(), "code")
	if err == nil || !strings.Contains(err.Error(), "42001") || strings.Contains(err.Error(), "失效") {
		t.Fatalf("未知错误码应透传码值、不套「凭证失效」文案: got %v", err)
	}
}

func TestWechatAppLogin_MissingOpenID(t *testing.T) {
	svc, _ := newAppSvc(t, configuredAppCfg(), map[string]any{"access_token": "at", "errcode": 0}, nil, nil)
	if _, err := svc.AppLogin(context.Background(), "code"); err == nil || !strings.Contains(err.Error(), "openid") {
		t.Fatalf("errcode=0 但缺 openid 应报错: got %v", err)
	}
}

func TestWechatAppLogin_UnreachableEndpoint(t *testing.T) {
	svc, _ := newAppSvc(t, configuredAppCfg(), appOK("oX", ""), nil, nil)
	svc.accessTokenBase = "http://127.0.0.1:1/unreachable"
	if _, err := svc.AppLogin(context.Background(), "code"); err == nil {
		t.Fatal("外呼失败应报服务不可用")
	}
}

// --- 凭证面隔离：移动应用那对不被小程序链路复用 ---

// TestWechatCredentialSetsAreDistinctFields 锁住「两套凭证各住各的字段」这条配置面判据：
// 一旦有人把 Mobile 与 MiniProgram 指到同一对键，App 端就会拿小程序凭证去调移动应用端点，
// 而那正是票面与 CONTEXT.md 判死的混用。
func TestWechatCredentialSetsAreDistinctFields(t *testing.T) {
	c := config.WechatConfig{
		MiniProgram: config.WechatAppConfig{AppID: "mp-id", AppSecret: "mp-secret"},
		Mobile:      config.WechatAppConfig{AppID: "mobile-id", AppSecret: "mobile-secret"},
	}
	if c.MiniProgram.AppID == c.Mobile.AppID {
		t.Fatal("小程序与移动应用凭证必须是两组独立配置")
	}
	if (c.Mobile == config.WechatAppConfig{}) {
		t.Fatal("WechatConfig 丢了 Mobile 字段：App 端换取链会拿零值凭证出门")
	}
}
