package points

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/clock"
	"forklift-training/internal/testutil"
)

// advisoryLockCount 查某个 advisory 锁键当前被持有的条数。
// advisory lock 在 pg_locks 里按 classid（高 32 位）+ objid（低 32 位）表达。
func advisoryLockCount(t *testing.T, svc *Service, key string) int {
	t.Helper()
	id := advisoryLockID(key)
	var n int
	if err := svc.db.Raw(
		"SELECT count(*) FROM pg_locks WHERE locktype = 'advisory' AND classid = ? AND objid = ?",
		uint32(uint64(id)>>32), uint32(uint64(id)&0xFFFFFFFF),
	).Scan(&n).Error; err != nil {
		t.Fatalf("查 pg_locks 失败: %v", err)
	}
	return n
}

// 同键互斥：抢到之后第二把拿不到；释放之后又能拿到。
// 这是「锁只是进程级双写护栏」里唯一可观测的那一半 —— 最终裁决仍由唯一索引承担（ADR-0023）。
func TestTryLockMutualExclusion(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	svc := NewService(db, zap.NewNop(), clock.Real(), nil)
	ctx := context.Background()

	const key = "points:grant:test:advisory"

	rel1 := svc.tryLock(ctx, key)
	if got := advisoryLockCount(t, svc, key); got != 1 {
		t.Fatalf("抢到后应持有 1 把锁, got %d", got)
	}

	rel2 := svc.tryLock(ctx, key) // 同键，应抢不到
	if got := advisoryLockCount(t, svc, key); got != 1 {
		t.Fatalf("同键第二把不应成功, got %d", got)
	}

	rel1()
	if got := advisoryLockCount(t, svc, key); got != 0 {
		t.Fatalf("释放后应 0 把锁, got %d", got)
	}

	rel2() // 空 release，必须是无害的
	rel3 := svc.tryLock(ctx, key)
	if got := advisoryLockCount(t, svc, key); got != 1 {
		t.Fatalf("释放后应能重新抢到, got %d", got)
	}
	rel3()
}

// 非 PG 方言（SQLite 契约测试库）退化为不加锁：不 panic、不阻断主流程。
func TestTryLockDegradesOnNonPostgres(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewService(db, zap.NewNop(), clock.Real(), nil)
	release := svc.tryLock(context.Background(), "points:grant:test:sqlite")
	release() // 必须是可调用且无害的
}

// 锁键到 bigint 的映射必须稳定（同一 key 恒等），否则释放会落到另一把锁上。
func TestAdvisoryLockIDStable(t *testing.T) {
	a, b := advisoryLockID("points:grant:1:daily_login"), advisoryLockID("points:grant:1:daily_login")
	if a != b {
		t.Fatalf("同一 key 的锁 id 必须稳定: %d != %d", a, b)
	}
	if advisoryLockID("points:grant:1:daily_login") == advisoryLockID("points:grant:1:growth_post") {
		t.Fatal("不同 key 不应映射到同一个锁 id（本用例的取值下）")
	}
}
