// brands 品牌 字典 CRUD（骨架走 dict_helpers 共享实现）
package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"forklift-training/internal/cache"
)

// ListBrands 列出全部品牌（按 k_brand 倒序）
func (r *DictionaryRepository) ListBrands(ctx context.Context) ([]Brand, error) {
	return listCached(r, ctx, "dict:brands:list", "查询品牌",
		`SELECT id, name, k_brand, is_active FROM brands ORDER BY k_brand DESC, name ASC`,
		func(rows pgx.Rows) (Brand, error) {
			var b Brand
			err := rows.Scan(&b.ID, &b.Name, &b.KBrand, &b.IsActive)
			return b, err
		})
}

// CreateBrand 新增品牌
func (r *DictionaryRepository) CreateBrand(ctx context.Context, name string, kBrand float64, isActive bool) (Brand, error) {
	id, err := insertReturningID(r, ctx, "新增品牌",
		`INSERT INTO brands (name, k_brand, is_active)
		 VALUES ($1, $2, $3) ON CONFLICT (name) DO UPDATE
		 SET k_brand = EXCLUDED.k_brand, is_active = EXCLUDED.is_active
		 RETURNING id`,
		name, kBrand, isActive)
	if err != nil {
		return Brand{}, err
	}
	return Brand{ID: id, Name: name, KBrand: kBrand, IsActive: isActive}, nil
}

// UpdateBrand 更新品牌系数与启用状态
func (r *DictionaryRepository) UpdateBrand(ctx context.Context, id int64, kBrand float64, isActive bool) error {
	return execWrite(r, ctx, "更新品牌",
		`UPDATE brands SET k_brand = $2, is_active = $3 WHERE id = $1`,
		id, kBrand, isActive)
}

// DeleteBrand 删除品牌
func (r *DictionaryRepository) DeleteBrand(ctx context.Context, id int64) error {
	return execWrite(r, ctx, "删除品牌", `DELETE FROM brands WHERE id = $1`, id)
}

// GetBrandByName 按名称查询品牌（供 service 实时计算 Kb 使用）
func (r *DictionaryRepository) GetBrandByName(ctx context.Context, name string) (Brand, error) {
	return getCached(r, ctx, cache.SafeKey("dict", "brand", "get", name), "查询品牌",
		`SELECT id, name, k_brand, is_active FROM brands WHERE name = $1`,
		func(row pgx.Row) (Brand, error) {
			var b Brand
			err := row.Scan(&b.ID, &b.Name, &b.KBrand, &b.IsActive)
			return b, err
		}, name)
}
