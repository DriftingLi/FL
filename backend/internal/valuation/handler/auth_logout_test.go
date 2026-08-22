// POST /api/valuation/auth/logout 估值登出测试（ADR-0016）：
// 与主站 /auth/refresh 口径一致——接收 refresh_token（请求体优先，回退 Bearer 头），
// 吊销后旧 refresh 不可再换新；不依赖 JWTAuth。
package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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
