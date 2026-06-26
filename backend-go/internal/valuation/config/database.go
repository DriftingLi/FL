// Package config 提供 PostgreSQL 连接池的初始化能力
package config

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPostgresPool 创建并验证 PostgreSQL 连接池
// 必须在启动服务前调用，连接失败直接退出
func NewPostgresPool(ctx context.Context, dsn string, maxOpen, maxIdle int, lifetimeSec int) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("解析 DSN 失败: %w", err)
	}
	if maxOpen > 0 {
		cfg.MaxConns = int32(maxOpen)
	}
	if maxIdle > 0 {
		cfg.MinConns = int32(maxIdle)
	}
	if lifetimeSec > 0 {
		cfg.MaxConnLifetime = time.Duration(lifetimeSec) * time.Second
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("创建连接池失败: %w", err)
	}

	// 启动时主动 Ping 一次，提前暴露连接问题
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}
	return pool, nil
}
