// 口令写面的结果形状（ADR-0064 决策 6 / D5）：「口令已成」与「吊销可选失败」是两格，
// 且第二格**不得回退**第一格。
//
// 为什么值得单独立锁：这两件事在旧签名上写成 `(revokeErr, err)` —— 两个 error、顺序反直觉，
// caller 记错顺序就把「吊销失败」当成「口令没改成」（或反之）。升成 PasswordWriteResult 之后，
// 本文件钉住的是语义那一半：吊销存储坏掉时口令必须已经落库、入口方法必须仍返回成功，
// 缺口如实出现在 RevokeErr 上由 caller 记日志。
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// brokenRevokeStore 一切写都失败、读一律「查得空」：只用来制造吊销标记写失败。
type brokenRevokeStore struct{ setErr error }

func (brokenRevokeStore) Get(context.Context, string) (string, error) { return "", nil }
func (b brokenRevokeStore) Set(context.Context, string, string, time.Duration) error {
	return b.setErr
}
func (b brokenRevokeStore) PutIfAbsent(context.Context, string, string, time.Duration) (bool, error) {
	return false, b.setErr
}

func newBrokenRevokeAuth(t *testing.T, db *gorm.DB) *AuthService {
	t.Helper()
	sess := security.NewSessionWithBlacklistAndRefresh(
		"pwd-result-test-secret", time.Hour, 7*24*time.Hour,
		security.CookieConfig{Name: "hrwai_token"}, brokenRevokeStore{setErr: errors.New("redis down")})
	return NewAuthService(db, sess, nil, "", "", "", zap.NewNop())
}

// TestPasswordWriteResultKeepsPasswordOnRevokeFailure 吊销写失败 ⇒ 口令照样生效、
// Applied() 为真、缺口如实出现在 RevokeErr（尽力而为族，不升级为整体失败）。
func TestPasswordWriteResultKeepsPasswordOnRevokeFailure(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	old, _ := HashPassword("oldpass123")
	stu := testutil.SeedStudent(t, db, "pwdres_stu", old)
	svc := newBrokenRevokeAuth(t, db)

	res := svc.SetNewPassword(context.Background(), stu.ID, "newpass123")
	if !res.Applied() {
		t.Fatalf("吊销写失败不得让口令判成未落成，实际 Err=%v", res.Err)
	}
	if res.RevokeErr == nil {
		t.Fatal("吊销存储的失败必须如实交出（RevokeErr 为空 = 谎报「已吊销」）")
	}
	var stored model.HrwaiUser
	if err := db.First(&stored, stu.ID).Error; err != nil {
		t.Fatalf("回读学员行失败: %v", err)
	}
	if !VerifyPassword("newpass123", stored.Password) {
		t.Fatal("口令没落库 ⇒ 本测试要钉的「口令已成」这一半不成立")
	}
	// 入口方法的表态：同族尽力而为 ⇒ 对 caller 返回 nil（用户找不回账号才是更坏的后果）。
	if err := svc.UpdatePassword(context.Background(), stu.ID, "another456"); err != nil {
		t.Fatalf("UpdatePassword 不应因吊销失败而拒绝改密，实际 %v", err)
	}
}

// TestPasswordWriteResultErrIsExclusive 口令没落成时不得同时报吊销失败：
// 两格互斥是结果类型的不变式（否则 caller 无法判断该不该记「已改密但没吊销」这条日志）。
func TestPasswordWriteResultErrIsExclusive(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	old, _ := HashPassword("oldpass123")
	stu := testutil.SeedStudent(t, db, "pwdres_excl", old)
	svc := newBrokenRevokeAuth(t, db)

	tooShort := svc.SetNewPassword(context.Background(), stu.ID, "123")
	if tooShort.Applied() || tooShort.RevokeErr != nil {
		t.Fatalf("长度非法应只报 Err，实际 Err=%v RevokeErr=%v", tooShort.Err, tooShort.RevokeErr)
	}
	missing := svc.SetNewPassword(context.Background(), 987654, "validpass123")
	if !errors.Is(missing.Err, ErrHrwaiUserNotFound) || missing.RevokeErr != nil {
		t.Fatalf("主体不存在应报该哨兵且不带 RevokeErr，实际 Err=%v RevokeErr=%v", missing.Err, missing.RevokeErr)
	}
	var still model.HrwaiUser
	if err := db.First(&still, stu.ID).Error; err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	if !VerifyPassword("oldpass123", still.Password) {
		t.Fatal("非法入参的这一次把口令改掉了 ⇒ 「没落成」那一格不可信")
	}
}
