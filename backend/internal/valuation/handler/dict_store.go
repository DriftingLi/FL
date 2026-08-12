// Package handler 实现 HTTP 处理器
// 本文件：字典配置存储接口（ConfigHandler 消费的字典契约，ADR-0008 收窄后的最终面）。
package handler

import (
	"context"

	"forklift-training/internal/valuation/dictcrud"
	"forklift-training/internal/valuation/repository"
	"forklift-training/internal/valuation/service"
)

// DictWriter 描述符驱动字典写面（ADR-0008）：机械 CRUD 塌缩后的窄面。
// 生产为 dictcrud.Store（DictionaryRepository 嵌入），测试为内存替身。
// UpdateByKey 供按 key 更新的实体使用（coefficient_configs，PUT /:key，返回完整行）。
type DictWriter interface {
	Create(ctx context.Context, d dictcrud.Descriptor, fields map[string]any) (int64, error)
	Update(ctx context.Context, d dictcrud.Descriptor, id int64, fields map[string]any) error
	UpdateByKey(ctx context.Context, d dictcrud.Descriptor, key string, fields map[string]any) (map[string]any, error)
	Delete(ctx context.Context, d dictcrud.Descriptor, id int64) error
}

// DictionaryConfigStore 字典配置存储接口（ConfigHandler 消费；生产为 pgx 仓储，测试为内存替身）。
// 读面 = service.DictionaryReader + 学生端字典查询 typed 方法（30+，形状异构不强求通用化）；
// 写面 = DictWriter（描述符驱动，全部实体的 CRUD 写操作塌缩到 4 个方法）。
type DictionaryConfigStore interface {
	service.DictionaryReader
	DictWriter

	ListBrands(ctx context.Context) ([]repository.Brand, error)
	ListVehicleTypes(ctx context.Context) ([]repository.VehicleType, error)
	ListVehicleTypesByBrand(ctx context.Context, brand string) ([]repository.VehicleType, error)
	ListSeries(ctx context.Context, brand string) ([]repository.Series, error)
	ListSeriesByCascade(ctx context.Context, brand, vehicleType string) ([]repository.Series, error)
	ListSeriesConfigOptions(ctx context.Context, brand, series string) (repository.SeriesConfigOptions, error)
	ListTonnages(ctx context.Context) ([]repository.Tonnage, error)
	ListTonnagesByCascade(ctx context.Context, brand, vehicleType, series string) ([]repository.Tonnage, error)
	ListConfigOptionsByCascade(ctx context.Context, brand, vehicleType, series, tonnage string) ([]repository.ConfigOption, error)
	ListMastTypes(ctx context.Context) ([]repository.MastType, error)
	ListMastTypesByCascade(ctx context.Context, brand, vehicleType, series, tonnage, configType string) ([]repository.MastType, error)
	ListMastHeights(ctx context.Context) ([]repository.MastHeight, error)
	ListMastHeightsByCascade(ctx context.Context, brand, vehicleType, series, tonnage, configType, mastType string) ([]repository.MastHeight, error)
	ListBatteryTypes(ctx context.Context) ([]repository.BatteryTypeDict, error)
	ListBatteryTypesByCascade(ctx context.Context, brand, vehicleType, series, tonnage string) ([]repository.BatteryTypeDict, error)
	ListTransmissionTypes(ctx context.Context) ([]repository.TransmissionType, error)
	ListEngineTypes(ctx context.Context) ([]repository.EngineType, error)
	ListConditionRatings(ctx context.Context) ([]repository.ConditionRating, error)
	ListRegionCoefficients(ctx context.Context, province string) ([]repository.RegionCoefficient, error)
	ListProvinces(ctx context.Context) ([]string, error)
	ListCities(ctx context.Context, province string) ([]string, error)
	ListOriginalPrices(ctx context.Context, limit, offset int) ([]repository.OriginalPrice, int, error)
	ListCoefficientConfigs(ctx context.Context) ([]repository.CoefficientConfig, error)
	ListAlgorithmParameters(ctx context.Context) (repository.AlgorithmParameters, error)
	GetEarliestFactoryYearByCascade(ctx context.Context, brand, vehicleType, series string, tonnage float64) (int, error)
}
