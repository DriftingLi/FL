package security

import (
	"context"
	"testing"
	"time"
)

// newTestDualSession 构造带 2h access / 7h refresh 的会话（refresh 短一些便于测试）。
func newTestDualSession(blacklist BlacklistStore) *Session {
	if blacklist == nil {
		blacklist = newInmemoryBlacklistStore()
	}
	return NewSessionWithBlacklistAndRefresh(testSecret, 2*time.Hour, 7*time.Hour, CookieConfig{Name: "hrwai_token"}, blacklist)
}

// TestIssuePair_Types 双令牌签发：access 与 refresh 都携带各自的 token_type，且二者不同。
func TestIssuePair_Types(t *testing.T) {
	sess := newTestDualSession(nil)
	access, refresh, err := sess.IssuePair(9, "u9", "hrwai_user")
	if err != nil {
		t.Fatalf("IssuePair 失败: %v", err)
	}
	ac, err := sess.VerifyAccess(access)
	if err != nil || ac.TokenType != TokenTypeAccess || ac.UserID != 9 {
		t.Errorf("access claims 异常: type=%v err=%v", claimsTypeOf(ac), err)
	}
	rc, err := sess.ValidateRefresh(refresh)
	if err != nil || rc.TokenType != TokenTypeRefresh {
		t.Errorf("refresh claims 异常: err=%v", err)
	}
	if access == refresh {
		t.Error("access 与 refresh 不应相同")
	}
}

func claimsTypeOf(c *Claims) string {
	if c == nil {
		return "<nil>"
	}
	return c.TokenType
}

// TestVerifyAccess_RejectsRefresh access 鉴权端点拒绝 refresh 类型令牌。
func TestVerifyAccess_RejectsRefresh(t *testing.T) {
	sess := newTestDualSession(nil)
	_, refresh, _ := sess.IssuePair(1, "u", "hrwai_user")
	if _, err := sess.VerifyAccess(refresh); err == nil {
		t.Error("refresh token 不应通过 VerifyAccess")
	}
}

// TestValidateRefresh_RejectsAccess 刷新端点拒绝 access 类型令牌。
func TestValidateRefresh_RejectsAccess(t *testing.T) {
	sess := newTestDualSession(nil)
	access, _, _ := sess.IssuePair(1, "u", "hrwai_user")
	if _, err := sess.ValidateRefresh(access); err == nil {
		t.Error("access token 不应通过 ValidateRefresh")
	}
}

// TestRefreshRotation_OldReplayRejected 轮换：刷新后旧 refresh 立即失效（重放被拒）。
func TestRefreshRotation_OldReplayRejected(t *testing.T) {
	store := newInmemoryBlacklistStore()
	sess := newTestDualSession(store)
	_, oldRefresh, _ := sess.IssuePair(7, "user7", "hrwai_user")

	// 模拟刷新端点轮换：校验旧 refresh 有效 → 签发新对 → 吊销旧 refresh
	if _, err := sess.ValidateRefresh(oldRefresh); err != nil {
		t.Fatalf("旧 refresh 应有效: %v", err)
	}
	if _, _, err := sess.IssuePair(7, "user7", "hrwai_user"); err != nil {
		t.Fatalf("签发新对失败: %v", err)
	}
	if err := sess.RevokeRefresh(context.Background(), oldRefresh); err != nil {
		t.Fatalf("吊销旧 refresh 失败: %v", err)
	}

	// 已被吊销的旧 refresh 在刷新端点的黑名单检查中被拒
	revoked, _ := sess.IsRevoked(context.Background(), oldRefresh)
	if !revoked {
		t.Error("轮换后旧 refresh 应命中黑名单（防重放）")
	}
}

// TestRevokeRefresh_OnlyRefresh 只允许吊销 refresh；access 传进来静默忽略（不入黑名单）。
func TestRevokeRefresh_OnlyRefresh(t *testing.T) {
	store := newInmemoryBlacklistStore()
	sess := newTestDualSession(store)
	access, refresh, _ := sess.IssuePair(3, "user3", "hrwai_user")

	// 误传 access 给 RevokeRefresh 应静默忽略
	if err := sess.RevokeRefresh(context.Background(), access); err != nil {
		t.Fatalf("RevokeRefresh(access) 应静默成功: %v", err)
	}
	if len(store.m) != 0 {
		t.Errorf("access 不应写入黑名单，实际 %d 条", len(store.m))
	}

	// 正确的 refresh 被吊销
	if err := sess.RevokeRefresh(context.Background(), refresh); err != nil {
		t.Fatalf("RevokeRefresh(refresh) 失败: %v", err)
	}
	if len(store.m) != 1 {
		t.Errorf("应写入 1 条黑名单，实际 %d 条", len(store.m))
	}
}

// TestLogout_RevokesRefresh 登出撤销 refresh：随后的刷新请求被拒。
func TestLogout_RevokesRefresh(t *testing.T) {
	store := newInmemoryBlacklistStore()
	sess := newTestDualSession(store)
	_, refresh, _ := sess.IssuePair(5, "user5", "hrwai_user")

	if err := sess.RevokeRefresh(context.Background(), refresh); err != nil {
		t.Fatalf("登出吊销 refresh 失败: %v", err)
	}
	if revoked, _ := sess.IsRevoked(context.Background(), refresh); !revoked {
		t.Error("登出后 refresh 应被吊销")
	}
}
