package cache

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"forklift-training/internal/config"
)

// testPrefix 测试用 key 前缀，避免与业务 key 冲突。
const testPrefix = "fl:test:"

// setupTestRedis 初始化测试用 Redis 连接。
// 若 REDIS_ADDR 未配置则跳过测试。
func setupTestRedis(t *testing.T) {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	cfg := config.RedisConfig{
		Addr:         addr,
		Password:     os.Getenv("REDIS_PASSWORD"),
		DB:           0,
		PoolSize:     5,
		MinIdleConns: 1,
		MaxRetries:   3,
		Prefix:       testPrefix,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolTimeout:  3 * time.Second,
		IdleTimeout:  2 * time.Minute,
	}
	if _, err := InitRedis(cfg); err != nil {
		t.Skipf("Redis 不可用，跳过测试: %v", err)
	}
}

// cleanupTestKeys 清理测试产生的 key。
func cleanupTestKeys(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	if err := InvalidatePattern(ctx, "*"); err != nil {
		t.Logf("清理测试 key 失败: %v", err)
	}
}

func TestMain(m *testing.M) {
	// 测试结束后清理
	code := m.Run()
	if client != nil {
		_ = InvalidatePattern(context.Background(), "*")
		CloseRedis(client)
	}
	os.Exit(code)
}

func TestSetAndGet(t *testing.T) {
	setupTestRedis(t)
	defer cleanupTestKeys(t)

	ctx := context.Background()
	key := "setget:key"
	val := "hello-redis"

	if err := Set(ctx, key, val, time.Minute); err != nil {
		t.Fatalf("Set 失败: %v", err)
	}

	got, err := Get(ctx, key)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if got != val {
		t.Errorf("Get 返回值不匹配: got %q, want %q", got, val)
	}
}

func TestGet_Miss(t *testing.T) {
	setupTestRedis(t)
	defer cleanupTestKeys(t)

	ctx := context.Background()
	_, err := Get(ctx, "nonexistent:key")
	if err == nil {
		t.Error("期望返回 redis.Nil，但返回 nil")
	}
}

func TestSetJSONAndGetJSON(t *testing.T) {
	setupTestRedis(t)
	defer cleanupTestKeys(t)

	ctx := context.Background()
	key := "json:item"
	type Item struct {
		Name  string
		Price float64
	}
	original := Item{Name: "叉车A", Price: 99.5}

	if err := SetJSON(ctx, key, original, time.Minute); err != nil {
		t.Fatalf("SetJSON 失败: %v", err)
	}

	var got Item
	if err := GetJSON(ctx, key, &got); err != nil {
		t.Fatalf("GetJSON 失败: %v", err)
	}
	if got.Name != original.Name || got.Price != original.Price {
		t.Errorf("GetJSON 结果不匹配: got %+v, want %+v", got, original)
	}
}

func TestGetOrSetJSON_CacheMiss(t *testing.T) {
	setupTestRedis(t)
	defer cleanupTestKeys(t)

	ctx := context.Background()
	key := "gorset:miss"
	type Data struct {
		Count int
	}

	var callCount int32
	loader := func() (any, error) {
		atomic.AddInt32(&callCount, 1)
		return Data{Count: 42}, nil
	}

	// 第一次：缓存 miss，调用 loader
	var result Data
	if err := GetOrSetJSON(ctx, key, time.Minute, &result, loader); err != nil {
		t.Fatalf("GetOrSetJSON 失败: %v", err)
	}
	if result.Count != 42 {
		t.Errorf("结果不匹配: got %d, want 42", result.Count)
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("loader 调用次数: got %d, want 1", callCount)
	}

	// 第二次：缓存命中，不调用 loader
	var result2 Data
	if err := GetOrSetJSON(ctx, key, time.Minute, &result2, loader); err != nil {
		t.Fatalf("GetOrSetJSON 第二次失败: %v", err)
	}
	if result2.Count != 42 {
		t.Errorf("第二次结果不匹配: got %d, want 42", result2.Count)
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("第二次 loader 应不调用，但 callCount=%d", callCount)
	}
}

func TestGetOrSetJSON_Singleflight(t *testing.T) {
	setupTestRedis(t)
	defer cleanupTestKeys(t)

	ctx := context.Background()
	key := "gorset:sf"
	type Data struct {
		Value int
	}

	var callCount int32
	loader := func() (any, error) {	
		atomic.AddInt32(&callCount, 1)
		time.Sleep(100 * time.Millisecond) // 模拟慢查询，让并发请求阻塞
		return Data{Value: 100}, nil
	}

	// 先清除可能残留的 key
	_ = Del(ctx, key)

	// 100 个并发请求同一 miss key
	var wg sync.WaitGroup
	results := make([]Data, 100)
	errs := make([]error, 100)
	for i := range 100 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = GetOrSetJSON(ctx, key, time.Minute, &results[idx], loader)
		}(i)
	}
	wg.Wait()

	// 验证所有请求都成功
	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d 出错: %v", i, err)
		}
		if results[i].Value != 100 {
			t.Errorf("goroutine %d 结果不匹配: got %d, want 100", i, results[i].Value)
		}
	}

	// singleflight 应合并并发请求，loader 调用次数应远小于 100
	// 注意：由于双重检查，loader 可能被调用 1 次（首个请求）+ 少量双重检查命中
	got := atomic.LoadInt32(&callCount)
	if got > 5 {
		t.Errorf("loader 调用次数 %d 过多，期望 singleflight 合并到 <=5 次", got)
	}
	t.Logf("loader 调用次数: %d（singleflight 合并效果）", got)
}

func TestInvalidatePattern(t *testing.T) {
	setupTestRedis(t)
	defer cleanupTestKeys(t)

	ctx := context.Background()

	// 写入 10 个匹配 pattern 的 key
	for i := range 10 {
		key := "inval:item:" + string(rune('a'+i))
		if err := Set(ctx, key, "v", time.Minute); err != nil {
			t.Fatalf("Set 失败: %v", err)
		}
	}
	// 写入 1 个不匹配的 key
	if err := Set(ctx, "other:key", "v", time.Minute); err != nil {
		t.Fatalf("Set other:key 失败: %v", err)
	}

	// 失效匹配 "inval:*" 的 key
	if err := InvalidatePattern(ctx, "inval:*"); err != nil {
		t.Fatalf("InvalidatePattern 失败: %v", err)
	}

	// 验证匹配的 key 已删除
	for i := range 10 {
		key := "inval:item:" + string(rune('a'+i))
		if _, err := Get(ctx, key); err == nil {
			t.Errorf("key %s 应已删除但仍存在", key)
		}
	}

	// 验证不匹配的 key 仍存在
	if _, err := Get(ctx, "other:key"); err != nil {
		t.Errorf("other:key 不应被删除: %v", err)
	}
}

func TestDel(t *testing.T) {
	setupTestRedis(t)
	defer cleanupTestKeys(t)

	ctx := context.Background()
	if err := Set(ctx, "del:key1", "v1", time.Minute); err != nil {
		t.Fatalf("Set 失败: %v", err)
	}
	if err := Set(ctx, "del:key2", "v2", time.Minute); err != nil {
		t.Fatalf("Set 失败: %v", err)
	}

	if err := Del(ctx, "del:key1", "del:key2"); err != nil {
		t.Fatalf("Del 失败: %v", err)
	}

	if _, err := Get(ctx, "del:key1"); err == nil {
		t.Error("del:key1 应已删除")
	}
	if _, err := Get(ctx, "del:key2"); err == nil {
		t.Error("del:key2 应已删除")
	}
}

func TestExists(t *testing.T) {
	setupTestRedis(t)
	defer cleanupTestKeys(t)

	ctx := context.Background()
	if err := Set(ctx, "exists:key", "v", time.Minute); err != nil {
		t.Fatalf("Set 失败: %v", err)
	}

	n, err := Exists(ctx, "exists:key")
	if err != nil {
		t.Fatalf("Exists 失败: %v", err)
	}
	if n != 1 {
		t.Errorf("Exists 返回 %d, want 1", n)
	}

	n, err = Exists(ctx, "exists:key", "nonexistent")
	if err != nil {
		t.Fatalf("Exists 失败: %v", err)
	}
	if n != 1 {
		t.Errorf("Exists 返回 %d, want 1", n)
	}
}

func TestExpire(t *testing.T) {
	setupTestRedis(t)
	defer cleanupTestKeys(t)

	ctx := context.Background()
	if err := Set(ctx, "expire:key", "v", time.Minute); err != nil {
		t.Fatalf("Set 失败: %v", err)
	}

	if err := Expire(ctx, "expire:key", 2*time.Second); err != nil {
		t.Fatalf("Expire 失败: %v", err)
	}

	// 等待过期
	time.Sleep(2200 * time.Millisecond)

	if _, err := Get(ctx, "expire:key"); err == nil {
		t.Error("key 应已过期删除")
	}
}

func TestSafeKey(t *testing.T) {
	tests := []struct {
		parts []string
		want  string
	}{
		{[]string{"dict", "brand", "list"}, "dict:brand:list"},
		{[]string{"dict", "brand", "get", "林德"}, "dict:brand:get:%E6%9E%97%E5%BE%B7"},
		{[]string{"a", "b:c", "d"}, "a:b%3Ac:d"},
		{[]string{}, ""},
		{[]string{"single"}, "single"},
	}
	for _, tt := range tests {
		got := SafeKey(tt.parts...)
		if got != tt.want {
			t.Errorf("SafeKey(%v) = %q, want %q", tt.parts, got, tt.want)
		}
	}
}

func TestPing(t *testing.T) {
	setupTestRedis(t)
	defer cleanupTestKeys(t)

	ctx := context.Background()
	if err := Ping(ctx); err != nil {
		t.Errorf("Ping 失败: %v", err)
	}
}
