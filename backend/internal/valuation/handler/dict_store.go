// Package handler 实现 HTTP 处理器
// 本文件：字典配置存储接口（ConfigHandler 消费的字典契约，ADR-0008 收窄后的最终面）。
package handler

import (
	"context"
	"forklift-training/internal/valuation"
)

// DictWriter 描述符驱动字典写面（ADR-0008）：机械 CRUD 塌缩后的窄面。
// 生产为 valuation.DictStore（DictionaryRepository 嵌入），测试为内存替身。
// UpdateByKey 供按 key 更新的实体使用（coefficient_configs，PUT /:key，返回完整行）。
type DictWriter interface {
	Create(ctx context.Context, d valuation.DictDescriptor, fields map[string]any) (int64, error)
	Update(ctx context.Context, d valuation.DictDescriptor, id int64, fields map[string]any) error
	UpdateByKey(ctx context.Context, d valuation.DictDescriptor, key string, fields map[string]any) (map[string]any, error)
	Delete(ctx context.Context, d valuation.DictDescriptor, id int64) error
}

// DictionaryConfigStore 字典配置存储接口（ConfigHandler 消费；生产为 pgx 仓储，测试为内存替身）。
// 读面 = valuation.DictionaryReader + 学生端字典查询 typed 方法（30+，形状异构不强求通用化）；
// 写面 = DictWriter（描述符驱动，全部实体的 CRUD 写操作塌缩到 4 个方法）。
type DictionaryConfigStore interface {
	valuation.DictionaryReader
	DictWriter

	ListBrands(ctx context.Context) ([]valuation.Brand, error)
	ListVehicleTypes(ctx context.Context) ([]valuation.VehicleType, error)
	ListVehicleTypesByBrand(ctx context.Context, brand string) ([]valuation.VehicleType, error)
	ListSeries(ctx context.Context, brand string) ([]valuation.Series, error)
	ListSeriesByCascade(ctx context.Context, brand, vehicleType string) ([]valuation.Series, error)
	ListSeriesConfigOptions(ctx context.Context, brand, series string) (valuation.SeriesConfigOptions, error)
	ListTonnages(ctx context.Context) ([]valuation.Tonnage, error)
	ListTonnagesByCascade(ctx context.Context, brand, vehicleType, series string) ([]valuation.Tonnage, error)
	ListConfigOptionsByCascade(ctx context.Context, brand, vehicleType, series, tonnage string) ([]valuation.ConfigOption, error)
	ListMastTypes(ctx context.Context) ([]valuation.MastType, error)
	ListMastTypesByCascade(ctx context.Context, brand, vehicleType, series, tonnage, configType string) ([]valuation.MastType, error)
	ListMastHeights(ctx context.Context) ([]valuation.MastHeight, error)
	ListMastHeightsByCascade(ctx context.Context, brand, vehicleType, series, tonnage, configType, mastType string) ([]valuation.MastHeight, error)
	ListBatteryTypes(ctx context.Context) ([]valuation.BatteryTypeDict, error)
	ListBatteryTypesByCascade(ctx context.Context, brand, vehicleType, series, tonnage string) ([]valuation.BatteryTypeDict, error)
	ListTransmissionTypes(ctx context.Context) ([]valuation.TransmissionType, error)
	ListEngineTypes(ctx context.Context) ([]valuation.EngineType, error)
	ListConditionRatings(ctx context.Context) ([]valuation.ConditionRating, error)
	ListRegionCoefficients(ctx context.Context, province string) ([]valuation.RegionCoefficient, error)
	ListProvinces(ctx context.Context) ([]string, error)
	ListCities(ctx context.Context, province string) ([]string, error)
	ListOriginalPrices(ctx context.Context, limit, offset int) ([]valuation.OriginalPrice, int, error)
	ListCoefficientConfigs(ctx context.Context) ([]valuation.CoefficientConfig, error)
	ListAlgorithmParameters(ctx context.Context) (valuation.AlgorithmParameters, error)
	GetEarliestFactoryYearByCascade(ctx context.Context, brand, vehicleType, series string, tonnage float64) (int, error)
}
