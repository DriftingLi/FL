// coefficient_configs 系数配置 CRUD 与算法参数聚合（从 dictionaries.go 拆分）
// 手写 pgx 仓储，统一使用 *pgxpool.Pool 直接操作
package repository

import (
	"context"
	"fmt"
	"time"
)

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

// AlgorithmParameters 算法参数聚合结果（管理员后台「算法参数」tab 一次加载）
type AlgorithmParameters struct {
	Coefficients       []CoefficientConfig `json:"coefficients"`
	Brands             []Brand             `json:"brands"`
	ConditionRatings   []ConditionRating   `json:"condition_ratings"`
	RegionCoefficients []RegionCoefficient `json:"region_coefficients"`
}

// ListAlgorithmParameters 聚合查询全部算法参数（4 类），供管理员后台一次加载
func (r *DictionaryRepository) ListAlgorithmParameters(ctx context.Context) (AlgorithmParameters, error) {
	var result AlgorithmParameters
	var err error
	if result.Coefficients, err = r.ListCoefficientConfigs(ctx); err != nil {
		return result, fmt.Errorf("查询算法系数失败: %w", err)
	}
	if result.Brands, err = r.ListBrands(ctx); err != nil {
		return result, fmt.Errorf("查询品牌系数失败: %w", err)
	}
	if result.ConditionRatings, err = r.ListConditionRatings(ctx); err != nil {
		return result, fmt.Errorf("查询车况系数失败: %w", err)
	}
	if result.RegionCoefficients, err = r.ListRegionCoefficients(ctx, ""); err != nil {
		return result, fmt.Errorf("查询区域系数失败: %w", err)
	}
	return result, nil
}
