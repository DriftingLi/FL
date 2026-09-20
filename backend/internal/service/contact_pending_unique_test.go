// 唯一性判据与落态单点的服务层锁（ADR-0061 §2 / #1197 复核后补）。
//
// 为什么要单独测这两个内部函数：`Create` 的偏索引回读分支只有在**真并发**下才会走到，
// 契约测试锁不住它。而复核时它就带着一个静默缺陷——`Count` 少了 `.Model()`，gorm 会报
// 「Table not set」，调用方一咽掉错误就把「查不动」当成「没有 pending」。
// 本文件直接问这两个函数，让这类形状错误不依赖调度也能红。
package service

import (
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func seedPending(t *testing.T, db *gorm.DB, recruiterID, studentUserID int, window time.Time) {
	t.Helper()
	row := model.ContactRequest{
		RecruiterID: recruiterID, StudentUserID: studentUserID, Message: "夹具",
		Status: string(ContactGrantPending), CreatedAt: window.Add(-contactDecisionWindow),
		UpdatedAt: window.Add(-contactDecisionWindow), ExpiresAt: &window,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed pending 失败: %v", err)
	}
}

func TestPendingCountForAnswers(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewContactService(db, zap.NewNop(), nil, nil)
	now := time.Now()

	seedPending(t, db, 7, 21, now.Add(time.Hour))    // 未闭窗
	seedPending(t, db, 7, 22, now.Add(-time.Minute)) // 已闭窗

	if cnt, err := svc.pendingCountFor(7, 21); err != nil || cnt != 1 {
		t.Fatalf("有 pending 的一对应答 1，实际 cnt=%d err=%v", cnt, err)
	}
	if cnt, err := svc.pendingCountFor(7, 22); err != nil || cnt != 1 {
		t.Fatalf("已闭窗的 pending 仍计入唯一性（落态由 expireClosed 负责，两件事不混）：cnt=%d err=%v", cnt, err)
	}
	if cnt, err := svc.pendingCountFor(7, 99); err != nil || cnt != 0 {
		t.Fatalf("无 pending 的一对应答 0，实际 cnt=%d err=%v", cnt, err)
	}
}

func TestExpireClosedScopes(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewContactService(db, zap.NewNop(), nil, nil)
	now := time.Now()

	seedPending(t, db, 7, 21, now.Add(-time.Minute))
	seedPending(t, db, 7, 22, now.Add(-time.Minute))
	seedPending(t, db, 8, 23, now.Add(-time.Minute))
	status := func(recruiterID, studentUserID int) string {
		t.Helper()
		var row model.ContactRequest
		if err := db.Where("recruiter_id = ? AND student_user_id = ?", recruiterID, studentUserID).
			First(&row).Error; err != nil {
			t.Fatalf("find %d/%d: %v", recruiterID, studentUserID, err)
		}
		return row.Status
	}

	// 定向：只收这一对，别的行不动。
	if n, err := svc.expireClosed(now, svc.pairScope(7, 21)); err != nil || n != 1 {
		t.Fatalf("定向落态应落 1 条，实际 n=%d err=%v", n, err)
	}
	if got := status(7, 21); got != string(ContactGrantExpired) {
		t.Fatalf("目标对应落为 expired，实际 %s", got)
	}
	if got := status(7, 22); got != string(ContactGrantPending) {
		t.Fatalf("同企业的另一对被误落态：%s", got)
	}
	// 全表（scope=nil，守护的形状）：剩下的两条都该落。
	if n, err := svc.ExpirePending(now); err != nil || n != 2 {
		t.Fatalf("全表收敛应落 2 条，实际 n=%d err=%v", n, err)
	}
	if got := status(7, 22); got != string(ContactGrantExpired) {
		t.Fatalf("全表收敛后仍挂着 pending：%s", got)
	}
	// 幂等：没有可落的行时报 0，不报错、不倒退状态。
	if n, err := svc.ExpirePending(now); err != nil || n != 0 {
		t.Fatalf("重复收敛应 0 条，实际 n=%d err=%v", n, err)
	}
}
