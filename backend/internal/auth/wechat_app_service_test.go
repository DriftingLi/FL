// Package auth App 端微信登录（#1482）服务测试。
// /sns/oauth2/access_token 外呼用 httptest server 注入 accessTokenBase 模拟（同包测试可直接改私有字段）。
// 每条判据都成对取证：既有「改坏了会红」的断言，也有「这条分支本身在测」的负样本。
package auth

import (
	"context"
	"encoding/json"
	"errors"
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

// newAppSvcOnDB 在**既有库**上装一个 App 端微信登录服务，外呼打到 mock 换取端点。
// 允许传库是为了跨服务用例（小程序登录回填 unionid ⇒ App 登录按 unionid 认出同一个账号）；
// resp 是端点返回的 JSON 字段集，lastQuery 记录最近一次请求参数，calls 记录外呼次数（均可为 nil）。
func newAppSvcOnDB(t *testing.T, db *gorm.DB, cfg config.WechatAppConfig, resp map[string]any,
	lastQuery *url.Values, calls *int, logger *zap.Logger) *WechatAppService {
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

	authSvc := NewService(db, security.NewSession(testJWTSecret, time.Hour, security.CookieConfig{}), core.NewForumCounter(),
		"admin123", "tutor123", "student123", logger)
	svc := NewWechatAppService(cfg, db, authSvc, logger)
	svc.accessTokenBase = ts.URL
	return svc
}

// newAppSvc 自建内存库的常用形态（连带返回 db 供落库断言）。
func newAppSvc(t *testing.T, cfg config.WechatAppConfig, resp map[string]any, lastQuery *url.Values, calls *int) (*WechatAppService, *gorm.DB) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	return newAppSvcOnDB(t, db, cfg, resp, lastQuery, calls, zap.NewNop()), db
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

// TestWechatAppLogin_DuplicateUnionIDPicksOldest 钉住重复 unionid 下的定序口径（认最早那个账号），
// 它是 000041「只建索引、不上唯一约束」这条决定的行为前提。
// 诚实交代射程：`First` 本身就会按主键升序（GORM v1.25.12 finisher_api.go:119），所以**单删
// 那句显式 Order 本用例不会红**——它防的是「换成 Find/limit 或改了选行规则」那类走偏，
// 不是防那一行子句被删。代码里保留显式 Order 正是为了不把确定性寄存在库实现细节上。
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

// --- 凭证面隔离那条判据已由 internal/config/wechat_keys_test.go 直接测键位映射，
// 这里不再放一份「断言自己造的 struct 字面量」的同义反复用例。

func TestWechatAppLogin_CodeAlreadyUsed(t *testing.T) {
	svc, _ := newAppSvc(t, configuredAppCfg(), map[string]any{"errcode": 40163, "errmsg": "code已使用"}, nil, nil)
	_, err := svc.AppLogin(context.Background(), "used-code")
	if err == nil || !strings.Contains(err.Error(), "已被使用") {
		t.Fatalf("40163（oauth_code已使用）应提示重新取码: got %v", err)
	}
}

// TestWechatAppLogin_BadCredentialCodes 40001（AppSecret 错）与 40013（appid 不合法）都是
// 「重试永远不会好」的配置类失败，不该给用户一句「请稍后再试」。
func TestWechatAppLogin_BadCredentialCodes(t *testing.T) {
	for _, code := range []int{40001, 40013} {
		svc, _ := newAppSvc(t, configuredAppCfg(), map[string]any{"errcode": code, "errmsg": "bad credential"}, nil, nil)
		_, err := svc.AppLogin(context.Background(), "code")
		if err == nil || !strings.Contains(err.Error(), "配置有误") {
			t.Fatalf("%d 属配置类失败，应给「配置有误」而非稍后再试: got %v", code, err)
		}
	}
}

func TestWechatAppLogin_SystemBusy(t *testing.T) {
	svc, _ := newAppSvc(t, configuredAppCfg(), map[string]any{"errcode": -1, "errmsg": "系统繁忙"}, nil, nil)
	_, err := svc.AppLogin(context.Background(), "code")
	if err == nil || !strings.Contains(err.Error(), "繁忙") {
		t.Fatalf("-1（系统繁忙）应给繁忙文案: got %v", err)
	}
}

// --- 凭据不落日志（#1482 评审发现的回归锁）---

// TestOutboundCauseStripsRequestURL 先把前提钉住：Go 的 *url.Error 确实把完整 URL 拼进
// Error()，而这两条链路的 URL 查询串里都有 AppSecret。前提不成立时本测试直接判红——
// 否则「outboundCause 去掉了 URL」会变成一句永远测不到东西的空话。
func TestOutboundCauseStripsRequestURL(t *testing.T) {
	raw := &url.Error{
		Op:  "Get",
		URL: "https://api.weixin.qq.com/sns/oauth2/access_token?secret=LEAK-CHECK",
		Err: errors.New("dial tcp: connection refused"),
	}
	if !strings.Contains(raw.Error(), "LEAK-CHECK") {
		t.Fatal("前提失效：*url.Error 不含请求 URL，本用例失去判别力")
	}
	if got := outboundCause(raw).Error(); strings.Contains(got, "LEAK-CHECK") {
		t.Fatalf("outboundCause 没去掉 URL: %s", got)
	}
	plain := errors.New("boom")
	if outboundCause(plain) != plain {
		t.Fatal("非 *url.Error 应原样返回，别把原因吞掉")
	}
}

// TestWechatAppLogin_FailureLogCarriesNoSecret 外呼失败时日志里不得出现 AppSecret。
// 把 wechat_app_service.go 的 zap.Error(outboundCause(err)) 改回 zap.Error(err) 即红。
func TestWechatAppLogin_FailureLogCarriesNoSecret(t *testing.T) {
	cfg := config.WechatAppConfig{AppID: "wx-mobile-appid", AppSecret: "MOBILE-SECRET-LEAK-CHECK"}
	zapCore, logs := observer.New(zap.WarnLevel)
	db := testutil.NewMemoryDB(t)
	authSvc := NewService(db, security.NewSession(testJWTSecret, time.Hour, security.CookieConfig{}), core.NewForumCounter(),
		"admin123", "tutor123", "student123", zap.NewNop())
	svc := NewWechatAppService(cfg, db, authSvc, zap.New(zapCore))
	svc.accessTokenBase = "http://127.0.0.1:1/unreachable"

	if _, err := svc.AppLogin(context.Background(), "code"); err == nil {
		t.Fatal("不可达端点应报错")
	}
	entries := logs.All()
	if len(entries) == 0 {
		t.Fatal("外呼失败一条日志都没写——本用例就成了空断言")
	}
	for _, e := range entries {
		line := e.Message + fmt.Sprintf("%v", e.ContextMap())
		if strings.Contains(line, "MOBILE-SECRET-LEAK-CHECK") {
			t.Fatalf("日志泄漏移动应用 AppSecret: %s", line)
		}
	}
}

// --- 跨服务：小程序登录回填 unionid ⇒ App 端认出同一个账号 ---

// TestWechatAppLogin_ReusesAccountBackfilledByMiniProgramLogin 是「同人」这一臂真正成立的判据。
// 场景是存量用户：账号在开放平台绑定**之前**注册，那行 wechat_unionid 是空串。
// 没有 backfillUnionID 这一步的话，App 端按 unionid 找不到他，会再建一个账号——
// 于是「两端同一自然人两个账号」这个退化结果恰好落在最不该落在的人身上（已在小程序里买过课的人）。
func TestWechatAppLogin_ReusesAccountBackfilledByMiniProgramLogin(t *testing.T) {
	const mpOpenID = "oMINI_pre_existing"
	const appOpenID = "oAPP_same_person"
	const sharedUnion = "uni-shared-after-binding"

	db := testutil.NewMemoryDB(t)
	// 建号时没有 unionid（绑定之前的老数据形状）
	if err := db.Create(&model.HrwaiUser{UID: 7301, Account: "wx_before_binding", Username: "绑定前注册",
		WechatOpenID: mpOpenID, Status: 1, CreatedAt: time.Now()}).Error; err != nil {
		t.Fatalf("预插老账号失败: %v", err)
	}

	// 第一步：小程序登录。此时 code2session 已能返回 unionid（绑定之后），应把它补回那行。
	mpSvc := newWxSvcOnDB(t, db, mpOpenID, sharedUnion, 0, nil, zap.NewNop())
	if _, err := mpSvc.MiniProgramLogin(context.Background(), "js-code"); err != nil {
		t.Fatalf("小程序登录失败: %v", err)
	}
	var afterMp model.HrwaiUser
	if err := db.Where("wechat_openid = ?", mpOpenID).First(&afterMp).Error; err != nil {
		t.Fatalf("回查老账号失败: %v", err)
	}
	if afterMp.WechatUnionID != sharedUnion {
		t.Fatalf("小程序登录应回填 unionid: got %q", afterMp.WechatUnionID)
	}

	// 第二步：同一个人从 App 端登录，拿到的 openid 不同、unionid 相同 ⇒ 必须落回同一个账号。
	appSvc := newAppSvcOnDB(t, db, configuredAppCfg(), appOK(appOpenID, sharedUnion), nil, nil, zap.NewNop())
	res, err := appSvc.AppLogin(context.Background(), "app-code")
	if err != nil {
		t.Fatalf("App 端登录失败: %v", err)
	}
	if res.UserID != afterMp.ID {
		t.Fatalf("App 端应复用回填后的老账号: want %d got %d", afterMp.ID, res.UserID)
	}
	var count int64
	db.Model(&model.HrwaiUser{}).Count(&count)
	if count != 1 {
		t.Fatalf("回填 + unionid 认人之后不该多出第二个账号: count=%d", count)
	}
}
