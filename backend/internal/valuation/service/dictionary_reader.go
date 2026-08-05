// Package service 实现核心业务逻辑
// 本文件：系数表读取窄接口——service 只依赖 interface 而非具体仓储，
// 公式计算（Kt/Kh/Kb/Kc/Km）由此脱离 Postgres，可纯内存单测。
package service

import (
	"context"

	"forklift-training/internal/valuation/repository"
)

// ConfigReader 系数键读取接口（Kt/Kh/Kc 只依赖按 key 读系数）。
type ConfigReader interface {
	Get(ctx context.Context, key string) (float64, error)
}

// DictionaryReader 系数表读取接口（评估服务与 K 系数计算消费的窄 interface）。
type DictionaryReader interface {
	GetBrandByName(ctx context.Context, name string) (repository.Brand, error)
	GetConditionRating(ctx context.Context, rating string) (repository.ConditionRating, error)
	GetRegionCoefficient(ctx context.Context, province, city string) (repository.RegionCoefficient, error)
	GetVehicleTypeByName(ctx context.Context, name string) (repository.VehicleType, error)
	FindOriginalPriceMatch(ctx context.Context, brand, vehicleType, series string, tonnage float64, configType, mastType string, mastHeightMM int) (repository.OriginalPrice, error)
	FindOriginalPriceFuzzy(ctx context.Context, brand, vehicleType, series string, tonnage float64) (repository.OriginalPrice, error)
	GetCoefficientByKey(ctx context.Context, key string) (repository.CoefficientConfig, error)
}
