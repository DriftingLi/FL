// 级联过滤方法：基于 original_prices 表查询有效组合（从 dictionaries.go 拆分）
// 手写 pgx 仓储，统一使用 *pgxpool.Pool 直接操作
package repository

import (
	"context"
	"fmt"
)

// ListSeriesByBrand 按品牌列出系列
func (r *DictionaryRepository) ListSeriesByBrand(ctx context.Context, brand string) ([]Series, error) {
	return r.ListSeries(ctx, brand)
}

// ListBatteryTypesByCascade 级联查询电池类型：基于品牌+系列过滤
// 通过 series_config_options 表查询该 series 支持的 battery 维度选项（去重）
// 注：vehicleType 与 tonnage 参数保留以维持接口兼容，但不再用于过滤
// （series_config_options 按 brand+series 索引，battery 维度与 vehicle_type/tonnage 无关）
func (r *DictionaryRepository) ListBatteryTypesByCascade(ctx context.Context, brand, vehicleType, series, tonnage string) ([]BatteryTypeDict, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT bt.id, bt.name
		FROM series_config_options sco
		JOIN battery_types bt ON bt.name = sco.option_name
		WHERE sco.brand = $1 AND sco.series = $2 AND sco.dimension = 'battery'
		ORDER BY bt.id ASC`, brand, series)
	if err != nil {
		return nil, fmt.Errorf("级联查询电池类型失败: %w", err)
	}
	defer rows.Close()
	out := make([]BatteryTypeDict, 0, 4)
	for rows.Next() {
		var b BatteryTypeDict
		if err := rows.Scan(&b.ID, &b.Name); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// GetEarliestFactoryYearByCascade 按品牌+车型+系列+吨位级联查询最早出厂年份
// 取该组合下所有原价记录 earliest_factory_year 的最小值，作为学生端出厂年份输入下限
// series 为空字符串时忽略 series 条件（用于 series="其它" 的降级场景）
// 无匹配记录时返回 1980（与学生端默认下限一致）
func (r *DictionaryRepository) GetEarliestFactoryYearByCascade(
	ctx context.Context, brand, vehicleType, series string, tonnage float64,
) (int, error) {
	var year int
	if series == "" {
		err := r.pool.QueryRow(ctx, `
			SELECT COALESCE(MIN(earliest_factory_year), 1980)
			FROM original_prices
			WHERE brand = $1 AND vehicle_type = $2 AND tonnage = $3`,
			brand, vehicleType, tonnage).Scan(&year)
		if err != nil {
			return 1980, fmt.Errorf("查询最早出厂年份失败: %w", err)
		}
		return year, nil
	}
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(MIN(earliest_factory_year), 1980)
		FROM original_prices
		WHERE brand = $1 AND vehicle_type = $2 AND series = $3 AND tonnage = $4`,
		brand, vehicleType, series, tonnage).Scan(&year)
	if err != nil {
		return 1980, fmt.Errorf("查询最早出厂年份失败: %w", err)
	}
	return year, nil
}

// CascadeFilter 级联过滤参数
// 每个字段对应前序已选层级；为空字符串表示该层级未选，不过滤
type CascadeFilter struct {
	Brand       string // 品牌
	VehicleType string // 车辆类型
	Series      string // 系列
	Tonnage     string // 吨位（字符串便于 SQL 拼接，调用方自行格式化）
	ConfigType  string // 配置类型
	MastType    string // 门架类型
}

// ListVehicleTypesByBrand 按品牌列出可选车辆类型（数据源为 original_prices 表）
// LEFT JOIN vehicle_types 字典表仅用于补充 power_type / earliest_factory_year
// 管理员在原价表里填新 vehicle_type 后学生端立即可见，无需先在 vehicle_types 字典表建记录
func (r *DictionaryRepository) ListVehicleTypesByBrand(ctx context.Context, brand string) ([]VehicleType, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT MIN(op.id), op.vehicle_type,
		       COALESCE(MIN(vt.power_type), ''),
		       COALESCE(MIN(vt.earliest_factory_year), 1980)
		FROM original_prices op
		LEFT JOIN vehicle_types vt ON vt.name = op.vehicle_type
		WHERE op.brand = $1
		GROUP BY op.vehicle_type
		ORDER BY op.vehicle_type`, brand)
	if err != nil {
		return nil, fmt.Errorf("级联查询车型失败: %w", err)
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

// ListSeriesByCascade 按品牌+车辆类型级联查询系列
// 数据源为 original_prices 表，LEFT JOIN series 字典表仅用于补充 earliest_factory_year
// 这样管理员在原价表里新增 series 名字后，学生端立即可见，无需先在 series 字典表建记录
func (r *DictionaryRepository) ListSeriesByCascade(ctx context.Context, brand, vehicleType string) ([]Series, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT MIN(op.id), op.brand, op.series, COALESCE(MIN(s.earliest_factory_year), 1980)
		FROM original_prices op
		LEFT JOIN series s ON s.brand = op.brand AND s.name = op.series
		WHERE op.brand = $1 AND op.vehicle_type = $2
		GROUP BY op.brand, op.series
		ORDER BY op.series`, brand, vehicleType)
	if err != nil {
		return nil, fmt.Errorf("级联查询系列失败: %w", err)
	}
	defer rows.Close()
	out := make([]Series, 0, 16)
	for rows.Next() {
		var s Series
		if err := rows.Scan(&s.ID, &s.Brand, &s.Name, &s.EarliestFactoryYear); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListTonnagesByCascade 按品牌+车辆类型+系列级联查询吨位（数据源为 original_prices 表）
// 管理员在原价表里填新 tonnage 后学生端立即可见，无需先在 tonnages 字典表建记录
func (r *DictionaryRepository) ListTonnagesByCascade(ctx context.Context, brand, vehicleType, series string) ([]Tonnage, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT MIN(op.id), op.tonnage
		FROM original_prices op
		WHERE op.brand = $1 AND op.vehicle_type = $2 AND op.series = $3
		GROUP BY op.tonnage
		ORDER BY op.tonnage`, brand, vehicleType, series)
	if err != nil {
		return nil, fmt.Errorf("级联查询吨位失败: %w", err)
	}
	defer rows.Close()
	out := make([]Tonnage, 0, 16)
	for rows.Next() {
		var t Tonnage
		if err := rows.Scan(&t.ID, &t.Value); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListConfigOptionsByCascade 按品牌+车辆类型+系列+吨位级联查询配置类型选项
// config_type 为复合字符串（如"手波/国产发动机"、"磷酸铁锂(LFP)"），直接从 original_prices 取 DISTINCT
func (r *DictionaryRepository) ListConfigOptionsByCascade(ctx context.Context, brand, vehicleType, series, tonnage string) ([]ConfigOption, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT MIN(op.id), op.config_type
		FROM original_prices op
		WHERE op.brand = $1 AND op.vehicle_type = $2 AND op.series = $3 AND op.tonnage = $4::numeric
		GROUP BY op.config_type
		ORDER BY op.config_type`, brand, vehicleType, series, tonnage)
	if err != nil {
		return nil, fmt.Errorf("级联查询配置类型失败: %w", err)
	}
	defer rows.Close()
	out := make([]ConfigOption, 0, 8)
	for rows.Next() {
		var c ConfigOption
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListMastTypesByCascade 按前序层级+配置类型级联查询门架类型（数据源为 original_prices 表）
// 管理员在原价表里填新 mast_type 后学生端立即可见，无需先在 mast_types 字典表建记录
func (r *DictionaryRepository) ListMastTypesByCascade(ctx context.Context, brand, vehicleType, series, tonnage, configType string) ([]MastType, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT MIN(op.id), op.mast_type
		FROM original_prices op
		WHERE op.brand = $1 AND op.vehicle_type = $2 AND op.series = $3 AND op.tonnage = $4::numeric AND op.config_type = $5
		GROUP BY op.mast_type
		ORDER BY op.mast_type`, brand, vehicleType, series, tonnage, configType)
	if err != nil {
		return nil, fmt.Errorf("级联查询门架类型失败: %w", err)
	}
	defer rows.Close()
	out := make([]MastType, 0, 8)
	for rows.Next() {
		var m MastType
		if err := rows.Scan(&m.ID, &m.Name); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListMastHeightsByCascade 按前序层级+门架类型级联查询门架高度（数据源为 original_prices 表）
// 管理员在原价表里填新 mast_height_mm 后学生端立即可见，无需先在 mast_heights 字典表建记录
func (r *DictionaryRepository) ListMastHeightsByCascade(ctx context.Context, brand, vehicleType, series, tonnage, configType, mastType string) ([]MastHeight, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT MIN(op.id), op.mast_height_mm
		FROM original_prices op
		WHERE op.brand = $1 AND op.vehicle_type = $2 AND op.series = $3 AND op.tonnage = $4::numeric AND op.config_type = $5 AND op.mast_type = $6
		GROUP BY op.mast_height_mm
		ORDER BY op.mast_height_mm`, brand, vehicleType, series, tonnage, configType, mastType)
	if err != nil {
		return nil, fmt.Errorf("级联查询门架高度失败: %w", err)
	}
	defer rows.Close()
	out := make([]MastHeight, 0, 8)
	for rows.Next() {
		var m MastHeight
		if err := rows.Scan(&m.ID, &m.ValueMM); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
