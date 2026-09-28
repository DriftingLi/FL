// ADR-0067（票 #1363）会话模块侧的 refresh Cookie 单测：读取优先级、下发属性、清除形状、
// 轮换回写。端点侧的契约（Cookie 优先于请求体 / 两族吊销）在 internal/api 的契约测试里锁。
package security

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRefreshCookieName_默认与配置推导(t *testing.T) {
	sess := NewSessionWithBlacklistAndRefresh(testSecret, time.Hour, 7*24*time.Hour,
		CookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: true}, newInmemoryBlacklistStore())

	if got := sess.RefreshCookieName(); got != DefaultRefreshCookieName {
		t.Errorf("RefreshCookieName = %q, want %q", got, DefaultRefreshCookieName)
	}
	if got := sess.RecruiterRefreshCookieName(); got != DefaultRecruiterRefreshCookieName {
		t.Errorf("RecruiterRefreshCookieName = %q", got)
	}
	// 优先级与 access 侧同口径：主站先、招聘者次之（两族并存时不做隐式回退）
	names := sess.RefreshCookieNames()
	if len(names) != 2 || names[0] != DefaultRefreshCookieName || names[1] != DefaultRecruiterRefreshCookieName {
		t.Errorf("RefreshCookieNames = %v", names)
	}
}

func TestExtractRefreshCookie_读取优先级(t *testing.T) {
	sess := NewSessionWithBlacklistAndRefresh(testSecret, time.Hour, 7*24*time.Hour,
		CookieConfig{Name: "hrwai_token", Domain: "example.com"}, newInmemoryBlacklistStore())

	cases := []struct {
		name    string
		cookies map[string]string
		want    string
	}{
		{name: "无 Cookie", cookies: nil, want: ""},
		{name: "只有主站", cookies: map[string]string{DefaultRefreshCookieName: "rt-main"}, want: "rt-main"},
		{name: "只有招聘者", cookies: map[string]string{DefaultRecruiterRefreshCookieName: "rt-rec"}, want: "rt-rec"},
		{name: "两族并存取主站", cookies: map[string]string{
			DefaultRefreshCookieName: "rt-main", DefaultRecruiterRefreshCookieName: "rt-rec"}, want: "rt-main"},
		{name: "空值 Cookie 不算带上通道", cookies: map[string]string{DefaultRefreshCookieName: ""}, want: ""},
		{name: "无关 Cookie 干扰", cookies: map[string]string{"hrwai_token": "access-x"}, want: ""},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodPost, RefreshCookiePath, nil)
		for n, v := range c.cookies {
			req.AddCookie(&http.Cookie{Name: n, Value: v})
		}
		if got := sess.ExtractRefreshCookie(req); got != c.want {
			t.Errorf("%s: ExtractRefreshCookie = %q, want %q", c.name, got, c.want)
		}
	}
}

// 下发属性：httpOnly + SameSite=Lax + Path 收在刷新端点 + Max-Age = refresh 有效期；
// 域口径逐字继承 access cookie（子域名多工作区，作用域只能有一处事实源）。
func TestSetRefreshCookie_属性与域口径继承(t *testing.T) {
	sess := NewSessionWithBlacklistAndRefresh(testSecret, time.Hour, 7*24*time.Hour,
		CookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: true}, newInmemoryBlacklistStore())

	w := httptest.NewRecorder()
	sess.SetRefreshCookie(w, "rt-1")
	ck := mustCookieNamed(t, w, DefaultRefreshCookieName)
	if !ck.HttpOnly {
		t.Error("refresh cookie 必须 HttpOnly（否则脚本可读，ADR-0067 的整层意义就没了）")
	}
	if ck.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v，必须是 Lax（None 允许跨站携带 ⇒ 判红）", ck.SameSite)
	}
	if !ck.Secure {
		t.Error("Secure 必须继承 access cookie 配置")
	}
	if ck.Domain != "example.com" {
		t.Errorf("Domain = %q，必须继承 access cookie 的父域", ck.Domain)
	}
	if ck.Path != RefreshCookiePath {
		t.Errorf("Path = %q, want %q", ck.Path, RefreshCookiePath)
	}
	if ck.MaxAge != int((7 * 24 * time.Hour).Seconds()) {
		t.Errorf("MaxAge = %d，应等于 refresh 有效期秒数", ck.MaxAge)
	}
}

func TestClearRefreshCookies_MaxAge为负且Path同写入(t *testing.T) {
	sess := NewSessionWithBlacklistAndRefresh(testSecret, time.Hour, 7*24*time.Hour,
		CookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: true}, newInmemoryBlacklistStore())

	w := httptest.NewRecorder()
	sess.ClearRefreshCookies(w)
	for _, name := range sess.RefreshCookieNames() {
		ck := mustCookieNamed(t, w, name)
		if ck.MaxAge >= 0 || ck.Value != "" {
			t.Errorf("%s 清除应为 MaxAge<0 且值为空: %+v", name, ck)
		}
		if ck.Path != RefreshCookiePath {
			t.Errorf("%s 清除的 Path 必须与写入一致（%q），否则浏览器不认这条删除: %q", name, RefreshCookiePath, ck.Path)
		}
	}
}

// 轮换回写：新 Cookie 的值必须就是响应体里那枚新 refresh，且旧值立即不可再用（ADR-0016 语义不变）。
func TestRotateAndSetCookie_回写新Cookie且旧值吊销(t *testing.T) {
	sess := NewSessionWithBlacklistAndRefresh(testSecret, time.Hour, 7*24*time.Hour,
		CookieConfig{Name: "hrwai_token", Domain: "example.com"}, newInmemoryBlacklistStore())
	ctx := context.Background()

	_, rt, _ := sess.IssuePair(3, "u3", "hrwai_user")
	w := httptest.NewRecorder()
	access, refresh, err := sess.RotateAndSetCookie(ctx, w, rt)
	if err != nil {
		t.Fatalf("轮换失败: %v", err)
	}
	if access == "" || refresh == "" {
		t.Fatal("RotateAndSetCookie 必须返回新双令牌")
	}
	ck := mustCookieNamed(t, w, DefaultRefreshCookieName)
	if ck.Value != refresh {
		t.Error("Cookie 与响应体必须是同一枚新 refresh")
	}
	if _, _, err := sess.RotateRefresh(ctx, rt); err == nil {
		t.Error("旧 refresh 必须已吊销（轮换语义不因 Cookie 通道而变）")
	}
}

// 角色分流：招聘者那支只落 host-only 的 recruiter_refresh，不写主站 cookie 名（作用域不外扩）。
func TestRotateAndSetCookie_按角色分流(t *testing.T) {
	sess := NewSessionWithRecruiterCookie(testSecret, time.Hour, 7*24*time.Hour,
		CookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: true},
		CookieConfig{Name: "recruiter_token", Domain: "", Secure: true}, newInmemoryBlacklistStore())

	_, rt, _ := sess.IssuePair(9, "hr001", "recruiter")
	w := httptest.NewRecorder()
	if _, _, err := sess.RotateAndSetCookie(context.Background(), w, rt); err != nil {
		t.Fatalf("轮换失败: %v", err)
	}
	ck := mustCookieNamed(t, w, DefaultRecruiterRefreshCookieName)
	if ck.Domain != "" {
		t.Errorf("招聘者 refresh 必须 host-only，实际 Domain=%q", ck.Domain)
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == DefaultRefreshCookieName {
			t.Error("招聘者那支不得写进主站 refresh cookie 名")
		}
	}
}

// SetLoginCookies：登录路径一次下发两枚（access 保持原口径、refresh 新增）。
func TestSetLoginCookies_两枚一次下发(t *testing.T) {
	sess := NewSessionWithBlacklistAndRefresh(testSecret, time.Hour, 7*24*time.Hour,
		CookieConfig{Name: "hrwai_token", Domain: "example.com"}, newInmemoryBlacklistStore())

	w := httptest.NewRecorder()
	sess.SetLoginCookies(w, "access-1", "refresh-1")
	cookies := w.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("应写 2 个 cookie，实际 %d", len(cookies))
	}
	access := mustCookieNamed(t, w, "hrwai_token")
	if access.Path != "/" || access.MaxAge != int(time.Hour.Seconds()) {
		t.Errorf("access cookie 口径不得因本票改变: %+v", access)
	}
	if raw := w.Header().Get("Set-Cookie"); !strings.Contains(raw, "HttpOnly") {
		t.Errorf("Set-Cookie 缺 HttpOnly: %q", raw)
	}
}

func mustCookieNamed(t *testing.T, w *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, ck := range w.Result().Cookies() {
		if ck.Name == name {
			return ck
		}
	}
	t.Fatalf("响应里找不到名为 %s 的 cookie: %v", name, w.Header().Values("Set-Cookie"))
	return nil
}
