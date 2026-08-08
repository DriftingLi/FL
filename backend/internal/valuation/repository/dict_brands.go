// brands 品牌 字典：只读方法（写面已迁至 dictcrud 描述符驱动，见 ADR-0008）。
package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// ListBrands 列出全部品牌（按 k_brand 倒序）
func (r *DictionaryRepository) ListBrands(ctx context.Context) ([]Brand, error) {
	return listCached(r, ctx, CacheKeyBrandsList, "查询品牌",
		`SELECT id, name, k_brand, is_active FROM brands ORDER BY k_brand DESC, name ASC`,
		func(rows pgx.Rows) (Brand, error) {
			var b Brand
			err := rows.Scan(&b.ID, &b.Name, &b.KBrand, &b.IsActive)
			return b, err
		})
}

// GetBrandByName 按名称查询品牌（供 service 实时计算 Kb 使用）
func (r *DictionaryRepository) GetBrandByName(ctx context.Context, name string) (Brand, error) {
	return getCached(r, ctx, cacheKey(CachePrefixBrandGet, name), "查询品牌",
		`SELECT id, name, k_brand, is_active FROM brands WHERE name = $1`,
		func(row pgx.Row) (Brand, error) {
			var b Brand
			err := row.Scan(&b.ID, &b.Name, &b.KBrand, &b.IsActive)
			return b, err
		}, name)
}
