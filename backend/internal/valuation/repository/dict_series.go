// series 系列 与 series_config_options 维度映射（骨架走 dict_helpers 共享实现）
package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// ListSeries 列出全部系列（可按 brand 筛选）
func (r *DictionaryRepository) ListSeries(ctx context.Context, brand string) ([]Series, error) {
	query := `SELECT id, brand, name, earliest_factory_year FROM series ORDER BY id ASC`
	args := []any{}
	if brand != "" {
		query = `SELECT id, brand, name, earliest_factory_year FROM series WHERE brand = $1 ORDER BY id ASC`
		args = append(args, brand)
	}
	return listCached(r, ctx, cacheKey(CachePrefixSeriesByBrand, brand), "查询系列", query,
		func(rows pgx.Rows) (Series, error) {
			var s Series
			err := rows.Scan(&s.ID, &s.Brand, &s.Name, &s.EarliestFactoryYear)
			return s, err
		}, args...)
}

// CreateSeries 新增系列
func (r *DictionaryRepository) CreateSeries(ctx context.Context, brand, name string, earliestFactoryYear int) (Series, error) {
	id, err := insertReturningID(r, ctx, "新增系列",
		`INSERT INTO series (brand, name, earliest_factory_year) VALUES ($1, $2, $3) ON CONFLICT (brand, name) DO NOTHING RETURNING id`,
		brand, name, earliestFactoryYear)
	if err != nil {
		return Series{}, err
	}
	return Series{ID: int(id), Brand: brand, Name: name, EarliestFactoryYear: earliestFactoryYear}, nil
}

// UpdateSeries 更新系列
func (r *DictionaryRepository) UpdateSeries(ctx context.Context, id int, brand, name string, earliestFactoryYear int) error {
	return execWrite(r, ctx, "更新系列",
		`UPDATE series SET brand = $2, name = $3, earliest_factory_year = $4 WHERE id = $1`,
		id, brand, name, earliestFactoryYear)
}

// DeleteSeries 删除系列
func (r *DictionaryRepository) DeleteSeries(ctx context.Context, id int) error {
	return execWrite(r, ctx, "删除系列", `DELETE FROM series WHERE id = $1`, id)
}

// ListSeriesConfigOptions 查询指定 series 支持的配置维度及可选项
// 返回三个维度的可选项列表；列表为空表示该 series 不支持此维度
func (r *DictionaryRepository) ListSeriesConfigOptions(ctx context.Context, brand, series string) (SeriesConfigOptions, error) {
	type scoRow struct {
		dimension  string
		optionName string
	}
	rows, err := listCached(r, ctx, cacheKey(CachePrefixSco, brand, series), "查询系列配置选项",
		`SELECT dimension, option_name FROM series_config_options
		 WHERE brand = $1 AND series = $2
		 ORDER BY dimension ASC, id ASC`,
		func(rows pgx.Rows) (scoRow, error) {
			var d scoRow
			err := rows.Scan(&d.dimension, &d.optionName)
			return d, err
		}, brand, series)
	if err != nil {
		return SeriesConfigOptions{}, err
	}
	out := SeriesConfigOptions{
		Transmission: make([]string, 0, 4),
		Engine:       make([]string, 0, 4),
		Battery:      make([]string, 0, 4),
	}
	for _, r := range rows {
		switch r.dimension {
		case "transmission":
			out.Transmission = append(out.Transmission, r.optionName)
		case "engine":
			out.Engine = append(out.Engine, r.optionName)
		case "battery":
			out.Battery = append(out.Battery, r.optionName)
		}
	}
	return out, nil
}
