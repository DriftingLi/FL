// series 系列 与 series_config_options 维度映射（从 dictionaries.go 拆分）
// 手写 pgx 仓储，统一使用 *pgxpool.Pool 直接操作
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"forklift-training/internal/cache"
)

// ListSeries 列出全部系列（可按 brand 筛选）
func (r *DictionaryRepository) ListSeries(ctx context.Context, brand string) ([]Series, error) {
	cacheKey := cache.SafeKey("dict", "series", "bybrand", brand)
	var result []Series
	err := cache.GetOrSetJSON(ctx, cacheKey, cache.TTLDictionary, &result, func() (interface{}, error) {
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
	})
	return result, err
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

// ListSeriesConfigOptions 查询指定 series 支持的配置维度及可选项
// 返回三个维度的可选项列表；列表为空表示该 series 不支持此维度
func (r *DictionaryRepository) ListSeriesConfigOptions(ctx context.Context, brand, series string) (SeriesConfigOptions, error) {
	cacheKey := cache.SafeKey("dict", "sco", brand, series)
	var result SeriesConfigOptions
	err := cache.GetOrSetJSON(ctx, cacheKey, cache.TTLDictionary, &result, func() (interface{}, error) {
		rows, err := r.pool.Query(ctx, `
			SELECT dimension, option_name FROM series_config_options
			WHERE brand = $1 AND series = $2
			ORDER BY dimension ASC, id ASC`, brand, series)
		if err != nil {
			return nil, fmt.Errorf("查询系列配置选项失败: %w", err)
		}
		defer rows.Close()
		out := SeriesConfigOptions{
			Transmission: make([]string, 0, 4),
			Engine:       make([]string, 0, 4),
			Battery:      make([]string, 0, 4),
		}
		for rows.Next() {
			var dimension, optionName string
			if err := rows.Scan(&dimension, &optionName); err != nil {
				return nil, err
			}
			switch dimension {
			case "transmission":
				out.Transmission = append(out.Transmission, optionName)
			case "engine":
				out.Engine = append(out.Engine, optionName)
			case "battery":
				out.Battery = append(out.Battery, optionName)
			}
		}
		return out, rows.Err()
	})
	return result, err
}
