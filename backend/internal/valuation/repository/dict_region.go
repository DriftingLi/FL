// region_coefficients 区域系数 字典 CRUD（骨架走 dict_helpers 共享实现）
package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// ListRegionCoefficients 列出全部区域系数（可按 province 筛选）
func (r *DictionaryRepository) ListRegionCoefficients(ctx context.Context, province string) ([]RegionCoefficient, error) {
	query := `SELECT id, province, city, coefficient FROM region_coefficients ORDER BY id ASC`
	args := []any{}
	if province != "" {
		query = `SELECT id, province, city, coefficient FROM region_coefficients WHERE province = $1 ORDER BY id ASC`
		args = append(args, province)
	}
	return listCached(r, ctx, cacheKey(CachePrefixRegionList, province), "查询区域系数", query,
		func(rows pgx.Rows) (RegionCoefficient, error) {
			var rc RegionCoefficient
			err := rows.Scan(&rc.ID, &rc.Province, &rc.City, &rc.Coefficient)
			return rc, err
		}, args...)
}

// ListProvinces 列出全部省份（去重）
func (r *DictionaryRepository) ListProvinces(ctx context.Context) ([]string, error) {
	return listCached(r, ctx, "dict:region:provinces", "查询省份",
		`SELECT DISTINCT province FROM region_coefficients ORDER BY province ASC`,
		func(rows pgx.Rows) (string, error) {
			var p string
			err := rows.Scan(&p)
			return p, err
		})
}

// ListCities 按省份列出城市
func (r *DictionaryRepository) ListCities(ctx context.Context, province string) ([]string, error) {
	return listCached(r, ctx, cacheKey(CachePrefixRegionCities, province), "查询城市",
		`SELECT city FROM region_coefficients WHERE province = $1 ORDER BY city ASC`,
		func(rows pgx.Rows) (string, error) {
			var c string
			err := rows.Scan(&c)
			return c, err
		}, province)
}

// CreateRegionCoefficient 新增区域系数
func (r *DictionaryRepository) CreateRegionCoefficient(ctx context.Context, province, city string, coefficient float64) (RegionCoefficient, error) {
	id, err := insertReturningID(r, ctx, "新增区域系数",
		`INSERT INTO region_coefficients (province, city, coefficient) VALUES ($1, $2, $3)
		 ON CONFLICT (province, city) DO UPDATE SET coefficient = EXCLUDED.coefficient RETURNING id`,
		province, city, coefficient)
	if err != nil {
		return RegionCoefficient{}, err
	}
	return RegionCoefficient{ID: int(id), Province: province, City: city, Coefficient: coefficient}, nil
}

// UpdateRegionCoefficient 更新区域系数
func (r *DictionaryRepository) UpdateRegionCoefficient(ctx context.Context, id int, coefficient float64) error {
	return execWrite(r, ctx, "更新区域系数",
		`UPDATE region_coefficients SET coefficient = $2 WHERE id = $1`, id, coefficient)
}

// DeleteRegionCoefficient 删除区域系数
func (r *DictionaryRepository) DeleteRegionCoefficient(ctx context.Context, id int) error {
	return execWrite(r, ctx, "删除区域系数", `DELETE FROM region_coefficients WHERE id = $1`, id)
}

// GetRegionCoefficient 按 province + city 查询区域系数（供 service 计算 Km 使用）
// 未命中时返回 pgx.ErrNoRows，由调用方决定是否使用默认值 1.0
func (r *DictionaryRepository) GetRegionCoefficient(ctx context.Context, province, city string) (RegionCoefficient, error) {
	return getCached(r, ctx, cacheKey(CachePrefixRegionGet, province, city), "查询区域系数",
		`SELECT id, province, city, coefficient FROM region_coefficients WHERE province = $1 AND city = $2`,
		func(row pgx.Row) (RegionCoefficient, error) {
			var rc RegionCoefficient
			err := row.Scan(&rc.ID, &rc.Province, &rc.City, &rc.Coefficient)
			return rc, err
		}, province, city)
}
