package model

import "time"

// EphemeralKV 带 TTL 的临时键值（#2109：Redis 退场后验证码 / 图形码 / JWT 黑名单的落点）。
//
// 为什么要这张表：这三类状态在 Redis 时代共用同一套「带过期的 get / set / setnx」语义，
// 且都不属于业务事实——掉一条只意味着「重发一次验证码」或「重新登录一次」。
//
// 过期判定一律由**应用时钟**给出（kv.Store 传参），不依赖库内 now()：
// 读与写两侧同一时钟源，测试里注入时钟即可复现过期。
type EphemeralKV struct {
	CacheKey   string    `gorm:"column:cache_key;primaryKey;size:255"`
	CacheValue string    `gorm:"column:cache_value;type:text;not null"`
	ExpiresAt  time.Time `gorm:"column:expires_at;not null;index:idx_ephemeral_kv_expires_at"`
}

// TableName 固定表名（与迁移 000048 逐字一致）。
func (EphemeralKV) TableName() string { return "ephemeral_kv" }
