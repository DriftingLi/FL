// 字典仓储共享骨架：List/Get 的行扫描循环与写操作的 ErrNoRows 检查收敛于此，
// 各实体方法只保留 SQL 与扫描列。
//
// #2109 阶段 B：字典缓存随 Redis 一起退役 —— 访客量≈0 时缓存收益为零，而它带来的
// 失效契约（读 key 前缀 ⊆ 写路径失效 pattern）要一直养着。需要时再按同样形状加回。
package valuation

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// listRows 列表查询通用骨架：Query + 行扫描循环。
func listRows[T any](r *DictionaryRepository, ctx context.Context, what, query string,
	scan func(pgx.Rows) (T, error), args ...any) ([]T, error) {
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
}

// getRow 单行查询通用骨架；未命中返回 pgx.ErrNoRows（可被 errors.Is 识别）。
func getRow[T any](r *DictionaryRepository, ctx context.Context, what, query string,
	scan func(pgx.Row) (T, error), args ...any) (T, error) {
	var zero T
	v, err := scan(r.pool.QueryRow(ctx, query, args...))
	if err != nil {
		return zero, fmt.Errorf("%s失败: %w", what, err)
	}
	return v, nil
}

// queryOne 无缓存单行查询（读操作）；未命中返回 pgx.ErrNoRows。
func queryOne[T any](r *DictionaryRepository, ctx context.Context, what, query string,
	scan func(pgx.Row) (T, error), args ...any) (T, error) {
	row := r.pool.QueryRow(ctx, query, args...)
	v, err := scan(row)
	if err != nil {
		return v, fmt.Errorf("%s失败: %w", what, err)
	}
	return v, nil
}
