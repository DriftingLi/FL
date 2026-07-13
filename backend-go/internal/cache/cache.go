package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// sfGroup 合并相同 key 的并发 loader 调用，防止缓存击穿。
var sfGroup singleflight.Group

// fullKey 拼接带前缀的完整缓存 key。
func fullKey(key string) string {
	if prefix == "" {
		prefix = DefaultKeyPrefix
	}
	return prefix + key
}

// SafeKey 用 ":" 拼接多段 key，并对每段做 URL 编码以避免特殊字符（空格、冒号等）导致 key 解析歧义。
// 示例：SafeKey("dict", "brand", "get", "林德(E16)") → "dict:brand:get:%E6%9E%97%E5%BE%B7%28E16%29"
func SafeKey(parts ...string) string {
	encoded := make([]string, len(parts))
	for i, p := range parts {
		encoded[i] = url.QueryEscape(p)
	}
	return strings.Join(encoded, ":")
}

// Get 从缓存读取字符串值。key 不存在时返回 ("", redis.Nil)。
func Get(ctx context.Context, key string) (string, error) {
	if client == nil {
		return "", fmt.Errorf("redis client 未初始化")
	}
	val, err := client.Get(ctx, fullKey(key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", err
		}
		slog.Warn("Redis Get 失败", "key", key, "error", err)
		return "", err
	}
	return val, nil
}

// Set 写入字符串值到缓存，带 TTL（<=0 使用 DefaultTTL）。
func Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	if client == nil {
		return fmt.Errorf("redis client 未初始化")
	}
	if err := client.Set(ctx, fullKey(key), value, ttl).Err(); err != nil {
		slog.Warn("Redis Set 失败", "key", key, "error", err)
		return err
	}
	return nil
}

// Del 删除一个或多个缓存 key，忽略 redis.Nil（key 不存在不算错误）。
func Del(ctx context.Context, keys ...string) error {
	if client == nil {
		return fmt.Errorf("redis client 未初始化")
	}
	fullKeys := make([]string, len(keys))
	for i, k := range keys {
		fullKeys[i] = fullKey(k)
	}
	if err := client.Del(ctx, fullKeys...).Err(); err != nil {
		slog.Warn("Redis Del 失败", "keys", keys, "error", err)
		return err
	}
	return nil
}

// Exists 检查一个或多个 key 是否存在，返回存在的数量。
func Exists(ctx context.Context, keys ...string) (int64, error) {
	if client == nil {
		return 0, fmt.Errorf("redis client 未初始化")
	}
	fullKeys := make([]string, len(keys))
	for i, k := range keys {
		fullKeys[i] = fullKey(k)
	}
	n, err := client.Exists(ctx, fullKeys...).Result()
	if err != nil {
		slog.Warn("Redis Exists 失败", "keys", keys, "error", err)
		return 0, err
	}
	return n, nil
}

// Expire 为指定 key 设置过期时间。
func Expire(ctx context.Context, key string, ttl time.Duration) error {
	if client == nil {
		return fmt.Errorf("redis client 未初始化")
	}
	if err := client.Expire(ctx, fullKey(key), ttl).Err(); err != nil {
		slog.Warn("Redis Expire 失败", "key", key, "error", err)
		return err
	}
	return nil
}

// SetJSON 将 value 序列化为 JSON 后写入缓存，带 TTL。
func SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("JSON 序列化失败: %w", err)
	}
	return Set(ctx, key, string(data), ttl)
}

// GetJSON 从缓存读取并反序列化 JSON 到 dest（必须传指针）。
// key 不存在时返回 redis.Nil。
func GetJSON(ctx context.Context, key string, dest interface{}) error {
	val, err := Get(ctx, key)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(val), dest); err != nil {
		slog.Warn("Redis 缓存数据 JSON 反序列化失败", "key", key, "error", err)
		return fmt.Errorf("JSON 反序列化失败: %w", err)
	}
	return nil
}

// GetOrSetJSON 实现 Cache-Aside 模式：
//  1. 先查缓存，命中则反序列化到 dest 并返回 nil
//  2. 缓存 miss 则调用 loader() 查 DB/执行计算
//  3. loader 成功时写回缓存（TTL），失败时只返回 error 不回写
//  4. 使用 singleflight 合并相同 key 的并发 loader 调用，防止缓存击穿
//
// dest 必须为指针类型，用于接收反序列化结果。
func GetOrSetJSON(ctx context.Context, key string, ttl time.Duration, dest interface{}, loader func() (interface{}, error)) error {
	// 1. 查缓存
	err := GetJSON(ctx, key, dest)
	if err == nil {
		return nil // 命中
	}
	if !errors.Is(err, redis.Nil) {
		// Redis 异常（网络/超时等）→ 降级查 DB
		slog.Warn("Redis GetJSON 异常，降级查 DB", "key", key, "error", err)
	}

	// 2. 缓存 miss → singleflight 合并并发 loader
	val, sfErr, _ := sfGroup.Do(key, func() (interface{}, error) {
		// 双重检查：可能在等待期间已被其他请求填充
		if e := GetJSON(ctx, key, dest); e == nil {
			return nil, nil
		}
		data, e := loader()
		if e != nil {
			return nil, e
		}
		// 回写缓存（忽略回写失败，不影响返回业务数据）
		if setErr := SetJSON(ctx, key, data, ttl); setErr != nil {
			slog.Warn("回写缓存失败", "key", key, "error", setErr)
		}
		return data, nil
	})
	if sfErr != nil {
		return sfErr
	}
	// singleflight 返回 nil 表示 dest 已由双重检查填充
	if val == nil {
		return nil
	}
	// 将 loader 结果写入 dest
	data, _ := json.Marshal(val)
	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("loader 结果序列化异常: %w", err)
	}
	return nil
}

// InvalidatePattern 使用 SCAN 遍历匹配 pattern 的 key 并用 Pipeline 批量 DEL。
// pattern 格式与 Redis KEYS 命令一致（如 "course:*"），内部自动拼接前缀，调用方只需传业务 pattern。
func InvalidatePattern(ctx context.Context, pattern string) error {
	if client == nil {
		return fmt.Errorf("redis client 未初始化")
	}
	fullPattern := fullKey(pattern)

	var cursor uint64
	var deleted int
	for {
		keys, nextCursor, err := client.Scan(ctx, cursor, fullPattern, 100).Result()
		if err != nil {
			slog.Warn("Redis SCAN 失败", "pattern", pattern, "error", err)
			return err
		}
		if len(keys) > 0 {
			// 用 Pipeline 批量 DEL，减少网络往返
			pipe := client.Pipeline()
			pipe.Del(ctx, keys...)
			if _, dErr := pipe.Exec(ctx); dErr != nil {
				slog.Warn("InvalidatePattern 批量 DEL 失败", "keys_count", len(keys), "error", dErr)
				// 继续处理下一批，不中断
			} else {
				deleted += len(keys)
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	if deleted > 0 {
		slog.Info("InvalidatePattern 完成", "pattern", pattern, "deleted", deleted)
	}
	return nil
}
