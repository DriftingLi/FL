// POST /api/auth/refresh 双令牌刷新端点测试（ADR-0012）：
// 轮换（旧 refresh 立即失效防重放）、黑名单拒绝、access 传入被拒、登出撤销 refresh。
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/security"
)

// memBlacklist 内存黑名单存储（本测试专用；生产走 Redis）。
type memBlacklist struct {
	mu sync.Mutex
	m  map[string]string
}

func newMemBlacklist() *memBlacklist { return &memBlacklist{m: map[string]string{}} }

func (s *memBlacklist) Get(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[key]; !ok {
		return "", errors.New("not found")
	}
	return "1", nil
}

func (s *memBlacklist) Set(_ context.Context, key, _ string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = "1"
	return nil
}

// PutIfAbsent 原子抢占（SETNX 语义），与生产 Redis 行为对齐。
func (s *memBlacklist) PutIfAbsent(_ context.Context, key, _ string, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[key]; ok {
		return false, nil
	}
	s.m[key] = "1"
	return true, nil
}

// newRefreshRouter 构造仅含 /api/auth/{refresh,logout} 的最小路由（其余服务注入 nil）。
func newRefreshRouter(sess *security.Session) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAuthHandler(sess, nil, nil, nil, nil, zap.NewNop())
	g := r.Group("/api/auth")
	g.POST("/refresh", h.Refresh)
	g.POST("/logout", h.Logout)
	return r
}

// refreshResp 刷新接口信封 data。
type refreshResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	} `json:"data"`
}

func doRefresh(r *gin.Engine, rt string) (int, refreshResp) {
	body, _ := json.Marshal(map[string]string{"refresh_token": rt})
	req, _ := http.NewRequest("POST", "/api/auth/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out refreshResp
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestRefresh_RotateAndRevokeOld(t *testing.T) {
	sess := security.NewSessionWithBlacklistAndRefresh("test", 2*time.Hour, 7*time.Hour, security.CookieConfig{Name: "hrwai_token"}, newMemBlacklist())
	r := newRefreshRouter(sess)

	_, oldRT, _ := sess.IssuePair(1, "user1", "hrwai_user")

	// 第一次刷新成功：返回新 access + 新 refresh，旧 refresh 被吊销
	code, resp := doRefresh(r, oldRT)
	if code != 200 || resp.Data.Token == "" || resp.Data.RefreshToken == "" {
		t.Fatalf("首次刷新应成功: code=%d resp=%+v", code, resp)
	}
	if resp.Data.RefreshToken == oldRT {
		t.Error("轮换后 refresh 应变更")
	}

	// 旧 refresh 重放被拒（已入黑名单）
	code2, _ := doRefresh(r, oldRT)
	if code2 != 401 {
		t.Errorf("旧 refresh 重放应 401，得到 %d", code2)
	}
}

func TestRefresh_RejectsOwnAccessToken(t *testing.T) {
	sess := security.NewSessionWithBlacklistAndRefresh("test", 2*time.Hour, 7*time.Hour, security.CookieConfig{Name: "hrwai_token"}, newMemBlacklist())
	r := newRefreshRouter(sess)

	access, _, _ := sess.IssuePair(1, "user1", "hrwai_user")
	// access token 传入刷新端点应被拒（type 分流）
	code, _ := doRefresh(r, access)
	if code != 401 {
		t.Errorf("access token 传入刷新端点应 401，得到 %d", code)
	}
}

func TestRefresh_MissingToken401(t *testing.T) {
	sess := security.NewSessionWithBlacklistAndRefresh("test", 2*time.Hour, 7*time.Hour, security.CookieConfig{Name: "hrwai_token"}, newMemBlacklist())
	r := newRefreshRouter(sess)

	code, _ := doRefresh(r, "")
	if code != 401 {
		t.Errorf("空 refresh 应 401，得到 %d", code)
	}
}

func TestLogout_ThenRefreshRejected(t *testing.T) {
	sess := security.NewSessionWithBlacklistAndRefresh("test", 2*time.Hour, 7*time.Hour, security.CookieConfig{Name: "hrwai_token"}, newMemBlacklist())
	r := newRefreshRouter(sess)

	_, rt, _ := sess.IssuePair(1, "user1", "hrwai_user")

	// 登出：撤销 refresh
	logoutBody, _ := json.Marshal(map[string]string{"refresh_token": rt})
	lreq, _ := http.NewRequest("POST", "/api/auth/logout", bytes.NewReader(logoutBody))
	lreq.Header.Set("Content-Type", "application/json")
	lw := httptest.NewRecorder()
	r.ServeHTTP(lw, lreq)
	if lw.Code != 200 {
		t.Fatalf("登出应 200，得到 %d", lw.Code)
	}

	// 登出后 refresh 失效
	code, _ := doRefresh(r, rt)
	if code != 401 {
		t.Errorf("登出后 refresh 刷新应 401，得到 %d", code)
	}
}
