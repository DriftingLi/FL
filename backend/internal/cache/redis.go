package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"forklift-training/internal/config"
)

// pkgLogger 包级日志器，InitRedis 时注入（缓存模块为包级函数形态）。
// 未初始化时回退 Nop，保证包级函数在测试等场景下可安全调用。
var pkgLogger = zap.NewNop()

// client 全局 Redis 客户端，InitRedis 后赋值。
var client *redis.Client

// prefix 全局 key 前缀，InitRedis 后由配置注入。
var prefix string

// InitRedis 连接 Redis 服务器，Ping 验证连通性，并初始化全局 client 与 key 前缀。
// 失败返回 error，调用方应在启动流程中以 os.Exit(1) 处理。
func InitRedis(cfg config.RedisConfig, logger *zap.Logger) (*redis.Client, error) {
	c := redis.NewClient(&redis.Options{
		Addr:            cfg.Addr,
		Password:        cfg.Password,
		DB:              cfg.DB,
		PoolSize:        cfg.PoolSize,
		MinIdleConns:    cfg.MinIdleConns,
		MaxRetries:      cfg.MaxRetries,
		DialTimeout:     cfg.DialTimeout,
		ReadTimeout:     cfg.ReadTimeout,
		WriteTimeout:    cfg.WriteTimeout,
		PoolTimeout:     cfg.PoolTimeout,
		ConnMaxIdleTime: cfg.IdleTimeout,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping 失败: %w", err)
	}

	client = c
	pkgLogger = logger

	// 前缀优先用配置值，回退默认
	if cfg.Prefix != "" {
		prefix = cfg.Prefix
	} else {
		prefix = DefaultKeyPrefix
	}

	logger.Info("Redis 连接成功",
		zap.String("addr", cfg.Addr),
		zap.Int("db", cfg.DB),
		zap.String("prefix", prefix),
		zap.Int("poolSize", cfg.PoolSize),
		zap.Int("minIdle", cfg.MinIdleConns),
	)
	return c, nil
}

// GetClient 返回全局 Redis 客户端，未初始化时返回 nil。
// 提供给需要直接使用 go-redis 高级特性（Pipeline、Pub/Sub 等）的调用方。
// 注意：使用此函数时需自行拼接 fullKey 前缀。
func GetClient() *redis.Client { return client }

// Ping 探测 Redis 连通性，用于健康检查。
func Ping(ctx context.Context) error {
	if client == nil {
		return errors.New("redis client 未初始化")
	}
	return client.Ping(ctx).Err()
}

// CloseRedis 优雅关闭 Redis 连接池。
// 传入 nil 时空操作；关闭失败仅记录日志，不阻断退出流程。
func CloseRedis(c *redis.Client, logger *zap.Logger) {
	if c == nil {
		return
	}
	if err := c.Close(); err != nil {
		logger.Warn("Redis 关闭异常", zap.Error(err))
		return
	}
	logger.Info("Redis 连接池已关闭")
}
