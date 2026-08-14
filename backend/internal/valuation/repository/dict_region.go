// region_coefficients 区域系数 字典 CRUD（骨架走 dict_helpers 共享实现）
package repository

import (
	"context"
)

// ListRegionCoefficients 列出全部区域系数（可按 province 筛选）
func (r *DictionaryRepository) ListRegionCoefficients(ctx context.Context, province string) ([]RegionCoefficient, error) {
	args := []any{}
	spec := readSpecRegionList
	if province != "" {
		spec.Where = "province = $1"
		args = append(args, province)
	}
	return readList[RegionCoefficient](r, ctx, spec, cacheKey(CachePrefixRegionList, province), "查询区域系数", args...)
}

// ListProvinces 列出全部省份（去重）
func (r *DictionaryRepository) ListProvinces(ctx context.Context) ([]string, error) {
	return readScalarList(r, ctx, readSpecRegionProvinces, CacheKeyRegionProvinces, "查询省份")
}

// ListCities 按省份列出城市
func (r *DictionaryRepository) ListCities(ctx context.Context, province string) ([]string, error) {
	return readScalarList(r, ctx, readSpecRegionCities, cacheKey(CachePrefixRegionCities, province), "查询城市", province)
}

// 注：region_coefficients 的写操作（Create/Update/Delete）已迁至描述符驱动核心
// （dictcrud.RegionCoefficientDescriptor + dictcrud.Store，见 ADR-0008）。

// GetRegionCoefficient 按 province + city 查询区域系数（供 service 计算 Km 使用）
// 未命中时返回 pgx.ErrNoRows，由调用方决定是否使用默认值 1.0
func (r *DictionaryRepository) GetRegionCoefficient(ctx context.Context, province, city string) (RegionCoefficient, error) {
	return readGet[RegionCoefficient](r, ctx, readSpecRegionGet, cacheKey(CachePrefixRegionGet, province, city), "查询区域系数", province, city)
}
