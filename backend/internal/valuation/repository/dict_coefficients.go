// coefficient_configs 系数配置 CRUD 与算法参数聚合（骨架走 dict_helpers 共享实现）
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/sync/errgroup"
)

// scanCoefficient 扫描一行系数配置（description 可空 + updated_at 格式化）。
func scanCoefficient(row interface{ Scan(dest ...any) error }) (CoefficientConfig, error) {
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

// ListCoefficientConfigs 列出全部系数配置
func (r *DictionaryRepository) ListCoefficientConfigs(ctx context.Context) ([]CoefficientConfig, error) {
	return listCached(r, ctx, CacheKeyCoefList, "查询系数配置",
		readSpecCoefList.selectSQL(),
		func(rows pgx.Rows) (CoefficientConfig, error) {
			return scanCoefficient(rows)
		})
}

// GetCoefficientByKey 按 key 查询系数
func (r *DictionaryRepository) GetCoefficientByKey(ctx context.Context, key string) (CoefficientConfig, error) {
	return getCached(r, ctx, cacheKey(CachePrefixCoefGet, key), "查询系数配置",
		readSpecCoefGet.selectSQL(),
		func(row pgx.Row) (CoefficientConfig, error) {
			return scanCoefficient(row)
		}, key)
}

// 注：coefficient_configs 的写操作（按 key 更新）已迁至描述符驱动核心
// （dictcrud.CoefficientConfigDescriptor + dictcrud.Store.UpdateByKey，见 ADR-0008）。

// AlgorithmParameters 算法参数聚合结果（管理员后台「算法参数」tab 一次加载）
type AlgorithmParameters struct {
	Coefficients       []CoefficientConfig `json:"coefficients"`
	Brands             []Brand             `json:"brands"`
	ConditionRatings   []ConditionRating   `json:"condition_ratings"`
	RegionCoefficients []RegionCoefficient `json:"region_coefficients"`
}

// ListAlgorithmParameters 聚合查询全部算法参数（4 类），供管理员后台一次加载。
// 各类子查询各自已缓存（ListCoefficientConfigs/ListBrands/ListConditionRatings/
// ListRegionCoefficients 均走 listCached），本方法不再包一层聚合缓存——消除嵌套复读
// 与失效契约漏覆盖（ADR-0013 候选 2）。
// 4 路查询并行执行（errgroup），单次聚合延迟由串行 4*RTT 降为 1*RTT（缓存命中时）。
func (r *DictionaryRepository) ListAlgorithmParameters(ctx context.Context) (AlgorithmParameters, error) {
	g, gCtx := errgroup.WithContext(ctx)
	var coeffs []CoefficientConfig
	var brands []Brand
	var conditions []ConditionRating
	var regions []RegionCoefficient

	g.Go(func() error {
		var err error
		coeffs, err = r.ListCoefficientConfigs(gCtx)
		if err != nil {
			return fmt.Errorf("查询算法系数失败: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		var err error
		brands, err = r.ListBrands(gCtx)
		if err != nil {
			return fmt.Errorf("查询品牌系数失败: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		var err error
		conditions, err = r.ListConditionRatings(gCtx)
		if err != nil {
			return fmt.Errorf("查询车况系数失败: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		var err error
		regions, err = r.ListRegionCoefficients(gCtx, "")
		if err != nil {
			return fmt.Errorf("查询区域系数失败: %w", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return AlgorithmParameters{}, err
	}
	return AlgorithmParameters{
		Coefficients:       coeffs,
		Brands:             brands,
		ConditionRatings:   conditions,
		RegionCoefficients: regions,
	}, nil
}
