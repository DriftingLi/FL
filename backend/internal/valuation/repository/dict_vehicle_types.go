// vehicle_types 车型 字典 CRUD（骨架走 dict_helpers 共享实现）
package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"forklift-training/internal/cache"
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

// CreateVehicleType 新增车型
func (r *DictionaryRepository) CreateVehicleType(ctx context.Context, name, powerType string, earliestFactoryYear int) (VehicleType, error) {
	id, err := insertReturningID(r, ctx, "新增车型",
		`INSERT INTO vehicle_types (name, power_type, earliest_factory_year) VALUES ($1, $2, $3) ON CONFLICT (name) DO UPDATE
		 SET power_type = EXCLUDED.power_type, earliest_factory_year = EXCLUDED.earliest_factory_year RETURNING id`,
		name, powerType, earliestFactoryYear)
	if err != nil {
		return VehicleType{}, err
	}
	return VehicleType{ID: int(id), Name: name, PowerType: powerType, EarliestFactoryYear: earliestFactoryYear}, nil
}

// UpdateVehicleType 更新车型动力类型与最早出厂年份
func (r *DictionaryRepository) UpdateVehicleType(ctx context.Context, id int, name, powerType string, earliestFactoryYear int) error {
	return execWrite(r, ctx, "更新车型",
		`UPDATE vehicle_types SET name = $2, power_type = $3, earliest_factory_year = $4 WHERE id = $1`,
		id, name, powerType, earliestFactoryYear)
}

// DeleteVehicleType 删除车型
func (r *DictionaryRepository) DeleteVehicleType(ctx context.Context, id int) error {
	return execWrite(r, ctx, "删除车型", `DELETE FROM vehicle_types WHERE id = $1`, id)
}

// GetVehicleTypeByName 按名称查询车型（供 service 判断电动/内燃使用）
func (r *DictionaryRepository) GetVehicleTypeByName(ctx context.Context, name string) (VehicleType, error) {
	return getCached(r, ctx, cache.SafeKey("dict", "vt", "get", name), "查询车型",
		`SELECT id, name, power_type, earliest_factory_year FROM vehicle_types WHERE name = $1`,
		func(row pgx.Row) (VehicleType, error) {
			var v VehicleType
			err := row.Scan(&v.ID, &v.Name, &v.PowerType, &v.EarliestFactoryYear)
			return v, err
		}, name)
}
