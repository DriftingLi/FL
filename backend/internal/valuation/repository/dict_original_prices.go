// original_prices 原价表 CRUD 与精确/模糊匹配（从 dictionaries.go 拆分）
// 手写 pgx 仓储，统一使用 *pgxpool.Pool 直接操作
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"forklift-training/internal/cache"
)

// ListOriginalPrices 列出全部原价记录（分页）
func (r *DictionaryRepository) ListOriginalPrices(ctx context.Context, limit, offset int) ([]OriginalPrice, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM original_prices`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计原价记录失败: %w", err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, brand, vehicle_type, series, tonnage,
		       config_type, mast_type, mast_height_mm, earliest_factory_year,
		       original_price, updated_at
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
		if err := rows.Scan(&o.ID, &o.Brand, &o.VehicleType, &o.Series, &o.Tonnage,
			&o.ConfigType, &o.MastType, &o.MastHeightMM, &o.EarliestFactoryYear,
			&o.OriginalPrice, &updatedAt); err != nil {
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
		SELECT id, brand, vehicle_type, series, tonnage,
		       config_type, mast_type, mast_height_mm, earliest_factory_year,
		       original_price, updated_at
		FROM original_prices WHERE id = $1`, id)
	var o OriginalPrice
	var updatedAt time.Time
	if err := row.Scan(&o.ID, &o.Brand, &o.VehicleType, &o.Series, &o.Tonnage,
		&o.ConfigType, &o.MastType, &o.MastHeightMM, &o.EarliestFactoryYear,
		&o.OriginalPrice, &updatedAt); err != nil {
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
			brand, vehicle_type, series, tonnage,
			config_type, mast_type, mast_height_mm, earliest_factory_year,
			original_price
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (brand, vehicle_type, series, tonnage,
		             config_type, mast_type, mast_height_mm)
		DO UPDATE SET earliest_factory_year = EXCLUDED.earliest_factory_year,
		              original_price = EXCLUDED.original_price, updated_at = NOW()
		RETURNING id`,
		o.Brand, o.VehicleType, o.Series, o.Tonnage,
		o.ConfigType, o.MastType, o.MastHeightMM, o.EarliestFactoryYear,
		o.OriginalPrice).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("新增原价记录失败: %w", err)
	}
	return id, nil
}

// UpdateOriginalPrice 更新原价记录的全部可编辑字段
// 包含 7 个唯一约束字段（brand/vehicle_type/series/tonnage/config_type/mast_type/mast_height_mm）
// 以及 earliest_factory_year、original_price；若新值触发 7 字段唯一约束冲突，返回原始 pgx 错误
func (r *DictionaryRepository) UpdateOriginalPrice(ctx context.Context, o *OriginalPrice) error {
	ct, err := r.pool.Exec(ctx, `
		UPDATE original_prices SET
			brand = $2, vehicle_type = $3, series = $4, tonnage = $5,
			config_type = $6, mast_type = $7, mast_height_mm = $8,
			earliest_factory_year = $9, original_price = $10,
			updated_at = NOW()
		WHERE id = $1`,
		o.ID, o.Brand, o.VehicleType, o.Series, o.Tonnage,
		o.ConfigType, o.MastType, o.MastHeightMM, o.EarliestFactoryYear,
		o.OriginalPrice)
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

// FindOriginalPriceMatch 精确匹配原价：按 7 个字段查询
// 未命中时返回 pgx.ErrNoRows，由调用方决定是否走模糊匹配
func (r *DictionaryRepository) FindOriginalPriceMatch(
	ctx context.Context, brand, vehicleType, series string,
	tonnage float64, configType, mastType string, mastHeightMM int,
) (OriginalPrice, error) {
	cacheKey := cache.SafeKey("dict", "op", "match", brand, vehicleType, series,
		fmt.Sprintf("%v", tonnage), configType, mastType, fmt.Sprintf("%d", mastHeightMM))
	var result OriginalPrice
		err := cache.GetOrSetJSON(ctx, cacheKey, cache.TTLDictionary, &result, func() (any, error) {
		row := r.pool.QueryRow(ctx, `
			SELECT id, brand, vehicle_type, series, tonnage,
			       config_type, mast_type, mast_height_mm, earliest_factory_year, original_price, updated_at
			FROM original_prices
			WHERE brand = $1 AND vehicle_type = $2 AND series = $3
			  AND tonnage = $4 AND config_type = $5 AND mast_type = $6 AND mast_height_mm = $7`,
			brand, vehicleType, series, tonnage, configType, mastType, mastHeightMM)
		var o OriginalPrice
		var updatedAt time.Time
		if err := row.Scan(&o.ID, &o.Brand, &o.VehicleType, &o.Series, &o.Tonnage,
			&o.ConfigType, &o.MastType, &o.MastHeightMM, &o.EarliestFactoryYear, &o.OriginalPrice, &updatedAt); err != nil {
			return nil, err
		}
		o.UpdatedAt = updatedAt.Format("2006-01-02T15:04:05Z07:00")
		return o, nil
	})
	return result, err
}

// FindOriginalPriceFuzzy 模糊匹配原价：按 brand + vehicle_type + series + tonnage 查询
// 忽略 config_type / mast_type / mast_height_mm
// 当 series 为空字符串时，忽略 series 条件（用于 series="其它" 的降级匹配）
// 多条命中时取 original_price 最高的（高配置与标准配置中偏高者，对卖家更友好）
func (r *DictionaryRepository) FindOriginalPriceFuzzy(
	ctx context.Context, brand, vehicleType, series string, tonnage float64,
) (OriginalPrice, error) {
	cacheKey := cache.SafeKey("dict", "op", "fuzzy", brand, vehicleType, series, fmt.Sprintf("%v", tonnage))
	var result OriginalPrice
	err := cache.GetOrSetJSON(ctx, cacheKey, cache.TTLDictionary, &result, func() (any, error) {
		var row pgx.Row
		if series == "" {
			// series 为空：忽略 series 条件
			row = r.pool.QueryRow(ctx, `
				SELECT id, brand, vehicle_type, series, tonnage,
			       config_type, mast_type, mast_height_mm, earliest_factory_year, original_price, updated_at
				FROM original_prices
				WHERE brand = $1 AND vehicle_type = $2
				  AND tonnage = $3
				ORDER BY original_price DESC LIMIT 1`,
				brand, vehicleType, tonnage)
		} else {
			row = r.pool.QueryRow(ctx, `
				SELECT id, brand, vehicle_type, series, tonnage,
			       config_type, mast_type, mast_height_mm, earliest_factory_year, original_price, updated_at
				FROM original_prices
				WHERE brand = $1 AND vehicle_type = $2 AND series = $3
				  AND tonnage = $4
				ORDER BY original_price DESC LIMIT 1`,
				brand, vehicleType, series, tonnage)
		}
		var o OriginalPrice
		var updatedAt time.Time
		if err := row.Scan(&o.ID, &o.Brand, &o.VehicleType, &o.Series, &o.Tonnage,
			&o.ConfigType, &o.MastType, &o.MastHeightMM, &o.EarliestFactoryYear, &o.OriginalPrice, &updatedAt); err != nil {
			return nil, err
		}
		o.UpdatedAt = updatedAt.Format("2006-01-02T15:04:05Z07:00")
		return o, nil
	})
	return result, err
}
