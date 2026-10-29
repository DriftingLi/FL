package kv

import (
	"context"
	"errors"
	"testing"
	"time"

	"forklift-training/internal/testutil"
)

// newTestStore 内存 SQLite 上的 kv.Store（模型经 testutil 的 AutoMigrate 建表）。
func newTestStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(testutil.NewMemoryDB(t))
}

func TestSetGet(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.Set(ctx, "k", "v", time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := s.Get(ctx, "k")
	if err != nil || got != "v" {
		t.Fatalf("get = (%q, %v), want (v, nil)", got, err)
	}
}

func TestGetMissing(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Get(context.Background(), "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("未命中应返回 ErrNotFound, got %v", err)
	}
}

func TestSetOverwritesValueAndTTL(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.WithClock(func() time.Time { return now })

	if err := s.Set(ctx, "k", "first", time.Minute); err != nil {
		t.Fatalf("set1: %v", err)
	}
	if err := s.Set(ctx, "k", "second", time.Hour); err != nil {
		t.Fatalf("set2: %v", err)
	}
	got, err := s.Get(ctx, "k")
	if err != nil || got != "second" {
		t.Fatalf("覆盖写后应读到 second, got (%q, %v)", got, err)
	}
	// TTL 也被刷新：31 分钟后仍在（若沿用 1 分钟早已过期）
	now = now.Add(31 * time.Minute)
	if got, err = s.Get(ctx, "k"); err != nil || got != "second" {
		t.Fatalf("TTL 应被覆盖为 1 小时, got (%q, %v)", got, err)
	}
}

func TestExpiredIsNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.WithClock(func() time.Time { return now })

	if err := s.Set(ctx, "k", "v", time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	now = now.Add(61 * time.Second)
	if _, err := s.Get(ctx, "k"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("过期后应返回 ErrNotFound, got %v", err)
	}
}

func TestPutIfAbsent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	ok, err := s.PutIfAbsent(ctx, "k", "1", time.Minute)
	if err != nil || !ok {
		t.Fatalf("首次抢占应成功, got (%v, %v)", ok, err)
	}
	ok, err = s.PutIfAbsent(ctx, "k", "2", time.Minute)
	if err != nil || ok {
		t.Fatalf("已存在时不应抢占成功, got (%v, %v)", ok, err)
	}
	if v, _ := s.Get(ctx, "k"); v != "1" {
		t.Fatalf("被拒的写入不应覆盖原值, got %q", v)
	}
}

// 过期残留不得被当成「已被占用」：这是 PutIfAbsent 里先清过期行的存在理由。
func TestPutIfAbsentTreatsExpiredAsAbsent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.WithClock(func() time.Time { return now })

	if ok, err := s.PutIfAbsent(ctx, "k", "old", time.Minute); err != nil || !ok {
		t.Fatalf("首次抢占失败: (%v, %v)", ok, err)
	}
	now = now.Add(2 * time.Minute)
	if ok, err := s.PutIfAbsent(ctx, "k", "new", time.Minute); err != nil || !ok {
		t.Fatalf("过期行应等价于不存在, got (%v, %v)", ok, err)
	}
	if v, _ := s.Get(ctx, "k"); v != "new" {
		t.Fatalf("应写入新值, got %q", v)
	}
}

func TestDel(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_ = s.Set(ctx, "a", "1", time.Minute)
	_ = s.Set(ctx, "b", "2", time.Minute)
	if err := s.Del(ctx, "a", "b"); err != nil {
		t.Fatalf("del: %v", err)
	}
	for _, k := range []string{"a", "b"} {
		if _, err := s.Get(ctx, k); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s 应已删除", k)
		}
	}
	// 删不存在的 key 不算错误
	if err := s.Del(ctx, "ghost"); err != nil {
		t.Fatalf("删除不存在的 key 不应报错: %v", err)
	}
}

func TestSweepOnlyRemovesExpired(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.WithClock(func() time.Time { return now })

	_ = s.Set(ctx, "gone", "1", time.Minute)
	_ = s.Set(ctx, "stay", "2", time.Hour)
	now = now.Add(5 * time.Minute)

	n, err := s.Sweep(ctx, 0)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if n != 1 {
		t.Fatalf("应只清掉 1 条过期行, got %d", n)
	}
	if _, err := s.Get(ctx, "stay"); err != nil {
		t.Fatalf("未过期行不应被清掉: %v", err)
	}
}

func TestSweepLimit(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.WithClock(func() time.Time { return now })
	for i := 0; i < 5; i++ {
		_ = s.Set(ctx, string(rune('a'+i)), "x", time.Minute)
	}
	now = now.Add(time.Hour)
	n, err := s.Sweep(ctx, 2)
	if err != nil || n != 2 {
		t.Fatalf("limit=2 应清 2 条, got (%d, %v)", n, err)
	}
}

// SafeKey 的形态与 Redis 时代逐字一致：线上存量 key 的语义不因存储换代而变。
func TestSafeKey(t *testing.T) {
	cases := map[string]string{
		SafeKey("email_code", "register", "a@b.com"):   "email_code:register:a%40b.com",
		SafeKey("captcha", "x y"):                      "captcha:x+y",
		SafeKey("jwt:pwd_revoked", "hrwai_user", "42"): "jwt%3Apwd_revoked:hrwai_user:42",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("SafeKey 形态变化: got %q want %q", got, want)
		}
	}
}
