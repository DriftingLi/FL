// Package kv 提供带 TTL 的临时键值存储 —— #2109 阶段 B 里 Redis 的落点。
//
// 三类消费方共用它：验证码（auth.AuthCodeStore）、图形码（captcha.Store）、
// JWT refresh 黑名单与身份吊销标记（security.BlacklistStore）。三者在 Redis 时代
// 就是同一个 keyspace 上的一套 get/set/setnx，所以这里也只有一个实现。
//
// 设计要点：
//   - 过期一律按**应用时钟**判（读路径带 expires_at > ?），清扫滞后也读不到过期值；
//   - 写用 upsert、抢占用 INSERT ... ON CONFLICT DO NOTHING（SETNX 语义，RowsAffected 即裁决）；
//   - key 形态与 Redis 时代逐字一致（SafeKey 原样搬过来），线上存量语义不变。
package kv

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"forklift-training/internal/model"
)

// ErrNotFound key 不存在或已过期。
var ErrNotFound = errors.New("kv: key not found")

// DefaultTTL 调用方没给 TTL 时的兜底（沿用 Redis 时代的 10 分钟）。
const DefaultTTL = 10 * time.Minute

// Store 基于 ephemeral_kv 的 KV 存储。
type Store struct {
	db  *gorm.DB
	now func() time.Time
}

// NewStore 构造存储（时钟默认 time.Now）。
func NewStore(db *gorm.DB) *Store { return &Store{db: db, now: time.Now} }

// WithClock 注入时钟，供测试复现过期（返回自身，便于链式构造）。
func (s *Store) WithClock(now func() time.Time) *Store {
	if now != nil {
		s.now = now
	}
	return s
}

// SafeKey 用 ":" 拼接多段 key，并对每段做 URL 编码，避免空格 / 冒号等字符让 key 产生歧义。
// 形如 SafeKey("captcha", id) → "captcha:<encoded>"；与 Redis 时代的 cache.SafeKey 逐字一致。
func SafeKey(parts ...string) string {
	encoded := make([]string, len(parts))
	for i, p := range parts {
		encoded[i] = url.QueryEscape(p)
	}
	return strings.Join(encoded, ":")
}

// Get 读取未过期的值；不存在或已过期返回 ErrNotFound。
func (s *Store) Get(ctx context.Context, key string) (string, error) {
	var row model.EphemeralKV
	err := s.db.WithContext(ctx).
		Where("cache_key = ? AND expires_at > ?", key, s.now()).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("kv get: %w", err)
	}
	return row.CacheValue, nil
}

// Set 写入并覆盖（upsert），ttl<=0 用 DefaultTTL。
func (s *Store) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	row := model.EphemeralKV{CacheKey: key, CacheValue: value, ExpiresAt: s.now().Add(normalizeTTL(ttl))}
	err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "cache_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"cache_value", "expires_at"}),
	}).Create(&row).Error
	if err != nil {
		return fmt.Errorf("kv set: %w", err)
	}
	return nil
}

// Del 删除一个或多个 key（不存在不算错误）。
func (s *Store) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	if err := s.db.WithContext(ctx).
		Where("cache_key IN ?", keys).
		Delete(&model.EphemeralKV{}).Error; err != nil {
		return fmt.Errorf("kv del: %w", err)
	}
	return nil
}

// PutIfAbsent 仅在 key 不存在时写入（SETNX 语义）：写入成功返回 true，已存在返回 false。
// 并发调用同一 key 恰有一方为 true —— 会话轮换的原子性靠它。
//
// 注意「已存在」包含**已过期但尚未被清扫**的行：过期行的覆盖交给 Set；
// 抢占语义要求「过期即等价于不存在」，故这里先按过期删一次再插入（见下）。
func (s *Store) PutIfAbsent(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	// 先把同 key 的过期行清掉，否则 ON CONFLICT DO NOTHING 会把「过期残留」当成「已被占用」。
	if err := s.db.WithContext(ctx).
		Where("cache_key = ? AND expires_at <= ?", key, s.now()).
		Delete(&model.EphemeralKV{}).Error; err != nil {
		return false, fmt.Errorf("kv put-if-absent cleanup: %w", err)
	}
	row := model.EphemeralKV{CacheKey: key, CacheValue: value, ExpiresAt: s.now().Add(normalizeTTL(ttl))}
	res := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if res.Error != nil {
		return false, fmt.Errorf("kv put-if-absent: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}

// Sweep 删除已过期行，返回删除条数；limit<=0 表示不限量。
func (s *Store) Sweep(ctx context.Context, limit int) (int64, error) {
	now := s.now()
	base := func() *gorm.DB {
		return s.db.WithContext(ctx).Model(&model.EphemeralKV{}).Where("expires_at <= ?", now)
	}
	if limit <= 0 {
		res := base().Delete(&model.EphemeralKV{})
		return res.RowsAffected, res.Error
	}
	var keys []string
	if err := base().Order("expires_at").Limit(limit).Pluck("cache_key", &keys).Error; err != nil {
		return 0, fmt.Errorf("kv sweep select: %w", err)
	}
	if len(keys) == 0 {
		return 0, nil
	}
	res := s.db.WithContext(ctx).Where("cache_key IN ?", keys).Delete(&model.EphemeralKV{})
	return res.RowsAffected, res.Error
}

func normalizeTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return DefaultTTL
	}
	return ttl
}
