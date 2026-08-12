// vehicle_types 车型 字典：只读方法（写面已迁至 dictcrud 描述符驱动，见 ADR-0008）。
package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// ListVehicleTypes 列出全部车型
func (r *DictionaryRepository) ListVehicleTypes(ctx context.Context) ([]VehicleType, error) {
	return listCached(r, ctx, "dict:vt:list", "查询车型",
		`SELECT id, name, power_type, earliest_factory_year FROM vehicle_types ORDER BY id ASC`,
		func(rows pgx.Rows) (VehicleType, error) {
			var v VehicleType
			err := rows.Scan(&v.ID, &v.Name, &v.PowerType, &v.EarliestFactoryYear)
			return v, err
		})
}

// GetVehicleTypeByName 按名称查询车型（供 service 判断电动/内燃使用）
func (r *DictionaryRepository) GetVehicleTypeByName(ctx context.Context, name string) (VehicleType, error) {
	return getCached(r, ctx, cacheKey(CachePrefixVtGet, name), "查询车型",
		`SELECT id, name, power_type, earliest_factory_year FROM vehicle_types WHERE name = $1`,
		func(row pgx.Row) (VehicleType, error) {
			var v VehicleType
			err := row.Scan(&v.ID, &v.Name, &v.PowerType, &v.EarliestFactoryYear)
			return v, err
		}, name)
}
