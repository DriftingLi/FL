// 字典仓储共享骨架：List/Get 的缓存 + 行扫描循环与写操作的
// ErrNoRows 检查收敛于此，各实体方法只保留 SQL 与扫描列。
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"forklift-training/internal/cache"
)

// listCached 缓存列表查询通用骨架：GetOrSetJSON + Query + 行扫描循环。
// 缓存 key 与 TTL 沿用全库统一的 dict:* 前缀与 TTLDictionary，线上 key 不变。
func listCached[T any](r *DictionaryRepository, ctx context.Context, cacheKey, what, query string,
	scan func(pgx.Rows) (T, error), args ...any) ([]T, error) {
	var result []T
	err := cache.GetOrSetJSON(ctx, cacheKey, cache.TTLDictionary, &result, func() (any, error) {
		rows, err := r.pool.Query(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("%s失败: %w", what, err)
		}
		defer rows.Close()
		out := make([]T, 0, 16)
		for rows.Next() {
			v, err := scan(rows)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, rows.Err()
	})
	return result, err
}

// getCached 缓存单行查询通用骨架；未命中返回 pgx.ErrNoRows（可被 errors.Is 识别）。
func getCached[T any](r *DictionaryRepository, ctx context.Context, cacheKey, what, query string,
	scan func(pgx.Row) (T, error), args ...any) (T, error) {
	var result T
	err := cache.GetOrSetJSON(ctx, cacheKey, cache.TTLDictionary, &result, func() (any, error) {
		row := r.pool.QueryRow(ctx, query, args...)
		v, err := scan(row)
		if err != nil {
			return nil, fmt.Errorf("%s失败: %w", what, err)
		}
		return v, nil
	})
	return result, err
}

// queryOne 无缓存单行查询（写操作 RETURNING 等）；未命中返回 pgx.ErrNoRows。
func queryOne[T any](r *DictionaryRepository, ctx context.Context, what, query string,
	scan func(pgx.Row) (T, error), args ...any) (T, error) {
	row := r.pool.QueryRow(ctx, query, args...)
	v, err := scan(row)
	if err != nil {
		return v, fmt.Errorf("%s失败: %w", what, err)
	}
	return v, nil
}

// insertReturningID 插入并返回自增 ID 的通用骨架。
func insertReturningID(r *DictionaryRepository, ctx context.Context, what, query string, args ...any) (int64, error) {
	var id int64
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&id); err != nil {
		return 0, fmt.Errorf("%s失败: %w", what, err)
	}
	return id, nil
}

// execWrite 更新/删除通用骨架：RowsAffected==0 时返回 pgx.ErrNoRows（实体不存在）。
func execWrite(r *DictionaryRepository, ctx context.Context, what, query string, args ...any) error {
	ct, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("%s失败: %w", what, err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
