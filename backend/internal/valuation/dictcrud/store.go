// 描述符驱动通用写存储：Create/Update/Delete 由描述符生成 SQL 直接执行。
// 语义与迁移前 repository 骨架一致：insert RETURNING id；
// update/delete RowsAffected==0 → pgx.ErrNoRows（handler 映射 404）。
package dictcrud

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store 描述符驱动通用 CRUD 写存储（生产实现，handler 的 DictWriter seam 消费）。
type Store struct {
	pool *pgxpool.Pool
}

// NewStore 构造通用写存储。
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Create 执行描述符 INSERT 并返回新行 id。
func (s *Store) Create(ctx context.Context, d Descriptor, values map[string]any) (int64, error) {
	var id int64
	if err := s.pool.QueryRow(ctx, BuildInsertSQL(d), BuildInsertArgs(d, values)...).Scan(&id); err != nil {
		return 0, fmt.Errorf("新增%s失败: %w", d.EntityLabel, err)
	}
	return id, nil
}

// Update 执行描述符 UPDATE；未命中返回 pgx.ErrNoRows。
func (s *Store) Update(ctx context.Context, d Descriptor, id int64, values map[string]any) error {
	ct, err := s.pool.Exec(ctx, BuildUpdateSQL(d), BuildUpdateArgs(d, id, values)...)
	if err != nil {
		return fmt.Errorf("更新%s失败: %w", d.EntityLabel, err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// UpdateByKey 按唯一 key 列更新（coefficient_configs，PUT /:key）。
// ResponseReturning 时 RETURNING 整行并经 ResponseScan 扫描为响应；未命中返回 pgx.ErrNoRows。
func (s *Store) UpdateByKey(ctx context.Context, d Descriptor, key string, values map[string]any) (map[string]any, error) {
	if !d.ResponseReturning {
		ct, err := s.pool.Exec(ctx, BuildUpdateKeySQL(d), BuildUpdateKeyArgs(d, key, values)...)
		if err != nil {
			return nil, fmt.Errorf("更新%s失败: %w", d.EntityLabel, err)
		}
		if ct.RowsAffected() == 0 {
			return nil, pgx.ErrNoRows
		}
		return nil, nil
	}
	row := s.pool.QueryRow(ctx, BuildUpdateKeySQL(d), BuildUpdateKeyArgs(d, key, values)...)
	out, err := d.ResponseScan(row)
	if err != nil {
		return nil, fmt.Errorf("更新%s失败: %w", d.EntityLabel, err)
	}
	return out, nil
}

// Delete 执行描述符 DELETE；未命中返回 pgx.ErrNoRows。
func (s *Store) Delete(ctx context.Context, d Descriptor, id int64) error {
	ct, err := s.pool.Exec(ctx, BuildDeleteSQL(d), id)
	if err != nil {
		return fmt.Errorf("删除%s失败: %w", d.EntityLabel, err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
