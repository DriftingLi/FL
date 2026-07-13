// Package cache 提供 Redis 缓存层：连接管理、通用读写封装与 Cache-Aside 辅助方法。
// 设计原则：
//   - Redis 不可用时降级为 nil/miss，不回传 error，让调用方自动回退到 DB 直查
//   - 异常仅通过 slog 记录，不中断业务流程
//   - Key 统一使用可配置前缀（REDIS_KEY_PREFIX），避免多项目共享同一 Redis 实例时的冲突
package cache

import "time"

// DefaultKeyPrefix 默认 key 前缀，可通过配置覆盖。
const DefaultKeyPrefix = "fl:"

// 默认过期时间
const (
	DefaultTTL = 10 * time.Minute // 写缓存默认过期时间

	TTLDictionary   = 60 * time.Minute // 字典/配置数据
	TTLStats        = 5 * time.Minute  // 聚合统计
	TTLUserProfile  = 5 * time.Minute  // 个人档案
	TTLValuation    = 3 * time.Minute  // 估价结果
	TTLJWTBlacklist = 24 * time.Hour   // JWT 黑名单（覆盖 token 最大有效期）
)
