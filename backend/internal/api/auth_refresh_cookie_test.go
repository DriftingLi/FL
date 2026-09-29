// ADR-0067（票 #1363）：/api/auth/refresh 的凭证读取顺序 = httpOnly Cookie → 请求体。
//
// seam：真实路由 + 内存库/内存黑名单，全部走 HTTP 层（与 auth_refresh_test.go 同一 seam）。
// 三条锁对应票面三条判据：
//
//	① Cookie 存在时忽略请求体（Cookie 与 body 各带一支合法 refresh 时，被消费的是 Cookie 那一支）；
//	   只有请求体时仍然可用（移动端与非浏览器客户端的生命线，响应体形状逐字不变）；
//	② 轮换/吊销语义不变：ADR-0016 的两族吊销（全会话 RevokeIdentity / 单会话 SignOut）
//	   对 Cookie 那一支同样成立——不是只对 body 那一族成立；
//	③ Cookie 属性（HttpOnly / SameSite=Lax / 生产 Secure / Path=/api/auth 认证族前缀）逐字锁死，
//	   SameSite 写成 None 必须判红；Path 收窄回单端点也要判红（那会让登出拿不到凭证，
//	   见 TestLogout_Cookie通道的refresh被吊销且响应清除Cookie 的第 ① 段）。
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

// cookieSession 生产口径的会话（父域 + Secure 可切），黑名单走内存实现。
// refresh cookie 的名字/域口径由 Session 自己推导（ADR-0067 不新增配置项），
// 因此这里断言的 Domain 就是 access cookie 的 Domain——两者只能同源。
func cookieSession(secure bool) *security.Session {
	return security.NewSessionWithBlacklistAndRefresh("test-secret", time.Hour, 7*24*time.Hour,
		security.CookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: secure}, newValBlacklist())
}

// doRefreshCookie 带 Cookie 通道（cookie 为空串则不带）发一次刷新请求；
// bodyToken 为空串则请求体里不带 refresh_token（连字段都不带，与「无 body 通道」等价）。
func doRefreshCookie(r *gin.Engine, cookie, bodyToken string) *httptest.ResponseRecorder {
	var payload any = map[string]string{}
	if bodyToken != "" {
		payload = map[string]string{"refresh_token": bodyToken}
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/auth/refresh", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: security.DefaultRefreshCookieName, Value: cookie})
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// refreshStatus 只看状态码的简写（响应头/响应体都要断言的场合直接用 doRefreshCookie）。
func refreshStatus(r *gin.Engine, cookie, bodyToken string) int {
	return doRefreshCookie(r, cookie, bodyToken).Code
}

// decodeRefresh 解析刷新信封：data 的键集合也在此锁定（响应体是移动端的唯一契约面）。
func decodeRefresh(t *testing.T, w *httptest.ResponseRecorder) (refreshResp, map[string]json.RawMessage) {
	t.Helper()
	var out refreshResp
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析刷新响应失败: %v body=%s", err, w.Body.String())
	}
	var envelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &envelope)
	return out, envelope.Data
}

// refreshCookieOf 从响应头里取主站 refresh cookie 的原始 Set-Cookie 行（逐字断言用）。
func refreshCookieOf(t *testing.T, w *httptest.ResponseRecorder, name string) (raw string, ok bool) {
	t.Helper()
	for _, line := range w.Result().Cookies() {
		if line.Name == name {
			hdr := w.Header().Values("Set-Cookie")
			for _, v := range hdr {
				if strings.HasPrefix(v, name+"=") {
					return v, true
				}
			}
			t.Fatalf("解析到 %s 但响应头里找不到对应 Set-Cookie: %v", name, w.Header().Values("Set-Cookie"))
		}
	}
	return "", false
}

func TestRefresh_Cookie优先于请求体(t *testing.T) {
	sess := cookieSession(false)
	r := newRefreshRouter(sess)

	// 同一身份手上两支**都合法**的 refresh：A 走 Cookie，B 走请求体。
	_, refA, _ := sess.IssuePair(1, "user1", service.HrwaiRole)
	_, refB, _ := sess.IssuePair(1, "user1", service.HrwaiRole)

	w := doRefreshCookie(r, refA, refB)
	if w.Code != http.StatusOK {
		t.Fatalf("Cookie 通道有效时应 200，实际 %d body=%s", w.Code, w.Body.String())
	}

	// 被消费的必须是 Cookie 那一支：A 已入黑名单（重放 401）。
	if code, _ := doRefresh(r, refA); code != http.StatusUnauthorized {
		t.Errorf("Cookie 那一支应被轮换消费（重放 401），实际 %d", code)
	}
	// 请求体那一支必须**原封不动**：它没被读、也没被吊销。
	if code, _ := doRefresh(r, refB); code != http.StatusOK {
		t.Errorf("请求体那一支应被忽略（仍可独立刷新），实际 %d", code)
	}
}

func TestRefresh_仅请求体仍然可用_移动端生命线(t *testing.T) {
	sess := cookieSession(false)
	r := newRefreshRouter(sess)
	_, rt, _ := sess.IssuePair(1, "user1", service.HrwaiRole)

	w := doRefreshCookie(r, "", rt)
	if w.Code != http.StatusOK {
		t.Fatalf("无 Cookie 时请求体通道应继续可用（移动端/非浏览器客户端），实际 %d body=%s", w.Code, w.Body.String())
	}
	out, data := decodeRefresh(t, w)
	if out.Data.Token == "" || out.Data.RefreshToken == "" {
		t.Fatalf("响应体必须同时返回新 access 与新 refresh: %+v", out.Data)
	}
	if out.Data.RefreshToken == rt {
		t.Error("轮换后 refresh 应变更")
	}
	// 移动端对齐用：data 的键集合逐字锁定（本票不新增/不改名/不删字段）。
	keys := refreshDataKeys(data)
	sort.Strings(keys)
	if got := strings.Join(keys, ","); got != "refresh_token,token" {
		t.Errorf("刷新响应 data 的键集合必须恰为 {refresh_token, token}，实际 %s", got)
	}
}

func TestRefresh_仅Cookie成功且同时回写新Cookie与响应体(t *testing.T) {
	sess := cookieSession(true)
	r := newRefreshRouter(sess)
	_, rt, _ := sess.IssuePair(1, "user1", service.HrwaiRole)

	w := doRefreshCookie(r, rt, "")
	if w.Code != http.StatusOK {
		t.Fatalf("Cookie 通道应 200，实际 %d body=%s", w.Code, w.Body.String())
	}
	out, _ := decodeRefresh(t, w)
	raw, ok := refreshCookieOf(t, w, security.DefaultRefreshCookieName)
	if !ok {
		t.Fatal("Cookie 通道轮换后必须回写新的 refresh Cookie（否则浏览器下一轮无凭证可用）")
	}
	if !strings.Contains(raw, out.Data.RefreshToken) {
		t.Error("Set-Cookie 里的值必须就是响应体里的新 refresh（两条通道同源，不能各发一支）")
	}
	if out.Data.RefreshToken == rt {
		t.Error("轮换后 refresh 应变更")
	}
	// 旧的那支（Cookie 里带来的）已吊销，重放 401
	if code := refreshStatus(r, rt, ""); code != http.StatusUnauthorized {
		t.Errorf("旧 Cookie 重放应 401，实际 %d", code)
	}
	// 新 Cookie 立即可用（链路可续）
	if code := refreshStatus(r, out.Data.RefreshToken, ""); code != http.StatusOK {
		t.Errorf("新 Cookie 应可继续轮换，实际 %d", code)
	}
}

func TestRefresh_Cookie与请求体都缺或都无效仍401(t *testing.T) {
	sess := cookieSession(false)
	r := newRefreshRouter(sess)
	_, rt, _ := sess.IssuePair(1, "user1", service.HrwaiRole)

	cases := []struct {
		name  string
		cook  string
		body  string
		exact bool // true 时请求体里连 refresh_token 字段都不带
	}{
		{name: "两者都缺", cook: "", body: ""},
		{name: "Cookie 无效", cook: "not-a-jwt", body: ""},
		{name: "请求体无效", cook: "", body: "not-a-jwt"},
		{name: "两者都无效", cook: "not-a-jwt", body: rt + "tampered"},
	}
	for _, c := range cases {
		w := doRefreshCookie(r, c.cook, c.body)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s：应 401，实际 %d body=%s", c.name, w.Code, w.Body.String())
		}
	}
	// 既有 4xx 语义（文案与状态码）不因新增 Cookie 通道而改变：统一「登录已过期，请重新登录」。
	w := doRefreshCookie(r, "", "")
	if !strings.Contains(w.Body.String(), "登录已过期，请重新登录") {
		t.Errorf("401 文案应保持既有口径，实际 %s", w.Body.String())
	}
	// access 传入刷新端点仍被拒（token_type 分流，ADR-0016）
	access, _, _ := sess.IssuePair(1, "user1", service.HrwaiRole)
	if code := refreshStatus(r, access, ""); code != http.StatusUnauthorized {
		t.Errorf("Cookie 里放 access 应 401，实际 %d", code)
	}
}

// 吊销族之一（ADR-0016 / ADR-0060 票2）：全会话吊销后，Cookie 里那支 refresh 必须被拒。
// 触发面走真实端点 DELETE /api/auth/account（注销 = RevokeIdentity），不是直接调函数。
func TestRefresh_全会话吊销后Cookie那支被拒(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	sess := cookieSession(false)
	u := model.HrwaiUser{UID: 900002, Account: "gone-cookie", Username: "注销带Cookie", Password: "x", Phone: "13900000002", Status: 1}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("播种学员账号失败: %v", err)
	}
	authSvc := service.NewAuthService(db, sess, service.NewForumCounter(), "admin", "tutor", "student", zap.NewNop())
	h := NewAuthHandler(sess, authSvc, nil, nil, nil, zap.NewNop())
	r := gin.New()
	g := r.Group("/api/auth", func(c *gin.Context) {
		c.Set(string(middleware.CtxUserID), u.ID)
		c.Next()
	})
	g.POST("/refresh", h.Refresh)
	g.DELETE("/account", h.DeleteAccount)

	// 注销前先在会话中段轮换一次（Cookie 通道），手上剩下的是新那支
	_, first, _ := sess.IssuePair(u.ID, u.Account, service.HrwaiRole)
	w := doRefreshCookie(r, first, "")
	if w.Code != http.StatusOK {
		t.Fatalf("注销前应能正常轮换: %d body=%s", w.Code, w.Body.String())
	}
	out, _ := decodeRefresh(t, w)
	live := out.Data.RefreshToken

	rec := performRequest(r, "DELETE", "/api/auth/account")
	if rec.Code != http.StatusOK {
		t.Fatalf("注销应 200，实际 %d", rec.Code)
	}
	// 吊销标记之后，Cookie 那一支（以及响应体那一份，同一枚）都必须换不出新令牌
	if code := refreshStatus(r, live, ""); code != http.StatusUnauthorized {
		t.Errorf("注销后 Cookie 那支 refresh 应 401，实际 %d", code)
	}
	// 注销响应要把 refresh Cookie 一并抹掉（本地凭证不留）
	if !hasClearedRefreshCookie(rec) {
		t.Error("注销响应必须带清除 refresh Cookie 的 Set-Cookie（Max-Age=-1）")
	}
}

// 吊销族之二（单会话终止 SignOut）。
//
// ⚠️ 本用例的**形状被改过**：上一版直接 `req.AddCookie(...)` 手工造请求，于是 Path 收在
// `/api/auth/refresh`（浏览器根本不会把它发到 /logout）时它照样绿 —— 典型的「锁声称比实际更硬」。
// 现在判据分两段，顺序不许换：
//
//	① 先按**浏览器的投递规则**问一句：服务端真正下发的那枚 Cookie，它的 Path 覆盖得到
//	  `/api/auth/logout` 吗？（Cookie 由 `SetRefreshCookie` 现下发，名字、值、属性都是浏览器手上那份）
//	② 覆盖得到，才谈吊销。
//
// ⇒ 谁把 Path 改窄回单端点，① 立刻判红，而不是留下一个「测过但生产不成立」的绿。
func TestLogout_Cookie通道的refresh被吊销且响应清除Cookie(t *testing.T) {
	sess := cookieSession(true)
	r := newRefreshRouter(sess)
	_, rt, _ := sess.IssuePair(1, "user1", service.HrwaiRole)

	// ① 投递前提：取服务端真正下发的那枚 Cookie（只读属性，不消费 rt）。
	issued := httptest.NewRecorder()
	sess.SetRefreshCookie(issued, rt)
	var ck *http.Cookie
	for _, c := range issued.Result().Cookies() {
		if c.Name == security.DefaultRefreshCookieName {
			ck = c
			break
		}
	}
	if ck == nil {
		t.Fatalf("登出入口的吊销前提无从判断：下发响应里没有 %s（实得 %v）",
			security.DefaultRefreshCookieName, issued.Result().Cookies())
	}
	if !strings.HasPrefix("/api/auth/logout", ck.Path) {
		t.Fatalf("refresh Cookie 的 Path=%q，按浏览器的前缀投递规则到不了 /api/auth/logout ⇒ 登出静默退化成「只清本地」"+
			"（`CONTEXT.md`「会话」词条的单会话终止失守）。若这是有意收窄，必须连同 ADR-0067 决策 2 与该词条一起改。", ck.Path)
	}

	// ② 吊销：浏览器据此把这枚 Cookie 投递到登出入口。
	req, _ := http.NewRequest("POST", "/api/auth/logout", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: ck.Name, Value: ck.Value})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("登出应 200，实际 %d", w.Code)
	}
	if code := refreshStatus(r, rt, ""); code != http.StatusUnauthorized {
		t.Errorf("登出后 Cookie 那支 refresh 应 401，实际 %d", code)
	}
	if !hasClearedRefreshCookie(w) {
		t.Error("登出响应必须清除 refresh Cookie")
	}
}

// Cookie 属性锁（ADR-0067 决策 2 的 CSRF 口径）：逐字断言 Set-Cookie 头部。
// 形状：SameSite 写成 None、Path 写成 /、Secure 在生产漏写，都必须判红。
func TestRefresh_Cookie属性锁(t *testing.T) {
	t.Run("生产（Secure=true）逐字口径", func(t *testing.T) {
		sess := cookieSession(true)
		r := newRefreshRouter(sess)
		_, rt, _ := sess.IssuePair(1, "user1", service.HrwaiRole)

		raw, ok := refreshCookieOf(t, doRefreshCookie(r, rt, ""), security.DefaultRefreshCookieName)
		if !ok {
			t.Fatal("轮换应下发 refresh Cookie")
		}
		attrs := cookieAttrTail(t, raw)
		// 属性集逐字锁定（顺序即 Go net/http 的写出顺序；改任何一个属性都会在这里判红）
		if attrs != "; Path=/api/auth; Domain=example.com; Max-Age=604800; HttpOnly; Secure; SameSite=Lax" {
			t.Errorf("Cookie 属性 = %q，期望生产口径（HttpOnly + SameSite=Lax + Secure + Path=认证族前缀）", attrs)
		}
	})

	t.Run("本地开发（Secure=false）只少 Secure 一位", func(t *testing.T) {
		sess := cookieSession(false)
		r := newRefreshRouter(sess)
		_, rt, _ := sess.IssuePair(1, "user1", service.HrwaiRole)

		attrs := cookieAttrTail(t, mustRefreshCookie(t, doRefreshCookie(r, rt, "")))
		if !strings.Contains(attrs, "HttpOnly") || !strings.Contains(attrs, "SameSite=Lax") {
			t.Errorf("本地口径也必须带 HttpOnly/SameSite=Lax: %q", attrs)
		}
		if strings.Contains(attrs, "Secure") {
			t.Errorf("Secure=false 时不得写出 Secure: %q", attrs)
		}
	})

	t.Run("SameSite=None 必须判红（防跨站触发刷新）", func(t *testing.T) {
		sess := cookieSession(true)
		r := newRefreshRouter(sess)
		_, rt, _ := sess.IssuePair(1, "user1", service.HrwaiRole)
		attrs := cookieAttrTail(t, mustRefreshCookie(t, doRefreshCookie(r, rt, "")))
		// 这一条就是「把 SameSite 写成 None 就得红」的形状：None 需要 Secure 才能落地，
		// 一旦有人改成 None，跨站 POST 就能带着凭证触发轮换（ADR-0067 决策 2 明令不允许）。
		if strings.Contains(attrs, "SameSite=None") {
			t.Errorf("refresh Cookie 不得是 SameSite=None: %q", attrs)
		}
	})

	t.Run("Path 必须收在该端点而非整站", func(t *testing.T) {
		sess := cookieSession(true)
		r := newRefreshRouter(sess)
		_, rt, _ := sess.IssuePair(1, "user1", service.HrwaiRole)
		attrs := cookieAttrTail(t, mustRefreshCookie(t, doRefreshCookie(r, rt, "")))
		if strings.Contains(attrs, "; Path=/;") || strings.HasSuffix(attrs, "; Path=/") {
			t.Errorf("Path 写成 / 会把 7 天凭证挂到全站每个请求上: %q", attrs)
		}
		if !strings.Contains(attrs, "; Path="+security.RefreshCookiePath+";") && !strings.HasSuffix(attrs, "; Path="+security.RefreshCookiePath) {
			t.Errorf("Path 必须等于 %s: %q", security.RefreshCookiePath, attrs)
		}
	})

	t.Run("域口径与 access cookie 同源，招聘者侧保持 host-only", func(t *testing.T) {
		// 主站：Domain 继承 AUTH_COOKIE_DOMAIN（子域名多工作区，域写错要么某子域拿不到、要么外扩）
		sess := cookieSession(true)
		if err := assertSameDomainScope(sess); err != nil {
			t.Error(err)
		}
		// 招聘者：access 是 host-only，refresh 也必须 host-only（不许把轮换凭证发到父域）
		rec := security.NewSessionWithRecruiterCookie("test-secret", time.Hour, 7*time.Hour,
			security.CookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: true},
			security.CookieConfig{Name: "recruiter_token", Domain: "", Secure: true}, newValBlacklist())
		_, refRT, _ := rec.IssuePair(5, "hr001", service.RecruiterRole)
		rr := newRefreshRouter(rec)
		req, _ := http.NewRequest("POST", "/api/auth/refresh", strings.NewReader(`{"refresh_token":"`+refRT+`"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		rr.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("招聘者请求体通道应 200，实际 %d", w.Code)
		}
		raw, ok := refreshCookieOf(t, w, security.DefaultRecruiterRefreshCookieName)
		if !ok {
			t.Fatalf("招聘者轮换应下发 recruiter_refresh，实际 Set-Cookie=%v", w.Header().Values("Set-Cookie"))
		}
		if strings.Contains(raw, "Domain=") {
			t.Errorf("招聘者 refresh Cookie 必须 host-only（不得带 Domain）: %q", raw)
		}
		if main := w.Header().Values("Set-Cookie"); countPrefixed(main, security.DefaultRefreshCookieName+"=") != 0 {
			t.Errorf("招聘者那支不得写进主站 cookie 名（作用域外扩）: %v", main)
		}
	})
}

// assertSameDomainScope refresh 与 access 两枚 Cookie 的 Domain/Secure 必须同源。
func assertSameDomainScope(sess *security.Session) error {
	w := httptest.NewRecorder()
	sess.SetLoginCookies(w, "access-x", "refresh-x")
	var accessDomain, refreshDomain string
	var accessSecure, refreshSecure bool
	var refreshPath string
	for _, ck := range w.Result().Cookies() {
		switch ck.Name {
		case "hrwai_token":
			accessDomain, accessSecure = ck.Domain, ck.Secure
		case security.DefaultRefreshCookieName:
			refreshDomain, refreshSecure, refreshPath = ck.Domain, ck.Secure, ck.Path
		}
	}
	if refreshDomain != accessDomain || refreshSecure != accessSecure {
		return fmt.Errorf("域口径漂移：access=(Domain:%q,Secure:%v) refresh=(Domain:%q,Secure:%v)",
			accessDomain, accessSecure, refreshDomain, refreshSecure)
	}
	if refreshPath == "/" {
		return fmt.Errorf("refresh cookie 的 Path 不得是 /")
	}
	return nil
}

// ===== helpers =====

func cookieAttrTail(t *testing.T, raw string) string {
	t.Helper()
	i := strings.Index(raw, ";")
	if i < 0 {
		t.Fatalf("Set-Cookie 缺属性段: %q", raw)
	}
	return raw[i:]
}

func mustRefreshCookie(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	raw, ok := refreshCookieOf(t, w, security.DefaultRefreshCookieName)
	if !ok {
		t.Fatalf("响应缺 refresh Cookie: %v", w.Header().Values("Set-Cookie"))
	}
	return raw
}

func hasClearedRefreshCookie(w *httptest.ResponseRecorder) bool {
	for _, ck := range w.Result().Cookies() {
		if ck.Name == security.DefaultRefreshCookieName && ck.MaxAge < 0 && ck.Value == "" {
			return true
		}
	}
	return false
}

func countPrefixed(lines []string, prefix string) int {
	var n int
	for _, l := range lines {
		if strings.HasPrefix(l, prefix) {
			n++
		}
	}
	return n
}

func refreshDataKeys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
