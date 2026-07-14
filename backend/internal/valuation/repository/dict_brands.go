// brands 品牌 字典 CRUD（从 dictionaries.go 拆分）
// 手写 pgx 仓储，统一使用 *pgxpool.Pool 直接操作
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"forklift-training/internal/cache"
)

// ListBrands 列出全部品牌（按 k_brand 倒序）
func (r *DictionaryRepository) ListBrands(ctx context.Context) ([]Brand, error) {
	const cacheKey = "dict:brands:list"
	var result []Brand
	err := cache.GetOrSetJSON(ctx, cacheKey, cache.TTLDictionary, &result, func() (interface{}, error) {
		rows, err := r.pool.Query(ctx,
			`SELECT id, name, k_brand, is_active FROM brands ORDER BY k_brand DESC, name ASC`)
		if err != nil {
			return nil, fmt.Errorf("查询品牌失败: %w", err)
		}
		defer rows.Close()
		out := make([]Brand, 0, 16)
		for rows.Next() {
			var b Brand
			if err := rows.Scan(&b.ID, &b.Name, &b.KBrand, &b.IsActive); err != nil {
				return nil, err
			}
			out = append(out, b)
		}
		return out, rows.Err()
	})
	return result, err
}

// CreateBrand 新增品牌
func (r *DictionaryRepository) CreateBrand(ctx context.Context, name string, kBrand float64, isActive bool) (Brand, error) {
	var id int64
	err := r.pool.QueryRow(ctx,
		`INSERT INTO brands (name, k_brand, is_active)
		 VALUES ($1, $2, $3) ON CONFLICT (name) DO UPDATE
		 SET k_brand = EXCLUDED.k_brand, is_active = EXCLUDED.is_active
		 RETURNING id`,
		name, kBrand, isActive).Scan(&id)
	if err != nil {
		return Brand{}, fmt.Errorf("新增品牌失败: %w", err)
	}
	return Brand{ID: id, Name: name, KBrand: kBrand, IsActive: isActive}, nil
}

// UpdateBrand 更新品牌系数与启用状态
func (r *DictionaryRepository) UpdateBrand(ctx context.Context, id int64, kBrand float64, isActive bool) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE brands SET k_brand = $2, is_active = $3 WHERE id = $1`,
		id, kBrand, isActive)
	if err != nil {
		return fmt.Errorf("更新品牌失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// DeleteBrand 删除品牌
func (r *DictionaryRepository) DeleteBrand(ctx context.Context, id int64) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM brands WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除品牌失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// GetBrandByName 按名称查询品牌（供 service 实时计算 Kb 使用）
func (r *DictionaryRepository) GetBrandByName(ctx context.Context, name string) (Brand, error) {
	cacheKey := cache.SafeKey("dict", "brand", "get", name)
	var result Brand
	err := cache.GetOrSetJSON(ctx, cacheKey, cache.TTLDictionary, &result, func() (interface{}, error) {
		row := r.pool.QueryRow(ctx,
			`SELECT id, name, k_brand, is_active FROM brands WHERE name = $1`, name)
		var b Brand
		if err := row.Scan(&b.ID, &b.Name, &b.KBrand, &b.IsActive); err != nil {
			return nil, err
		}
		return b, nil
	})
	return result, err
}
