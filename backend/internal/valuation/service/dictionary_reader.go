// Package service 实现核心业务逻辑
// 本文件：系数表读取窄接口——service 只依赖 interface 而非具体仓储，
// 公式计算（Kt/Kh/Kb/Kc/Km）由此脱离 Postgres，可纯内存单测。
package service

import (
	"context"
	"fmt"

	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
)

// DictionaryReader 系数表读取接口（评估服务与 K 系数计算消费的窄 interface）。
type DictionaryReader interface {
	GetBrandByName(ctx context.Context, name string) (repository.Brand, error)
	GetConditionRating(ctx context.Context, rating string) (repository.ConditionRating, error)
	GetRegionCoefficient(ctx context.Context, province, city string) (repository.RegionCoefficient, error)
	GetVehicleTypeByName(ctx context.Context, name string) (repository.VehicleType, error)
	FindOriginalPriceMatch(ctx context.Context, brand, vehicleType, series string, tonnage float64, configType, mastType string, mastHeightMM int) (repository.OriginalPrice, error)
	FindOriginalPriceFuzzy(ctx context.Context, brand, vehicleType, series string, tonnage float64) (repository.OriginalPrice, error)
	GetCoefficientByKey(ctx context.Context, key string) (repository.CoefficientConfig, error)
	// ListCoefficientConfigs 系数配置全表读取（快照加载用，一次缓存往返）。
	ListCoefficientConfigs(ctx context.Context) ([]repository.CoefficientConfig, error)
}

// λ 兜底默认值（与迁移种子一致；仅在系数配置缺失时用于评估结果锁定的 λ 字段）。
const (
	defaultLambdaElectric   = 0.12
	defaultLambdaCombustion = 0.10
)

// CoefficientSnapshot 系数配置快照：一次读取全表后按 key 提供系数，
// 替代评估流程内逐 key 串行缓存往返（一次评估约 15 次 → 1 次）。
type CoefficientSnapshot struct {
	values map[string]float64
}

// NewCoefficientSnapshot 由全表系数配置构造快照。
func NewCoefficientSnapshot(configs []repository.CoefficientConfig) *CoefficientSnapshot {
	m := make(map[string]float64, len(configs))
	for _, c := range configs {
		m[c.Key] = c.Value
	}
	return &CoefficientSnapshot{values: m}
}

// Get 从快照按 key 读取（无 I/O）；缺失返回 ErrCoefficientNotFound（与 provider 语义一致）。
func (s *CoefficientSnapshot) Get(_ context.Context, key string) (float64, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return 0, fmt.Errorf("%w: %s", model.ErrCoefficientNotFound, key)
}

// ReadFloat 从快照读取系数，失败或非正数时返回 fallback（与 provider 语义一致）。
func (s *CoefficientSnapshot) ReadFloat(ctx context.Context, key string, fallback float64) float64 {
	return coefReadFloat(ctx, s, key, fallback)
}

// LoadCoefficientSnapshot 一次加载系数快照；读取失败返回错误（调用方回退逐 key provider）。
func LoadCoefficientSnapshot(ctx context.Context, reader DictionaryReader) (*CoefficientSnapshot, error) {
	configs, err := reader.ListCoefficientConfigs(ctx)
	if err != nil {
		return nil, err
	}
	return NewCoefficientSnapshot(configs), nil
}
