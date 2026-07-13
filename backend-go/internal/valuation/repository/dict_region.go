// region_coefficients 区域系数 字典 CRUD（从 dictionaries.go 拆分）
// 手写 pgx 仓储，统一使用 *pgxpool.Pool 直接操作
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"forklift-training/internal/cache"
)

// ListRegionCoefficients 列出全部区域系数
func (r *DictionaryRepository) ListRegionCoefficients(ctx context.Context, province string) ([]RegionCoefficient, error) {
	cacheKey := cache.SafeKey("dict", "region", "list", province)
	var result []RegionCoefficient
	err := cache.GetOrSetJSON(ctx, cacheKey, cache.TTLDictionary, &result, func() (interface{}, error) {
		var rows pgx.Rows
		var err error
		if province != "" {
			rows, err = r.pool.Query(ctx,
				`SELECT id, province, city, coefficient FROM region_coefficients WHERE province = $1 ORDER BY id ASC`, province)
		} else {
			rows, err = r.pool.Query(ctx, `SELECT id, province, city, coefficient FROM region_coefficients ORDER BY id ASC`)
		}
		if err != nil {
			return nil, fmt.Errorf("查询区域系数失败: %w", err)
		}
		defer rows.Close()
		out := make([]RegionCoefficient, 0, 16)
		for rows.Next() {
			var rc RegionCoefficient
			if err := rows.Scan(&rc.ID, &rc.Province, &rc.City, &rc.Coefficient); err != nil {
				return nil, err
			}
			out = append(out, rc)
		}
		return out, rows.Err()
	})
	return result, err
}

// ListProvinces 列出全部省份（去重）
func (r *DictionaryRepository) ListProvinces(ctx context.Context) ([]string, error) {
	const cacheKey = "dict:region:provinces"
	var result []string
	err := cache.GetOrSetJSON(ctx, cacheKey, cache.TTLDictionary, &result, func() (interface{}, error) {
		rows, err := r.pool.Query(ctx, `SELECT DISTINCT province FROM region_coefficients ORDER BY province ASC`)
		if err != nil {
			return nil, fmt.Errorf("查询省份失败: %w", err)
		}
		defer rows.Close()
		out := make([]string, 0, 8)
		for rows.Next() {
			var p string
			if err := rows.Scan(&p); err != nil {
				return nil, err
			}
			out = append(out, p)
		}
		return out, rows.Err()
	})
	return result, err
}

// ListCities 按省份列出城市
func (r *DictionaryRepository) ListCities(ctx context.Context, province string) ([]string, error) {
	cacheKey := cache.SafeKey("dict", "region", "cities", province)
	var result []string
	err := cache.GetOrSetJSON(ctx, cacheKey, cache.TTLDictionary, &result, func() (interface{}, error) {
		rows, err := r.pool.Query(ctx,
			`SELECT city FROM region_coefficients WHERE province = $1 ORDER BY city ASC`, province)
		if err != nil {
			return nil, fmt.Errorf("查询城市失败: %w", err)
		}
		defer rows.Close()
		out := make([]string, 0, 8)
		for rows.Next() {
			var c string
			if err := rows.Scan(&c); err != nil {
				return nil, err
			}
			out = append(out, c)
		}
		return out, rows.Err()
	})
	return result, err
}

// CreateRegionCoefficient 新增区域系数
func (r *DictionaryRepository) CreateRegionCoefficient(ctx context.Context, province, city string, coefficient float64) (RegionCoefficient, error) {
	var id int
	err := r.pool.QueryRow(ctx,
		`INSERT INTO region_coefficients (province, city, coefficient) VALUES ($1, $2, $3)
		 ON CONFLICT (province, city) DO UPDATE SET coefficient = EXCLUDED.coefficient RETURNING id`,
		province, city, coefficient).Scan(&id)
	if err != nil {
		return RegionCoefficient{}, fmt.Errorf("新增区域系数失败: %w", err)
	}
	return RegionCoefficient{ID: id, Province: province, City: city, Coefficient: coefficient}, nil
}

// UpdateRegionCoefficient 更新区域系数
func (r *DictionaryRepository) UpdateRegionCoefficient(ctx context.Context, id int, coefficient float64) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE region_coefficients SET coefficient = $2 WHERE id = $1`, id, coefficient)
	if err != nil {
		return fmt.Errorf("更新区域系数失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// DeleteRegionCoefficient 删除区域系数
func (r *DictionaryRepository) DeleteRegionCoefficient(ctx context.Context, id int) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM region_coefficients WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除区域系数失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// GetRegionCoefficient 按 province + city 查询区域系数（供 service 计算 Km 使用）
// 未命中时返回 pgx.ErrNoRows，由调用方决定是否使用默认值 1.0
func (r *DictionaryRepository) GetRegionCoefficient(ctx context.Context, province, city string) (RegionCoefficient, error) {
	cacheKey := cache.SafeKey("dict", "region", "get", province, city)
	var result RegionCoefficient
	err := cache.GetOrSetJSON(ctx, cacheKey, cache.TTLDictionary, &result, func() (interface{}, error) {
		row := r.pool.QueryRow(ctx,
			`SELECT id, province, city, coefficient FROM region_coefficients WHERE province = $1 AND city = $2`, province, city)
		var rc RegionCoefficient
		if err := row.Scan(&rc.ID, &rc.Province, &rc.City, &rc.Coefficient); err != nil {
			return nil, err
		}
		return rc, nil
	})
	return result, err
}
