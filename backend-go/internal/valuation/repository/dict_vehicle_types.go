// vehicle_types 车型 字典 CRUD（从 dictionaries.go 拆分）
// 手写 pgx 仓储，统一使用 *pgxpool.Pool 直接操作
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ListVehicleTypes 列出全部车型
func (r *DictionaryRepository) ListVehicleTypes(ctx context.Context) ([]VehicleType, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, power_type, earliest_factory_year FROM vehicle_types ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询车型失败: %w", err)
	}
	defer rows.Close()
	out := make([]VehicleType, 0, 8)
	for rows.Next() {
		var v VehicleType
		if err := rows.Scan(&v.ID, &v.Name, &v.PowerType, &v.EarliestFactoryYear); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// CreateVehicleType 新增车型
func (r *DictionaryRepository) CreateVehicleType(ctx context.Context, name, powerType string, earliestFactoryYear int) (VehicleType, error) {
	var id int
	err := r.pool.QueryRow(ctx,
		`INSERT INTO vehicle_types (name, power_type, earliest_factory_year) VALUES ($1, $2, $3) ON CONFLICT (name) DO UPDATE
		 SET power_type = EXCLUDED.power_type, earliest_factory_year = EXCLUDED.earliest_factory_year RETURNING id`,
		name, powerType, earliestFactoryYear).Scan(&id)
	if err != nil {
		return VehicleType{}, fmt.Errorf("新增车型失败: %w", err)
	}
	return VehicleType{ID: id, Name: name, PowerType: powerType, EarliestFactoryYear: earliestFactoryYear}, nil
}

// UpdateVehicleType 更新车型动力类型与最早出厂年份
func (r *DictionaryRepository) UpdateVehicleType(ctx context.Context, id int, name, powerType string, earliestFactoryYear int) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE vehicle_types SET name = $2, power_type = $3, earliest_factory_year = $4 WHERE id = $1`,
		id, name, powerType, earliestFactoryYear)
	if err != nil {
		return fmt.Errorf("更新车型失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// DeleteVehicleType 删除车型
func (r *DictionaryRepository) DeleteVehicleType(ctx context.Context, id int) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM vehicle_types WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除车型失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// GetVehicleTypeByName 按名称查询车型（供 service 判断电动/内燃使用）
func (r *DictionaryRepository) GetVehicleTypeByName(ctx context.Context, name string) (VehicleType, error) {
	row := r.pool.QueryRow(ctx, `SELECT id, name, power_type, earliest_factory_year FROM vehicle_types WHERE name = $1`, name)
	var v VehicleType
	if err := row.Scan(&v.ID, &v.Name, &v.PowerType, &v.EarliestFactoryYear); err != nil {
		return VehicleType{}, err
	}
	return v, nil
}
