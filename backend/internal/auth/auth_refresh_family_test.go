// 族判定在**端点缝**上的契约（#1376 跨端评审 · 移动端 ADR-0030 ④ 第 1 项的四条用例 + 域锁）。
//
// internal/security 的 refresh_family_test.go 锁的是「解析函数怎么选族」；本文件锁的是
// 「两个消费点（/refresh 与 /logout）真的走它」——位置判据必须打在消费点上，否则解析函数
// 写得再对，端点哪天换回名序也一样测不出（上一波在三条护栏上各空转过一次，教训在此）。
//
// 缺陷本体：两族 refresh cookie 在同一 host 上并存时，按 Cookie 名序选族会让招聘者面的续期
// 轮换掉学员那一族（UI 还停在招聘者身份 ⇒ 静默换身份）。旧实现在本文件第 1、3 条上判红。
package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/core"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// twoFamilySession 两族都配齐的会话：主站 refresh 落父域、招聘者 refresh 落 host-only
// （ADR-0022 的招牌形状）。生产里 `recruit.` 子域上「父域那枚学员 + host-only 那枚招聘者」
// 就是这么并存的 —— 这是**浏览器**侧的形状；App 侧容器不维持 cookie jar（#1389 真机读数），
// 所以本文件的夹具模拟的是 Web，不是移动端。
func twoFamilySession(t *testing.T) *security.Session {
	t.Helper()
	return security.NewSessionWithRecruiterCookie("test-secret", time.Hour, 7*24*time.Hour,
		security.CookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: true},
		security.CookieConfig{Name: "recruiter_token", Domain: "", Secure: true}, testutil.NewValueBlacklist())
}

// familyHTTP 向真实路由发一枚带两族 Cookie 的请求。
// header：Bearer 头里放的令牌（空串 = 不带）；accessCookies：access 侧 Cookie（可为空）；
// mainRT / recRT：两族 refresh（传空串表示这一族没带 Cookie）。
type familyHTTP struct {
	header        string
	accessCookies map[string]string
	mainRT        string
	recRT         string
}

func doFamily(t *testing.T, r *gin.Engine, path string, f familyHTTP) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if f.header != "" {
		req.Header.Set("Authorization", "Bearer "+f.header)
	}
	for name, value := range f.accessCookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	if f.mainRT != "" {
		req.AddCookie(&http.Cookie{Name: security.DefaultRefreshCookieName, Value: f.mainRT})
	}
	if f.recRT != "" {
		req.AddCookie(&http.Cookie{Name: security.DefaultRecruiterRefreshCookieName, Value: f.recRT})
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// setCookieNames 响应头里写下的 Cookie 名（按出现顺序，重复名保留）。
func setCookieNames(w *httptest.ResponseRecorder) []string {
	var names []string
	for _, line := range w.Header().Values("Set-Cookie") {
		if i := strings.Index(line, "="); i > 0 {
			names = append(names, line[:i])
		}
	}
	return names
}

func setCookieLine(w *httptest.ResponseRecorder, name string) (string, bool) {
	for _, line := range w.Header().Values("Set-Cookie") {
		if strings.HasPrefix(line, name+"=") {
			return line, true
		}
	}
	return "", false
}

// 用例 1：两族并存 + 头指向招聘者 ⇒ 轮换**招聘者**那一族。
// 判据分三面：状态码 / 哪一支旧凭证被消费掉（另一支必须原封不动）/ 回写落在哪一枚 Cookie 上。
func TestRefreshFamily_招聘者线索轮换招聘者族(t *testing.T) {
	sess := twoFamilySession(t)
	r := newRefreshRouter(sess)

	recAcc, recRT, _ := sess.IssuePair(9, "hr001", core.RecruiterRole)
	_, mainRT, _ := sess.IssuePair(1, "u1", core.HrwaiRole)

	w := doFamily(t, r, "/api/auth/refresh", familyHTTP{header: recAcc, mainRT: mainRT, recRT: recRT})
	if w.Code != http.StatusOK {
		t.Fatalf("招聘者族续期应 200，实际 %d body=%s", w.Code, w.Body.String())
	}

	if code, _ := doRefresh(r, recRT); code != http.StatusUnauthorized {
		t.Errorf("招聘者那一族应被消费（重放 401），实际 %d —— 名序选族正是在这里把学员那族换掉的", code)
	}
	if code, _ := doRefresh(r, mainRT); code != http.StatusOK {
		t.Errorf("学员那一支必须原封不动（仍可独立续期），实际 %d —— 被跨族吊销就是串族", code)
	}
	raw, ok := setCookieLine(w, security.DefaultRecruiterRefreshCookieName)
	if !ok {
		t.Fatalf("回写必须落在 recruiter_refresh 上，实际 Set-Cookie=%v", setCookieNames(w))
	}
	if n := countPrefixed(setCookieNames(w), security.DefaultRefreshCookieName); n != 0 {
		t.Errorf("招聘者续期不得写主站 refresh（两枚都写=身份漂移）：%v", setCookieNames(w))
	}
	// 用例 5（域锁，按新判据重述）：招聘者那族续出来的凭证仍 host-only，不外扩到父域。
	if strings.Contains(raw, "Domain=") {
		t.Errorf("招聘者 refresh 必须 host-only: %q", raw)
	}
}

// 用例 2（防反向）：两族并存 + 线索指向学员 ⇒ 轮换学员那一族，招聘者那一支不许被碰。
func TestRefreshFamily_学员线索轮换学员族(t *testing.T) {
	sess := twoFamilySession(t)
	r := newRefreshRouter(sess)

	mainAcc, mainRT, _ := sess.IssuePair(1, "u1", core.HrwaiRole)
	recAcc, recRT, _ := sess.IssuePair(9, "hr001", core.RecruiterRole)

	w := doFamily(t, r, "/api/auth/refresh", familyHTTP{header: mainAcc, mainRT: mainRT, recRT: recRT})
	if w.Code != http.StatusOK {
		t.Fatalf("学员族续期应 200，实际 %d", w.Code)
	}
	if code, _ := doRefresh(r, mainRT); code != http.StatusUnauthorized {
		t.Errorf("学员那一族应被消费，实际 %d", code)
	}
	if code, _ := doRefresh(r, recRT); code != http.StatusOK {
		t.Errorf("招聘者那一支必须原封不动，实际 %d（recruiter access 仍有效=%t）",
			code, func() bool { _, err := sess.VerifyAccess(recAcc); return err == nil }())
	}
	raw, ok := setCookieLine(w, security.DefaultRefreshCookieName)
	if !ok {
		t.Fatalf("回写必须落在 hrwai_refresh 上，实际 %v", setCookieNames(w))
	}
	if !strings.Contains(raw, "Domain=example.com") {
		t.Errorf("主站 refresh 必须继承父域（子域名多工作区）: %q", raw)
	}
}

// 用例 3（负锁，本票缺陷的本体）：两族并存而**没有任何族线索** ⇒ 401，
// 且两族都**没有被轮换**。「绝不回退名序」只有从这两面同时断言才算锁：
// 只断 401 的话，实现改成「先轮换主站、再报错」也能过。
func TestRefreshFamily_无线索不回退名序(t *testing.T) {
	sess := twoFamilySession(t)
	r := newRefreshRouter(sess)

	_, mainRT, _ := sess.IssuePair(1, "u1", core.HrwaiRole)
	_, recRT, _ := sess.IssuePair(9, "hr001", core.RecruiterRole)

	// 不带 Authorization、不带任何 access cookie，只带两枚 refresh；请求体也没有凭证。
	w := doFamily(t, r, "/api/auth/refresh", familyHTTP{mainRT: mainRT, recRT: recRT})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("无族线索时必须 401（回退名序就是在替别人续期），实际 %d body=%s", w.Code, w.Body.String())
	}
	if _, ok := setCookieLine(w, security.DefaultRefreshCookieName); ok {
		t.Error("401 的响应不得下发主站 refresh")
	}
	if _, ok := setCookieLine(w, security.DefaultRecruiterRefreshCookieName); ok {
		t.Error("401 的响应不得下发招聘者 refresh")
	}
	// 两支都还在手上：谁都没被吊销（旧实现在这里会把主站那支换掉）。
	if code, _ := doRefresh(r, mainRT); code != http.StatusOK {
		t.Errorf("主站那支不该被无线索的请求消费，重放应 200，实际 %d", code)
	}
	if code, _ := doRefresh(r, recRT); code != http.StatusOK {
		t.Errorf("招聘者那支不该被无线索的请求消费，重放应 200，实际 %d", code)
	}
}

// 用例 4（防回归）：单族请求的行为逐字不变——只带主站 Cookie + 学员头 ⇒ 200 且回写主站那枚。
func TestRefreshFamily_单族行为不变(t *testing.T) {
	sess := twoFamilySession(t)
	r := newRefreshRouter(sess)
	mainAcc, mainRT, _ := sess.IssuePair(1, "u1", core.HrwaiRole)

	w := doFamily(t, r, "/api/auth/refresh", familyHTTP{header: mainAcc, mainRT: mainRT})
	if w.Code != http.StatusOK {
		t.Fatalf("单族续期应 200，实际 %d body=%s", w.Code, w.Body.String())
	}
	out, _ := decodeRefresh(t, w)
	if out.Data.Token == "" || out.Data.RefreshToken == "" {
		t.Fatalf("响应体双令牌形状不变: %+v", out.Data)
	}
	if _, ok := setCookieLine(w, security.DefaultRefreshCookieName); !ok {
		t.Errorf("必须回写主站 refresh，实际 %v", setCookieNames(w))
	}
	// 新那枚 Cookie 继续可用（族线索换成续出来的新 access——链路可续，形状与前端下一轮一致）。
	if code := doFamily(t, r, "/api/auth/refresh",
		familyHTTP{header: out.Data.Token, mainRT: out.Data.RefreshToken}).Code; code != http.StatusOK {
		t.Errorf("新那枚 Cookie 应能继续轮换，实际 %d", code)
	}
}

// 同源锁：同一枚请求里，「鉴权面认的活跃身份」与「续期面选的令牌族」必须是同一个答案。
// 形状取最刁的一组：不带 Bearer 头，两枚 access cookie 并存（父域学员 + host-only 招聘者），
// 于是两边都只能按 access cookie 的顺序定族——一旦有人只改其中一处的顺序，本用例即红。
func TestRefreshFamily_与鉴权面同源(t *testing.T) {
	sess := twoFamilySession(t)
	r := newRefreshRouter(sess)
	probe := gin.New()
	probe.GET("/p", middleware.JWTAuth(sess), func(c *gin.Context) {
		role, _ := c.Get(string(middleware.CtxUserRole))
		c.String(http.StatusOK, "%v", role)
	})

	mainAcc, mainRT, _ := sess.IssuePair(1, "u1", core.HrwaiRole)
	recAcc, recRT, _ := sess.IssuePair(9, "hr001", core.RecruiterRole)

	preq, _ := http.NewRequest(http.MethodGet, "/p", nil)
	preq.AddCookie(&http.Cookie{Name: "hrwai_token", Value: mainAcc})
	preq.AddCookie(&http.Cookie{Name: "recruiter_token", Value: recAcc})
	pw := httptest.NewRecorder()
	probe.ServeHTTP(pw, preq)
	authed := pw.Body.String()
	if !strings.Contains(authed, core.HrwaiRole) {
		t.Fatalf("夹具前提不成立：鉴权面对两枚 access cookie 并存认的不是学员，实得 %q", authed)
	}

	w := doFamily(t, r, "/api/auth/refresh", familyHTTP{
		accessCookies: map[string]string{"hrwai_token": mainAcc, "recruiter_token": recAcc},
		mainRT:        mainRT, recRT: recRT,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("按 access cookie 定族应 200，实际 %d body=%s", w.Code, w.Body.String())
	}
	if _, ok := setCookieLine(w, security.DefaultRefreshCookieName); !ok {
		t.Errorf("续期面选的族与鉴权面不一致（鉴权面=%q，回写=%v）：两处顺序必须同源", authed, setCookieNames(w))
	}
	if code, _ := doRefresh(r, recRT); code != http.StatusOK {
		t.Error("招聘者那一族不该被学员身份的续期消费")
	}
}

// 登出与续期共用同一把定族钥匙（Path 放宽到 /api/auth 之后，登出也面对同样的并存问题）：
// 招聘者线索的登出只吊销招聘者那一族，主站那支必须继续可用。
func TestRefreshFamily_登出同口径(t *testing.T) {
	sess := twoFamilySession(t)
	r := newRefreshRouter(sess)

	recAcc, recRT, _ := sess.IssuePair(9, "hr001", core.RecruiterRole)
	_, mainRT, _ := sess.IssuePair(1, "u1", core.HrwaiRole)

	w := doFamily(t, r, "/api/auth/logout", familyHTTP{header: recAcc, mainRT: mainRT, recRT: recRT})
	if w.Code != http.StatusOK {
		t.Fatalf("登出应 200，实际 %d", w.Code)
	}
	if code := refreshStatus(r, "", recRT); code != http.StatusUnauthorized {
		t.Errorf("招聘者那一族应被登出吊销，实际 %d", code)
	}
	if code := refreshStatus(r, "", mainRT); code != http.StatusOK {
		t.Errorf("学员那一支不该被招聘者的登出吊销（仍可续期），实际 %d", code)
	}
	if !hasClearedRefreshCookie(w) {
		t.Error("登出仍要清除两族 Cookie（本地凭证不留）")
	}
}
