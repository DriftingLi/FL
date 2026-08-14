// brands 品牌 字典：只读方法（写面已迁至 dictcrud 描述符驱动，见 ADR-0008）。
package repository

import (
	"context"
)

// ListBrands 列出全部品牌（按 k_brand 倒序）
func (r *DictionaryRepository) ListBrands(ctx context.Context) ([]Brand, error) {
	return readList[Brand](r, ctx, readSpecBrandsList, CacheKeyBrandsList, "查询品牌")
}

// GetBrandByName 按名称查询品牌（供 service 实时计算 Kb 使用）
func (r *DictionaryRepository) GetBrandByName(ctx context.Context, name string) (Brand, error) {
	return readGet[Brand](r, ctx, readSpecBrandsGet, cacheKey(CachePrefixBrandGet, name), "查询品牌", name)
}
