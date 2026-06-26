// Package service 实现核心业务逻辑
// 本文件：系数配置加载器
// 从 PostgreSQL 加载并缓存衰减率、权重、市场系数等可调参数
package service

import (
	"context"
	"fmt"
	"sync"

	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
)

// CoefficientLoader 系数配置加载器
// 提供线程安全的懒加载与刷新能力
type CoefficientLoader struct {
	queries *repository.Queries

	mu     sync.RWMutex
	cached map[string]float64
}

// NewCoefficientLoader 构造系数加载器
func NewCoefficientLoader(q *repository.Queries) *CoefficientLoader {
	return &CoefficientLoader{
		queries: q,
		cached:  make(map[string]float64),
	}
}

// LoadAll 加载所有系数到内存缓存
// 应用启动时调用一次即可，后续业务计算走缓存
func (c *CoefficientLoader) LoadAll(ctx context.Context) error {
	rows, err := c.queries.ListCoefficientConfigs(ctx)
	if err != nil {
		return fmt.Errorf("加载系数配置失败: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	// 重置缓存，避免删除的 key 残留
	c.cached = make(map[string]float64, len(rows))
	for _, r := range rows {
		c.cached[r.Key] = r.Value
	}
	return nil
}

// Get 从缓存中读取某个系数
// 未找到时返回 model.ErrCoefficientNotFound
func (c *CoefficientLoader) Get(key string) (float64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.cached[key]
	if !ok {
		return 0, fmt.Errorf("%w: %s", model.ErrCoefficientNotFound, key)
	}
	return v, nil
}

// MustGet 读取系数，找不到则 panic（仅用于内部必填系数）
func (c *CoefficientLoader) MustGet(key string) float64 {
	v, err := c.Get(key)
	if err != nil {
		panic(err)
	}
	return v
}

// Reload 重新加载（用于系数更新后热刷新）
func (c *CoefficientLoader) Reload(ctx context.Context) error {
	return c.LoadAll(ctx)
}

// Keys 系数键常量集中定义，避免散落字符串
const (
	KeyLambdaElectric   = "lambda_electric"   // 电动叉车时间衰减率 λ
	KeyLambdaCombustion = "lambda_combustion" // 内燃叉车时间衰减率 λ
	KeyWWorkCondition   = "w_work_condition"  // 工况权重 w₁
	KeyWBrand           = "w_brand"           // 品牌权重 w₂
	KeyWCondition       = "w_condition"       // 车况权重 w₃
	KeyWMarket          = "w_market"          // 市场权重 w₄
	KeyKMarket          = "k_market"          // 市场系数 Km
	KeyConfidenceRange  = "confidence_range"  // 95% 置信水平对应的置信区间范围 ±5%
)
