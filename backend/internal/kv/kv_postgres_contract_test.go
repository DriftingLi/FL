package kv

import (
	"context"
	"sync"
	"testing"
	"time"

	"forklift-training/internal/testutil"
)

// 并发抢占同一 key 恰有一方成功 —— 会话轮换原子性的落地判据（SETNX 语义）。
// 只在真实 PG 上跑：ON CONFLICT 的并发语义是这里要证明的东西，SQLite 单写者证明不了。
func TestPutIfAbsentConcurrent(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	s := NewStore(db)
	ctx := context.Background()

	const n = 8
	var wg sync.WaitGroup
	wins := make(chan bool, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := s.PutIfAbsent(ctx, "race-key", "1", time.Minute)
			if err != nil {
				t.Errorf("PutIfAbsent 出错: %v", err)
			}
			wins <- ok
		}()
	}
	wg.Wait()
	close(wins)

	got := 0
	for ok := range wins {
		if ok {
			got++
		}
	}
	if got != 1 {
		t.Fatalf("并发抢占同一 key 应恰有一方成功, got %d", got)
	}
}
