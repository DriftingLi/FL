// POST /api/valuation/auth/logout 估值登出测试（ADR-0016 + #1388 的 ADR-0067 射程补齐）：
// 接收 refresh_token（请求体优先，回退 Bearer 头），吊销后旧 refresh 不可再换新，
// 并且**必须清除登录态 Cookie**（access + 两族 refresh）；不依赖 JWTAuth。
// 另有一条反向可达性锁：这枚 refresh cookie 的 Path 结构上覆盖不到本端点（见文件末）。
package valuation

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	mainmodel "forklift-training/internal/model"
	"forklift-training/internal/security"
)

// memBlacklistStore 内存版黑名单存储（本测试专用；生产走 Redis SETNX）。
type memBlacklistStore struct {
	mu sync.Mutex
	m  map[string]string
}

func newMemBlacklistStore() *memBlacklistStore { return &memBlacklistStore{m: map[string]string{}} }

func (s *memBlacklistStore) Get(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[key]; !ok {
		return "", errors.New("not found")
	}
	return "1", nil
}

func (s *memBlacklistStore) Set(_ context.Context, key, value string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
	return nil
}

func (s *memBlacklistStore) PutIfAbsent(_ context.Context, key, value string, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[key]; ok {
		return false, nil
	}
	s.m[key] = value
	return true, nil
}

func newLogoutRouter(sess *security.Session) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewValuationAuthHandler(&fakeValuationAuth{
		user: &mainmodel.HrwaiUser{ID: 1, Account: "acct_alice", Username: "alice"},
	}, sess)
	r.POST("/api/valuation/auth/logout", h.Logout)
	return r
}

func doValuationLogout(r *gin.Engine, body string, bearer string) int {
	req, _ := http.NewRequest("POST", "/api/valuation/auth/logout", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

// TestValuationLogout_RevokesRefreshThenRotationRejected 估值登出吊销 refresh：
// 登出后旧 refresh 不可再换新（RotateRefresh 返回 ErrInvalidRefresh）。
func TestValuationLogout_RevokesRefreshThenRotationRejected(t *testing.T) {
	store := newMemBlacklistStore()
	sess := security.NewSessionWithBlacklistAndRefresh("test-secret", 2*time.Hour, 7*time.Hour,
		security.CookieConfig{Name: "hrwai_token"}, store)
	r := newLogoutRouter(sess)

	_, rt, err := sess.IssuePair(1, "acct_alice", "hrwai_user")
	if err != nil {
		t.Fatalf("签发双令牌失败: %v", err)
	}

	// 无 Authorization 头、仅 body 带 refresh_token（access 过期场景亦可登出）
	if code := doValuationLogout(r, `{"refresh_token":"`+rt+`"}`, ""); code != 200 {
		t.Fatalf("登出应 200，得到 %d", code)
	}

	// 旧 refresh 不可再换新：主站刷新路径原子轮换被拒
	if _, _, err := sess.RotateRefresh(context.Background(), rt); !errors.Is(err, security.ErrInvalidRefresh) {
		t.Fatalf("估值登出后旧 refresh 应不可再换新, got %v", err)
	}
}

// TestValuationLogout_BearerHeaderFallback 回退口径：Bearer 头携带 refresh_token 亦可登出。
func TestValuationLogout_BearerHeaderFallback(t *testing.T) {
	store := newMemBlacklistStore()
	sess := security.NewSessionWithBlacklistAndRefresh("test-secret", 2*time.Hour, 7*time.Hour,
		security.CookieConfig{Name: "hrwai_token"}, store)
	r := newLogoutRouter(sess)

	_, rt, _ := sess.IssuePair(1, "acct_alice", "hrwai_user")

	if code := doValuationLogout(r, `{}`, rt); code != 200 {
		t.Fatalf("Bearer 头登出应 200，得到 %d", code)
	}
	if _, _, err := sess.RotateRefresh(context.Background(), rt); !errors.Is(err, security.ErrInvalidRefresh) {
		t.Fatalf("Bearer 头登出后旧 refresh 应不可再换新, got %v", err)
	}
}

// TestValuationLogout_EmptyRequestSilentlyOK 空 body / 无头静默放行（幂等，不报错）。
func TestValuationLogout_EmptyRequestSilentlyOK(t *testing.T) {
	store := newMemBlacklistStore()
	sess := security.NewSessionWithBlacklistAndRefresh("test-secret", 2*time.Hour, 7*time.Hour,
		security.CookieConfig{Name: "hrwai_token"}, store)
	r := newLogoutRouter(sess)

	if code := doValuationLogout(r, "", ""); code != 200 {
		t.Fatalf("空请求登出应静默 200，得到 %d", code)
	}
	if len(store.m) != 0 {
		t.Errorf("无 token 不应写黑名单，实际 %d 条", len(store.m))
	}
}

// doValuationLogoutRecorder 与 doValuationLogout 同形状，但把整个响应留着断言响应头。
func doValuationLogoutRecorder(r *gin.Engine, body, bearer string) *httptest.ResponseRecorder {
	req, _ := http.NewRequest("POST", "/api/valuation/auth/logout", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// loggedOutCookie 断言响应里名为 name 的 Set-Cookie 是「删除形状」：值为空、Max-Age<0，
// 且 refresh 那两枚必须与写入侧同一个 Path（Path 不匹配浏览器就不认这条是删除）。
func loggedOutCookie(t *testing.T, w *httptest.ResponseRecorder, name string, wantPath string) {
	t.Helper()
	for _, ck := range w.Result().Cookies() {
		if ck.Name != name {
			continue
		}
		if ck.Value != "" || ck.MaxAge >= 0 {
			t.Errorf("%s 未清除（应为空值 + Max-Age<0）: %+v", name, ck)
		}
		if wantPath != "" && ck.Path != wantPath {
			t.Errorf("%s 清除的 Path = %q，必须与写入侧 %q 一致", name, ck.Path, wantPath)
		}
		return
	}
	t.Errorf("登出响应缺 %s 的清除头（假登出：浏览器侧凭证原封不动）: %v", name, w.Header().Values("Set-Cookie"))
}

// TestValuationLogout_三种入口都必须清除登录态Cookie 是 #1388 的本体：
// 旧实现只吊销、一枚 Cookie 都不清 ⇒ 还在用这个对外入口的客户端「登出」后，浏览器里那枚
// 7 天 refresh 与 access cookie 原封不动，登出退化成前端自己把状态擦了。
// 判据按**入口**取表而不是只测「吊销成功」那一路：空请求（拿不到任何可吊销凭证）同样必须清 ——
// 只断言成功那一路的话，「失败不清」这个形状是测不到的。
func TestValuationLogout_三种入口都必须清除登录态Cookie(t *testing.T) {
	newSess := func() *security.Session {
		return security.NewSessionWithBlacklistAndRefresh("test-secret", 2*time.Hour, 7*time.Hour,
			security.CookieConfig{Name: "hrwai_token"}, newMemBlacklistStore())
	}

	t.Run("body 带 refresh", func(t *testing.T) {
		sess := newSess()
		_, rt, _ := sess.IssuePair(1, "acct_alice", "hrwai_user")
		w := doValuationLogoutRecorder(newLogoutRouter(sess), `{"refresh_token":"`+rt+`"}`, "")
		assertLoginCookiesCleared(t, w)
	})

	t.Run("Bearer 头兜底", func(t *testing.T) {
		sess := newSess()
		_, rt, _ := sess.IssuePair(1, "acct_alice", "hrwai_user")
		assertLoginCookiesCleared(t, doValuationLogoutRecorder(newLogoutRouter(sess), `{}`, rt))
	})

	t.Run("空请求（无可吊销凭证）", func(t *testing.T) {
		sess := newSess()
		assertLoginCookiesCleared(t, doValuationLogoutRecorder(newLogoutRouter(sess), "", ""))
	})
}

func assertLoginCookiesCleared(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	loggedOutCookie(t, w, "hrwai_token", "/")
	loggedOutCookie(t, w, security.DefaultRefreshCookieName, security.RefreshCookiePath)
	loggedOutCookie(t, w, security.DefaultRecruiterRefreshCookieName, security.RefreshCookiePath)
}

// TestValuationLogout_刷新Cookie到不了本端点 把「为什么本端点不读 Cookie」从注释里的断言
// 变成实测：取服务端**真正下发**的那枚 refresh cookie 的 Path，按浏览器的路径前缀投递规则
// 问一句它覆盖得到本端点吗。
//
// 方向与主站那条登出锁**相反**（#1376 的 `TestLogout_Cookie通道的refresh被吊销且响应清除Cookie`
// 断言 `/api/auth/logout` 必须送得到），两条一起才把 `RefreshCookiePath` 的口径钉死：
// 收窄回 `/api/auth/refresh` 会红在主站那条，放宽到 `/`（把 7 天凭证挂到全站每一个请求上）
// 会红在这一条 —— 而不是只有一个人说话。
func TestValuationLogout_刷新Cookie到不了本端点(t *testing.T) {
	sess := security.NewSessionWithBlacklistAndRefresh("test-secret", 2*time.Hour, 7*time.Hour,
		security.CookieConfig{Name: "hrwai_token"}, newMemBlacklistStore())

	issued := httptest.NewRecorder()
	_, rt, _ := sess.IssuePair(1, "acct_alice", "hrwai_user")
	sess.SetRefreshCookie(issued, rt)

	var ck *http.Cookie
	for _, c := range issued.Result().Cookies() {
		if c.Name == security.DefaultRefreshCookieName {
			ck = c
			break
		}
	}
	if ck == nil {
		t.Fatalf("判据没找到输入：下发响应里没有 %s（实得 %v）", security.DefaultRefreshCookieName,
			issued.Result().Cookies())
	}
	if strings.HasPrefix("/api/valuation/auth/logout", ck.Path) {
		t.Fatalf("refresh cookie 的 Path=%q 现在覆盖得到估值登出入口 ⇒ ADR-0067 决策 2 的「最小暴露面」口径已变，"+
			"必须连带重新论证（Path=/ 等于把 7 天凭证挂到全站每个请求上），并给本端点补上「Cookie 优先 + 按 access 定族」"+
			"的读取与用例。不许只留这条红话过去。", ck.Path)
	}
}
