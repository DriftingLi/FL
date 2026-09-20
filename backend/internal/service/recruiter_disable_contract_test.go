// 禁用招聘者 = 全会话吊销（ADR-0060 票2 / spec #1201 场景 29）。
// seam：service 的具名动作 + security.Session 的轮换结果。
// 只改状态列不构成「停用真实生效」——手上仍持 refresh 的会话能继续换新 access。
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// valueBlacklist 保留写入值并实现 SETNX 语义（吊销标记的**值**是时间戳，
// 只记 key 的 spyBlacklist 会把标记读成 1970 年，测不出「已吊销」）。
type valueBlacklist struct{ m map[string]string }

func (s *valueBlacklist) Get(_ context.Context, k string) (string, error) {
	if v, ok := s.m[k]; ok {
		return v, nil
	}
	return "", errors.New("not found")
}

func (s *valueBlacklist) Set(_ context.Context, k, v string, _ time.Duration) error {
	s.m[k] = v
	return nil
}

func (s *valueBlacklist) PutIfAbsent(_ context.Context, k, v string, _ time.Duration) (bool, error) {
	if _, ok := s.m[k]; ok {
		return false, nil
	}
	s.m[k] = v
	return true, nil
}

func newRecruiterFixture(t *testing.T) (*AuthService, *security.Session, int) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	sess := security.NewSessionWithBlacklistAndRefresh("test-secret", time.Hour, 7*time.Hour,
		security.CookieConfig{Name: "recruiter_token"}, &valueBlacklist{m: map[string]string{}})
	svc := NewAuthService(db, sess, NewForumCounter(), "admin", "tutor", "student", zap.NewNop())
	r := model.RecruiterUser{Username: "rec1", Password: "x", CompanyName: "禁用测试企业", Status: 1}
	if err := db.Create(&r).Error; err != nil {
		t.Fatalf("播种招聘者失败: %v", err)
	}
	return svc, sess, r.ID
}

func TestToggleRecruiterStatus_禁用即吊销全会话(t *testing.T) {
	svc, sess, id := newRecruiterFixture(t)
	ctx := context.Background()

	_, refresh, err := sess.IssuePair(id, "rec1", "recruiter")
	if err != nil {
		t.Fatalf("签发 refresh 失败: %v", err)
	}
	// 对照组：禁用前可正常轮换（被拒的原因必须是禁用，不是令牌本身无效）
	if _, _, err := sess.RotateRefresh(ctx, refresh); err != nil {
		t.Fatalf("禁用前应可轮换: %v", err)
	}

	next, err := svc.ToggleRecruiterStatus(ctx, id)
	if err != nil {
		t.Fatalf("切换状态失败: %v", err)
	}
	if next != 0 {
		t.Fatalf("启用中的招聘者应被禁用，实际 status=%d", next)
	}
	// 该身份手上剩下的这枚（禁用前一刻轮换出来的）一律换不出新令牌
	if _, _, err := sess.RotateRefresh(ctx, refresh); !errors.Is(err, security.ErrInvalidRefresh) {
		t.Errorf("禁用后 refresh 应报 ErrInvalidRefresh（端点侧映射 401），实际: %v", err)
	}
}

// 重新启用不得顺手把吊销标记清掉——标记 TTL 到期前该身份必须重新登录。
func TestToggleRecruiterStatus_重新启用不撤销吊销(t *testing.T) {
	svc, sess, id := newRecruiterFixture(t)
	ctx := context.Background()

	if _, err := svc.ToggleRecruiterStatus(ctx, id); err != nil {
		t.Fatalf("禁用失败: %v", err)
	}
	_, staleRefresh, _ := sess.IssuePair(id, "rec1", "recruiter")
	if _, err := svc.ToggleRecruiterStatus(ctx, id); err != nil {
		t.Fatalf("重新启用失败: %v", err)
	}
	if _, _, err := sess.RotateRefresh(ctx, staleRefresh); !errors.Is(err, security.ErrInvalidRefresh) {
		t.Errorf("启用不该撤销吊销标记，旧 refresh 仍应被拒，实际: %v", err)
	}
}
