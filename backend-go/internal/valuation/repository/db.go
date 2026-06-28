// Package repository 提供 DBTX 接口与 Queries 容器。
// 历史上由 sqlc 生成，重构后保留作为通用查询容器，
// battery 仓储使用 *pgxpool.Pool 直接操作，本接口仅供兼容旧装配入口。
package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX 数据访问接口（兼容 pgxpool.Pool 与 pgx.Tx）
type DBTX interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

// New 构造 Queries 容器
func New(db DBTX) *Queries {
	return &Queries{db: db}
}

// Queries 通用查询容器（保留供历史调用方使用）
type Queries struct {
	db DBTX
}

// WithTx 在事务上下文中构造新的 Queries
func (q *Queries) WithTx(tx pgx.Tx) *Queries {
	return &Queries{
		db: tx,
	}
}
