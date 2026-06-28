// Package repository - 字典表与 original_prices 数据访问
// 手写 pgx 仓储，覆盖学生端只读与管理员 CRUD 接口
// 设计参考 battery.go，统一使用 *pgxpool.Pool 直接操作
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// =====================================================
// 字典 DTO 定义
// =====================================================

// BrandType 品牌类型（进口一线 / 国产一线 等）
type BrandType struct {
	Name  string  `json:"name"`
	KType float64 `json:"k_type"`
}

// Brand 品牌
type Brand struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	BrandType string  `json:"brand_type"`
	KBrand   float64 `json:"k_brand"`
	IsActive bool    `json:"is_active"`
}

// VehicleType 车型（含动力类型 electric / combustion）
type VehicleType struct {
	ID                  int    `json:"id"`
	Name                string `json:"name"`
	PowerType           string `json:"power_type"`
	EarliestFactoryYear int    `json:"earliest_factory_year"`
}

// Series 系列
type Series struct {
	ID                  int    `json:"id"`
	Brand               string `json:"brand"`
	Name                string `json:"name"`
	EarliestFactoryYear int    `json:"earliest_factory_year"`
}

// Tonnage 吨位
type Tonnage struct {
	ID    int     `json:"id"`
	Value float64 `json:"value"`
}

// ConfigType 配置类型（标准配置 / 高配置）
type ConfigType struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// MastType 门架类型
type MastType struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// MastHeight 门架高度
type MastHeight struct {
	ID      int `json:"id"`
	ValueMM int `json:"value_mm"`
}

// BatteryTypeDict 电池类型字典
type BatteryTypeDict struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ConditionRating 车况评级
type ConditionRating struct {
	ID             int     `json:"id"`
	Rating         string  `json:"rating"`
	Label          string  `json:"label"`
	BaseCoefficient float64 `json:"base_coefficient"`
}

// RegionCoefficient 区域系数
type RegionCoefficient struct {
	ID          int     `json:"id"`
	Province    string  `json:"province"`
	City        string  `json:"city"`
	Coefficient float64 `json:"coefficient"`
}

// OriginalPrice 车辆原价
type OriginalPrice struct {
	ID           int64    `json:"id"`
	BrandType    string   `json:"brand_type"`
	Brand        string   `json:"brand"`
	VehicleType  string   `json:"vehicle_type"`
	Series       string   `json:"series"`
	Tonnage      float64  `json:"tonnage"`
	ConfigType   string   `json:"config_type"`
	MastType     string   `json:"mast_type"`
	MastHeightMM int      `json:"mast_height_mm"`
	BatteryType  *string  `json:"battery_type,omitempty"`
	OriginalPrice float64 `json:"original_price"`
	IsActive     bool     `json:"is_active"`
	UpdatedAt    string   `json:"updated_at"`
}

// CoefficientConfig 系数配置（全局可调参数）
type CoefficientConfig struct {
	ID          int32   `json:"id"`
	Key         string  `json:"key"`
	Value       float64 `json:"value"`
	Description string  `json:"description"`
	UpdatedAt   string  `json:"updated_at"`
}

// =====================================================
// 仓储入口
// =====================================================

// DictionaryRepository 字典与原价仓储
// 持有 *pgxpool.Pool，所有方法均为线程安全（pgx 连接池内置并发控制）
type DictionaryRepository struct {
	pool *pgxpool.Pool
}

// NewDictionaryRepository 构造字典仓储
func NewDictionaryRepository(pool *pgxpool.Pool) *DictionaryRepository {
	return &DictionaryRepository{pool: pool}
}

// =====================================================
// brand_types
// =====================================================

// ListBrandTypes 列出全部品牌类型
func (r *DictionaryRepository) ListBrandTypes(ctx context.Context) ([]BrandType, error) {
	rows, err := r.pool.Query(ctx, `SELECT name, k_type FROM brand_types ORDER BY k_type DESC`)
	if err != nil {
		return nil, fmt.Errorf("查询品牌类型失败: %w", err)
	}
	defer rows.Close()
	out := make([]BrandType, 0, 8)
	for rows.Next() {
		var bt BrandType
		if err := rows.Scan(&bt.Name, &bt.KType); err != nil {
			return nil, err
		}
		out = append(out, bt)
	}
	return out, rows.Err()
}

// CreateBrandType 新增品牌类型
func (r *DictionaryRepository) CreateBrandType(ctx context.Context, name string, kType float64) (BrandType, error) {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO brand_types (name, k_type) VALUES ($1, $2) ON CONFLICT (name) DO NOTHING`, name, kType)
	if err != nil {
		return BrandType{}, fmt.Errorf("新增品牌类型失败: %w", err)
	}
	return BrandType{Name: name, KType: kType}, nil
}

// UpdateBrandType 更新品牌类型系数
func (r *DictionaryRepository) UpdateBrandType(ctx context.Context, name string, kType float64) error {
	ct, err := r.pool.Exec(ctx, `UPDATE brand_types SET k_type = $2 WHERE name = $1`, name, kType)
	if err != nil {
		return fmt.Errorf("更新品牌类型失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// DeleteBrandType 删除品牌类型
func (r *DictionaryRepository) DeleteBrandType(ctx context.Context, name string) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM brand_types WHERE name = $1`, name)
	if err != nil {
		return fmt.Errorf("删除品牌类型失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// =====================================================
// brands
// =====================================================

// ListBrands 列出全部品牌（可按 brand_type 筛选）
func (r *DictionaryRepository) ListBrands(ctx context.Context, brandType string) ([]Brand, error) {
	var rows pgx.Rows
	var err error
	if brandType != "" {
		rows, err = r.pool.Query(ctx,
			`SELECT id, name, brand_type, k_brand, is_active FROM brands WHERE brand_type = $1 ORDER BY k_brand DESC, name ASC`, brandType)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT id, name, brand_type, k_brand, is_active FROM brands ORDER BY k_brand DESC, name ASC`)
	}
	if err != nil {
		return nil, fmt.Errorf("查询品牌失败: %w", err)
	}
	defer rows.Close()
	out := make([]Brand, 0, 16)
	for rows.Next() {
		var b Brand
		if err := rows.Scan(&b.ID, &b.Name, &b.BrandType, &b.KBrand, &b.IsActive); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ListBrandsByType 按品牌类型列出品牌
func (r *DictionaryRepository) ListBrandsByType(ctx context.Context, brandType string) ([]Brand, error) {
	return r.ListBrands(ctx, brandType)
}

// CreateBrand 新增品牌
func (r *DictionaryRepository) CreateBrand(ctx context.Context, name, brandType string, kBrand float64, isActive bool) (Brand, error) {
	var id int64
	err := r.pool.QueryRow(ctx,
		`INSERT INTO brands (name, brand_type, k_brand, is_active)
		 VALUES ($1, $2, $3, $4) ON CONFLICT (name) DO UPDATE
		 SET brand_type = EXCLUDED.brand_type, k_brand = EXCLUDED.k_brand, is_active = EXCLUDED.is_active
		 RETURNING id`,
		name, brandType, kBrand, isActive).Scan(&id)
	if err != nil {
		return Brand{}, fmt.Errorf("新增品牌失败: %w", err)
	}
	return Brand{ID: id, Name: name, BrandType: brandType, KBrand: kBrand, IsActive: isActive}, nil
}

// UpdateBrand 更新品牌
func (r *DictionaryRepository) UpdateBrand(ctx context.Context, id int64, brandType string, kBrand float64, isActive bool) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE brands SET brand_type = $2, k_brand = $3, is_active = $4 WHERE id = $1`,
		id, brandType, kBrand, isActive)
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
	row := r.pool.QueryRow(ctx,
		`SELECT id, name, brand_type, k_brand, is_active FROM brands WHERE name = $1`, name)
	var b Brand
	if err := row.Scan(&b.ID, &b.Name, &b.BrandType, &b.KBrand, &b.IsActive); err != nil {
		return Brand{}, err
	}
	return b, nil
}

// =====================================================
// vehicle_types
// =====================================================

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

// =====================================================
// series
// =====================================================

// ListSeries 列出全部系列（可按 brand 筛选）
func (r *DictionaryRepository) ListSeries(ctx context.Context, brand string) ([]Series, error) {
	var rows pgx.Rows
	var err error
	if brand != "" {
		rows, err = r.pool.Query(ctx, `SELECT id, brand, name, earliest_factory_year FROM series WHERE brand = $1 ORDER BY id ASC`, brand)
	} else {
		rows, err = r.pool.Query(ctx, `SELECT id, brand, name, earliest_factory_year FROM series ORDER BY id ASC`)
	}
	if err != nil {
		return nil, fmt.Errorf("查询系列失败: %w", err)
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

// ListSeriesByBrand 按品牌列出系列
func (r *DictionaryRepository) ListSeriesByBrand(ctx context.Context, brand string) ([]Series, error) {
	return r.ListSeries(ctx, brand)
}

// CreateSeries 新增系列
func (r *DictionaryRepository) CreateSeries(ctx context.Context, brand, name string, earliestFactoryYear int) (Series, error) {
	var id int
	err := r.pool.QueryRow(ctx,
		`INSERT INTO series (brand, name, earliest_factory_year) VALUES ($1, $2, $3) ON CONFLICT (brand, name) DO NOTHING RETURNING id`,
		brand, name, earliestFactoryYear).Scan(&id)
	if err != nil {
		return Series{}, fmt.Errorf("新增系列失败: %w", err)
	}
	return Series{ID: id, Brand: brand, Name: name, EarliestFactoryYear: earliestFactoryYear}, nil
}

// UpdateSeries 更新系列
func (r *DictionaryRepository) UpdateSeries(ctx context.Context, id int, brand, name string, earliestFactoryYear int) error {
	ct, err := r.pool.Exec(ctx, `UPDATE series SET brand = $2, name = $3, earliest_factory_year = $4 WHERE id = $1`, id, brand, name, earliestFactoryYear)
	if err != nil {
		return fmt.Errorf("更新系列失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// DeleteSeries 删除系列
func (r *DictionaryRepository) DeleteSeries(ctx context.Context, id int) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM series WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除系列失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// =====================================================
// tonnages
// =====================================================

// ListTonnages 列出全部吨位
func (r *DictionaryRepository) ListTonnages(ctx context.Context) ([]Tonnage, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, value FROM tonnages ORDER BY value ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询吨位失败: %w", err)
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

// CreateTonnage 新增吨位
func (r *DictionaryRepository) CreateTonnage(ctx context.Context, value float64) (Tonnage, error) {
	var id int
	err := r.pool.QueryRow(ctx,
		`INSERT INTO tonnages (value) VALUES ($1) ON CONFLICT (value) DO NOTHING RETURNING id`, value).Scan(&id)
	if err != nil {
		return Tonnage{}, fmt.Errorf("新增吨位失败: %w", err)
	}
	return Tonnage{ID: id, Value: value}, nil
}

// DeleteTonnage 删除吨位
func (r *DictionaryRepository) DeleteTonnage(ctx context.Context, id int) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM tonnages WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除吨位失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// =====================================================
// config_types
// =====================================================

// ListConfigTypes 列出全部配置类型
func (r *DictionaryRepository) ListConfigTypes(ctx context.Context) ([]ConfigType, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name FROM config_types ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询配置类型失败: %w", err)
	}
	defer rows.Close()
	out := make([]ConfigType, 0, 8)
	for rows.Next() {
		var c ConfigType
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateConfigType 新增配置类型
func (r *DictionaryRepository) CreateConfigType(ctx context.Context, name string) (ConfigType, error) {
	var id int
	err := r.pool.QueryRow(ctx,
		`INSERT INTO config_types (name) VALUES ($1) ON CONFLICT (name) DO NOTHING RETURNING id`, name).Scan(&id)
	if err != nil {
		return ConfigType{}, fmt.Errorf("新增配置类型失败: %w", err)
	}
	return ConfigType{ID: id, Name: name}, nil
}

// DeleteConfigType 删除配置类型
func (r *DictionaryRepository) DeleteConfigType(ctx context.Context, id int) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM config_types WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除配置类型失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// =====================================================
// mast_types
// =====================================================

// ListMastTypes 列出全部门架类型
func (r *DictionaryRepository) ListMastTypes(ctx context.Context) ([]MastType, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name FROM mast_types ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询门架类型失败: %w", err)
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

// CreateMastType 新增门架类型
func (r *DictionaryRepository) CreateMastType(ctx context.Context, name string) (MastType, error) {
	var id int
	err := r.pool.QueryRow(ctx,
		`INSERT INTO mast_types (name) VALUES ($1) ON CONFLICT (name) DO NOTHING RETURNING id`, name).Scan(&id)
	if err != nil {
		return MastType{}, fmt.Errorf("新增门架类型失败: %w", err)
	}
	return MastType{ID: id, Name: name}, nil
}

// DeleteMastType 删除门架类型
func (r *DictionaryRepository) DeleteMastType(ctx context.Context, id int) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM mast_types WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除门架类型失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// =====================================================
// mast_heights
// =====================================================

// ListMastHeights 列出全部门架高度
func (r *DictionaryRepository) ListMastHeights(ctx context.Context) ([]MastHeight, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, value_mm FROM mast_heights ORDER BY value_mm ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询门架高度失败: %w", err)
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

// CreateMastHeight 新增门架高度
func (r *DictionaryRepository) CreateMastHeight(ctx context.Context, valueMM int) (MastHeight, error) {
	var id int
	err := r.pool.QueryRow(ctx,
		`INSERT INTO mast_heights (value_mm) VALUES ($1) ON CONFLICT (value_mm) DO NOTHING RETURNING id`, valueMM).Scan(&id)
	if err != nil {
		return MastHeight{}, fmt.Errorf("新增门架高度失败: %w", err)
	}
	return MastHeight{ID: id, ValueMM: valueMM}, nil
}

// DeleteMastHeight 删除门架高度
func (r *DictionaryRepository) DeleteMastHeight(ctx context.Context, id int) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM mast_heights WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除门架高度失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// =====================================================
// battery_types
// =====================================================

// ListBatteryTypes 列出全部电池类型
func (r *DictionaryRepository) ListBatteryTypes(ctx context.Context) ([]BatteryTypeDict, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name FROM battery_types ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询电池类型失败: %w", err)
	}
	defer rows.Close()
	out := make([]BatteryTypeDict, 0, 8)
	for rows.Next() {
		var b BatteryTypeDict
		if err := rows.Scan(&b.ID, &b.Name); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ListBatteryTypesByCascade 级联查询电池类型：基于品牌+车型+系列+吨位过滤
// 通过 original_prices 表查询该组合下实际可用的电池类型（去重）
func (r *DictionaryRepository) ListBatteryTypesByCascade(ctx context.Context, brand, vehicleType, series, tonnage string) ([]BatteryTypeDict, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT bt.id, bt.name
		FROM original_prices op
		JOIN battery_types bt ON bt.name = op.battery_type
		WHERE op.brand = $1 AND op.vehicle_type = $2 AND op.series = $3 AND op.tonnage = $4::numeric
		      AND op.battery_type IS NOT NULL AND op.is_active = TRUE
		ORDER BY bt.id ASC`, brand, vehicleType, series, tonnage)
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

// CreateBatteryType 新增电池类型
func (r *DictionaryRepository) CreateBatteryType(ctx context.Context, name string) (BatteryTypeDict, error) {
	var id int
	err := r.pool.QueryRow(ctx,
		`INSERT INTO battery_types (name) VALUES ($1) ON CONFLICT (name) DO NOTHING RETURNING id`, name).Scan(&id)
	if err != nil {
		return BatteryTypeDict{}, fmt.Errorf("新增电池类型失败: %w", err)
	}
	return BatteryTypeDict{ID: id, Name: name}, nil
}

// DeleteBatteryType 删除电池类型
func (r *DictionaryRepository) DeleteBatteryType(ctx context.Context, id int) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM battery_types WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除电池类型失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// =====================================================
// condition_ratings
// =====================================================

// ListConditionRatings 列出全部车况评级
func (r *DictionaryRepository) ListConditionRatings(ctx context.Context) ([]ConditionRating, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, rating, label, base_coefficient FROM condition_ratings ORDER BY base_coefficient DESC`)
	if err != nil {
		return nil, fmt.Errorf("查询车况评级失败: %w", err)
	}
	defer rows.Close()
	out := make([]ConditionRating, 0, 8)
	for rows.Next() {
		var c ConditionRating
		if err := rows.Scan(&c.ID, &c.Rating, &c.Label, &c.BaseCoefficient); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateConditionRating 新增车况评级
func (r *DictionaryRepository) CreateConditionRating(ctx context.Context, rating, label string, base float64) (ConditionRating, error) {
	var id int
	err := r.pool.QueryRow(ctx,
		`INSERT INTO condition_ratings (rating, label, base_coefficient) VALUES ($1, $2, $3)
		 ON CONFLICT (rating) DO UPDATE SET label = EXCLUDED.label, base_coefficient = EXCLUDED.base_coefficient
		 RETURNING id`, rating, label, base).Scan(&id)
	if err != nil {
		return ConditionRating{}, fmt.Errorf("新增车况评级失败: %w", err)
	}
	return ConditionRating{ID: id, Rating: rating, Label: label, BaseCoefficient: base}, nil
}

// UpdateConditionRating 更新车况评级
func (r *DictionaryRepository) UpdateConditionRating(ctx context.Context, id int, label string, base float64) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE condition_ratings SET label = $2, base_coefficient = $3 WHERE id = $1`, id, label, base)
	if err != nil {
		return fmt.Errorf("更新车况评级失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// DeleteConditionRating 删除车况评级
func (r *DictionaryRepository) DeleteConditionRating(ctx context.Context, id int) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM condition_ratings WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除车况评级失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// GetConditionRating 按 rating 查询（供 service 计算 Kc 使用）
func (r *DictionaryRepository) GetConditionRating(ctx context.Context, rating string) (ConditionRating, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, rating, label, base_coefficient FROM condition_ratings WHERE rating = $1`, rating)
	var c ConditionRating
	if err := row.Scan(&c.ID, &c.Rating, &c.Label, &c.BaseCoefficient); err != nil {
		return ConditionRating{}, err
	}
	return c, nil
}

// =====================================================
// region_coefficients
// =====================================================

// ListRegionCoefficients 列出全部区域系数
func (r *DictionaryRepository) ListRegionCoefficients(ctx context.Context, province string) ([]RegionCoefficient, error) {
	var rows pgx.Rows
	var err error
	if province != "" {
		rows, err = r.pool.Query(ctx,
			`SELECT id, province, city, coefficient FROM region_coefficients WHERE province = $1 ORDER BY id ASC`, province)
	} else {
		rows, err = r.pool.Query(ctx, `SELECT id, province, city, coefficient FROM region_coefficients ORDER BY id ASC`)
	}
	if err != nil {
		return nil, fmt.Errorf("查询区域系数失败: %w", err)
	}
	defer rows.Close()
	out := make([]RegionCoefficient, 0, 16)
	for rows.Next() {
		var rc RegionCoefficient
		if err := rows.Scan(&rc.ID, &rc.Province, &rc.City, &rc.Coefficient); err != nil {
			return nil, err
		}
		out = append(out, rc)
	}
	return out, rows.Err()
}

// ListProvinces 列出全部省份（去重）
func (r *DictionaryRepository) ListProvinces(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT DISTINCT province FROM region_coefficients ORDER BY province ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询省份失败: %w", err)
	}
	defer rows.Close()
	out := make([]string, 0, 8)
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListCities 按省份列出城市
func (r *DictionaryRepository) ListCities(ctx context.Context, province string) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT city FROM region_coefficients WHERE province = $1 ORDER BY city ASC`, province)
	if err != nil {
		return nil, fmt.Errorf("查询城市失败: %w", err)
	}
	defer rows.Close()
	out := make([]string, 0, 8)
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateRegionCoefficient 新增区域系数
func (r *DictionaryRepository) CreateRegionCoefficient(ctx context.Context, province, city string, coefficient float64) (RegionCoefficient, error) {
	var id int
	err := r.pool.QueryRow(ctx,
		`INSERT INTO region_coefficients (province, city, coefficient) VALUES ($1, $2, $3)
		 ON CONFLICT (province, city) DO UPDATE SET coefficient = EXCLUDED.coefficient RETURNING id`,
		province, city, coefficient).Scan(&id)
	if err != nil {
		return RegionCoefficient{}, fmt.Errorf("新增区域系数失败: %w", err)
	}
	return RegionCoefficient{ID: id, Province: province, City: city, Coefficient: coefficient}, nil
}

// UpdateRegionCoefficient 更新区域系数
func (r *DictionaryRepository) UpdateRegionCoefficient(ctx context.Context, id int, coefficient float64) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE region_coefficients SET coefficient = $2 WHERE id = $1`, id, coefficient)
	if err != nil {
		return fmt.Errorf("更新区域系数失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// DeleteRegionCoefficient 删除区域系数
func (r *DictionaryRepository) DeleteRegionCoefficient(ctx context.Context, id int) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM region_coefficients WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除区域系数失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// GetRegionCoefficient 按 province + city 查询区域系数（供 service 计算 Km 使用）
// 未命中时返回 pgx.ErrNoRows，由调用方决定是否使用默认值 1.0
func (r *DictionaryRepository) GetRegionCoefficient(ctx context.Context, province, city string) (RegionCoefficient, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, province, city, coefficient FROM region_coefficients WHERE province = $1 AND city = $2`, province, city)
	var rc RegionCoefficient
	if err := row.Scan(&rc.ID, &rc.Province, &rc.City, &rc.Coefficient); err != nil {
		return RegionCoefficient{}, err
	}
	return rc, nil
}

// =====================================================
// original_prices
// =====================================================

// ListOriginalPrices 列出全部原价记录（分页）
func (r *DictionaryRepository) ListOriginalPrices(ctx context.Context, limit, offset int) ([]OriginalPrice, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM original_prices`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计原价记录失败: %w", err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, brand_type, brand, vehicle_type, series, tonnage,
		       config_type, mast_type, mast_height_mm, battery_type, original_price, is_active, updated_at
		FROM original_prices
		ORDER BY id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("查询原价记录失败: %w", err)
	}
	defer rows.Close()
	out := make([]OriginalPrice, 0, limit)
	for rows.Next() {
		var o OriginalPrice
		var updatedAt time.Time
		if err := rows.Scan(&o.ID, &o.BrandType, &o.Brand, &o.VehicleType, &o.Series, &o.Tonnage,
			&o.ConfigType, &o.MastType, &o.MastHeightMM, &o.BatteryType, &o.OriginalPrice, &o.IsActive, &updatedAt); err != nil {
			return nil, 0, err
		}
		o.UpdatedAt = updatedAt.Format("2006-01-02T15:04:05Z07:00")
		out = append(out, o)
	}
	return out, total, rows.Err()
}

// GetOriginalPriceByID 按主键查询原价
func (r *DictionaryRepository) GetOriginalPriceByID(ctx context.Context, id int64) (OriginalPrice, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, brand_type, brand, vehicle_type, series, tonnage,
		       config_type, mast_type, mast_height_mm, battery_type, original_price, is_active, updated_at
		FROM original_prices WHERE id = $1`, id)
	var o OriginalPrice
	var updatedAt time.Time
	if err := row.Scan(&o.ID, &o.BrandType, &o.Brand, &o.VehicleType, &o.Series, &o.Tonnage,
		&o.ConfigType, &o.MastType, &o.MastHeightMM, &o.BatteryType, &o.OriginalPrice, &o.IsActive, &updatedAt); err != nil {
		return OriginalPrice{}, err
	}
	o.UpdatedAt = updatedAt.Format("2006-01-02T15:04:05Z07:00")
	return o, nil
}

// CreateOriginalPrice 新增原价记录
func (r *DictionaryRepository) CreateOriginalPrice(ctx context.Context, o *OriginalPrice) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO original_prices (
			brand_type, brand, vehicle_type, series, tonnage,
			config_type, mast_type, mast_height_mm, battery_type, original_price, is_active
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (brand_type, brand, vehicle_type, series, tonnage,
		             config_type, mast_type, mast_height_mm, battery_type)
		DO UPDATE SET original_price = EXCLUDED.original_price, is_active = EXCLUDED.is_active, updated_at = NOW()
		RETURNING id`,
		o.BrandType, o.Brand, o.VehicleType, o.Series, o.Tonnage,
		o.ConfigType, o.MastType, o.MastHeightMM, nullableStrPtr(o.BatteryType), o.OriginalPrice, o.IsActive).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("新增原价记录失败: %w", err)
	}
	return id, nil
}

// UpdateOriginalPrice 更新原价记录
func (r *DictionaryRepository) UpdateOriginalPrice(ctx context.Context, id int64, price float64, isActive bool) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE original_prices SET original_price = $2, is_active = $3, updated_at = NOW() WHERE id = $1`,
		id, price, isActive)
	if err != nil {
		return fmt.Errorf("更新原价记录失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// DeleteOriginalPrice 删除原价记录
func (r *DictionaryRepository) DeleteOriginalPrice(ctx context.Context, id int64) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM original_prices WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除原价记录失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// FindOriginalPriceMatch 精确匹配原价：按全部 9 个字段查询
// 未命中时返回 pgx.ErrNoRows，由调用方决定是否走模糊匹配
func (r *DictionaryRepository) FindOriginalPriceMatch(
	ctx context.Context, brandType, brand, vehicleType, series string,
	tonnage float64, configType, mastType string, mastHeightMM int, batteryType *string,
) (OriginalPrice, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, brand_type, brand, vehicle_type, series, tonnage,
		       config_type, mast_type, mast_height_mm, battery_type, original_price, is_active, updated_at
		FROM original_prices
		WHERE brand_type = $1 AND brand = $2 AND vehicle_type = $3 AND series = $4
		  AND tonnage = $5 AND config_type = $6 AND mast_type = $7 AND mast_height_mm = $8
		  AND COALESCE(battery_type, '') = COALESCE($9, '')
		  AND is_active = TRUE`,
		brandType, brand, vehicleType, series, tonnage, configType, mastType, mastHeightMM, nullableStrPtr(batteryType))
	var o OriginalPrice
	var updatedAt time.Time
	if err := row.Scan(&o.ID, &o.BrandType, &o.Brand, &o.VehicleType, &o.Series, &o.Tonnage,
		&o.ConfigType, &o.MastType, &o.MastHeightMM, &o.BatteryType, &o.OriginalPrice, &o.IsActive, &updatedAt); err != nil {
		return OriginalPrice{}, err
	}
	o.UpdatedAt = updatedAt.Format("2006-01-02T15:04:05Z07:00")
	return o, nil
}

// FindOriginalPriceFuzzy 模糊匹配原价：按 brand_type + brand + vehicle_type + series + tonnage 查询
// 忽略 config_type / mast_type / mast_height_mm / battery_type
// 当 series 为空字符串时，忽略 series 条件（用于 series="无" 的降级匹配）
// 多条命中时取 original_price 最高的（高配置与标准配置中偏高者，对卖家更友好）
func (r *DictionaryRepository) FindOriginalPriceFuzzy(
	ctx context.Context, brandType, brand, vehicleType, series string, tonnage float64,
) (OriginalPrice, error) {
	var row pgx.Row
	if series == "" {
		// series 为空：忽略 series 条件
		row = r.pool.QueryRow(ctx, `
			SELECT id, brand_type, brand, vehicle_type, series, tonnage,
			       config_type, mast_type, mast_height_mm, battery_type, original_price, is_active, updated_at
			FROM original_prices
			WHERE brand_type = $1 AND brand = $2 AND vehicle_type = $3
			  AND tonnage = $4 AND is_active = TRUE
			ORDER BY original_price DESC LIMIT 1`,
			brandType, brand, vehicleType, tonnage)
	} else {
		row = r.pool.QueryRow(ctx, `
			SELECT id, brand_type, brand, vehicle_type, series, tonnage,
			       config_type, mast_type, mast_height_mm, battery_type, original_price, is_active, updated_at
			FROM original_prices
			WHERE brand_type = $1 AND brand = $2 AND vehicle_type = $3 AND series = $4
			  AND tonnage = $5 AND is_active = TRUE
			ORDER BY original_price DESC LIMIT 1`,
			brandType, brand, vehicleType, series, tonnage)
	}
	var o OriginalPrice
	var updatedAt time.Time
	if err := row.Scan(&o.ID, &o.BrandType, &o.Brand, &o.VehicleType, &o.Series, &o.Tonnage,
		&o.ConfigType, &o.MastType, &o.MastHeightMM, &o.BatteryType, &o.OriginalPrice, &o.IsActive, &updatedAt); err != nil {
		return OriginalPrice{}, err
	}
	o.UpdatedAt = updatedAt.Format("2006-01-02T15:04:05Z07:00")
	return o, nil
}

// =====================================================
// coefficient_configs
// =====================================================

// ListCoefficientConfigs 列出全部系数配置
func (r *DictionaryRepository) ListCoefficientConfigs(ctx context.Context) ([]CoefficientConfig, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, key, value, description, updated_at FROM coefficient_configs ORDER BY key ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询系数配置失败: %w", err)
	}
	defer rows.Close()
	out := make([]CoefficientConfig, 0, 16)
	for rows.Next() {
		var c CoefficientConfig
		var desc *string
		var updatedAt time.Time
		if err := rows.Scan(&c.ID, &c.Key, &c.Value, &desc, &updatedAt); err != nil {
			return nil, err
		}
		if desc != nil {
			c.Description = *desc
		}
		c.UpdatedAt = updatedAt.Format("2006-01-02T15:04:05Z07:00")
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetCoefficientByKey 按 key 查询系数
func (r *DictionaryRepository) GetCoefficientByKey(ctx context.Context, key string) (CoefficientConfig, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, key, value, description, updated_at FROM coefficient_configs WHERE key = $1`, key)
	var c CoefficientConfig
	var desc *string
	var updatedAt time.Time
	if err := row.Scan(&c.ID, &c.Key, &c.Value, &desc, &updatedAt); err != nil {
		return CoefficientConfig{}, err
	}
	if desc != nil {
		c.Description = *desc
	}
	c.UpdatedAt = updatedAt.Format("2006-01-02T15:04:05Z07:00")
	return c, nil
}

// UpdateCoefficientByKey 按 key 更新系数值
func (r *DictionaryRepository) UpdateCoefficientByKey(ctx context.Context, key string, value float64) (CoefficientConfig, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE coefficient_configs SET value = $2, updated_at = NOW() WHERE key = $1
		 RETURNING id, key, value, description, updated_at`, key, value)
	var c CoefficientConfig
	var desc *string
	var updatedAt time.Time
	if err := row.Scan(&c.ID, &c.Key, &c.Value, &desc, &updatedAt); err != nil {
		return CoefficientConfig{}, err
	}
	if desc != nil {
		c.Description = *desc
	}
	c.UpdatedAt = updatedAt.Format("2006-01-02T15:04:05Z07:00")
	return c, nil
}

// =====================================================
// 级联过滤方法：基于 original_prices 表查询有效组合
// 设计：以 original_prices 为真实数据源，DISTINCT 查询各级可选值
// =====================================================

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

// ListVehicleTypesByBrand 按品牌列出可选车辆类型（从 original_prices DISTINCT 查询）
func (r *DictionaryRepository) ListVehicleTypesByBrand(ctx context.Context, brand string) ([]VehicleType, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT vt.id, vt.name, vt.power_type, vt.earliest_factory_year
		FROM original_prices op
		JOIN vehicle_types vt ON vt.name = op.vehicle_type
		WHERE op.brand = $1 AND op.is_active = TRUE
		ORDER BY vt.id ASC`, brand)
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
func (r *DictionaryRepository) ListSeriesByCascade(ctx context.Context, brand, vehicleType string) ([]Series, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT s.id, s.brand, s.name, s.earliest_factory_year
		FROM original_prices op
		JOIN series s ON s.brand = op.brand AND s.name = op.series
		WHERE op.brand = $1 AND op.vehicle_type = $2 AND op.is_active = TRUE
		ORDER BY s.id ASC`, brand, vehicleType)
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

// ListTonnagesByCascade 按品牌+车辆类型+系列级联查询吨位
func (r *DictionaryRepository) ListTonnagesByCascade(ctx context.Context, brand, vehicleType, series string) ([]Tonnage, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT t.id, t.value
		FROM original_prices op
		JOIN tonnages t ON t.value = op.tonnage
		WHERE op.brand = $1 AND op.vehicle_type = $2 AND op.series = $3 AND op.is_active = TRUE
		ORDER BY t.value ASC`, brand, vehicleType, series)
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

// ListConfigTypesByCascade 按品牌+车辆类型+系列+吨位级联查询配置类型
func (r *DictionaryRepository) ListConfigTypesByCascade(ctx context.Context, brand, vehicleType, series, tonnage string) ([]ConfigType, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT c.id, c.name
		FROM original_prices op
		JOIN config_types c ON c.name = op.config_type
		WHERE op.brand = $1 AND op.vehicle_type = $2 AND op.series = $3 AND op.tonnage = $4::numeric AND op.is_active = TRUE
		ORDER BY c.id ASC`, brand, vehicleType, series, tonnage)
	if err != nil {
		return nil, fmt.Errorf("级联查询配置类型失败: %w", err)
	}
	defer rows.Close()
	out := make([]ConfigType, 0, 8)
	for rows.Next() {
		var c ConfigType
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListMastTypesByCascade 按前序层级+配置类型级联查询门架类型
func (r *DictionaryRepository) ListMastTypesByCascade(ctx context.Context, brand, vehicleType, series, tonnage, configType string) ([]MastType, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT m.id, m.name
		FROM original_prices op
		JOIN mast_types m ON m.name = op.mast_type
		WHERE op.brand = $1 AND op.vehicle_type = $2 AND op.series = $3 AND op.tonnage = $4::numeric AND op.config_type = $5 AND op.is_active = TRUE
		ORDER BY m.id ASC`, brand, vehicleType, series, tonnage, configType)
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

// ListMastHeightsByCascade 按前序层级+门架类型级联查询门架高度
func (r *DictionaryRepository) ListMastHeightsByCascade(ctx context.Context, brand, vehicleType, series, tonnage, configType, mastType string) ([]MastHeight, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT mh.id, mh.value_mm
		FROM original_prices op
		JOIN mast_heights mh ON mh.value_mm = op.mast_height_mm
		WHERE op.brand = $1 AND op.vehicle_type = $2 AND op.series = $3 AND op.tonnage = $4::numeric AND op.config_type = $5 AND op.mast_type = $6 AND op.is_active = TRUE
		ORDER BY mh.value_mm ASC`, brand, vehicleType, series, tonnage, configType, mastType)
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

// =====================================================
// 工具函数
// =====================================================

// nullableStrPtr 把 *string 转为 SQL 占位符（nil → NULL）
// 与 battery.go 的 nullableString 不同，此处用于 *string 类型字段
func nullableStrPtr(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}
