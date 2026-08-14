// vehicle_types 车型 字典：只读方法（写面已迁至 dictcrud 描述符驱动，见 ADR-0008）。
package repository

import (
	"context"
)

// ListVehicleTypes 列出全部车型
func (r *DictionaryRepository) ListVehicleTypes(ctx context.Context) ([]VehicleType, error) {
	return readList[VehicleType](r, ctx, readSpecVtList, CacheKeyVtList, "查询车型")
}

// GetVehicleTypeByName 按名称查询车型（供 service 判断电动/内燃使用）
func (r *DictionaryRepository) GetVehicleTypeByName(ctx context.Context, name string) (VehicleType, error) {
	return readGet[VehicleType](r, ctx, readSpecVtGet, cacheKey(CachePrefixVtGet, name), "查询车型", name)
}
