// Package handler 实现 HTTP 处理器
// 本文件：字典配置存储接口（ConfigHandler 消费的完整字典 CRUD 契约）。
package handler

import (
	"context"

	"forklift-training/internal/valuation/repository"
	"forklift-training/internal/valuation/service"
)

// DictionaryConfigStore 字典配置存储接口（ConfigHandler 消费；生产为 pgx 仓储，测试为内存替身）。
// 嵌入 service.DictionaryReader（评估流程读面），加上字典 CRUD 写面——面宽是 ConfigHandler
// 的真实消费面：30+ 只读字典查询 + 26 个 CRUD 写操作。
type DictionaryConfigStore interface {
	service.DictionaryReader

	ListBrands(ctx context.Context) ([]repository.Brand, error)
	CreateBrand(ctx context.Context, name string, kBrand float64, isActive bool) (repository.Brand, error)
	UpdateBrand(ctx context.Context, id int64, kBrand float64, isActive bool) error
	DeleteBrand(ctx context.Context, id int64) error

	ListVehicleTypes(ctx context.Context) ([]repository.VehicleType, error)
	ListVehicleTypesByBrand(ctx context.Context, brand string) ([]repository.VehicleType, error)
	CreateVehicleType(ctx context.Context, name, powerType string, earliestFactoryYear int) (repository.VehicleType, error)
	UpdateVehicleType(ctx context.Context, id int, name, powerType string, earliestFactoryYear int) error
	DeleteVehicleType(ctx context.Context, id int) error

	ListSeries(ctx context.Context, brand string) ([]repository.Series, error)
	ListSeriesByCascade(ctx context.Context, brand, vehicleType string) ([]repository.Series, error)
	ListSeriesConfigOptions(ctx context.Context, brand, series string) (repository.SeriesConfigOptions, error)
	CreateSeries(ctx context.Context, brand, name string, earliestFactoryYear int) (repository.Series, error)
	UpdateSeries(ctx context.Context, id int, brand, name string, earliestFactoryYear int) error
	DeleteSeries(ctx context.Context, id int) error

	ListTonnages(ctx context.Context) ([]repository.Tonnage, error)
	ListTonnagesByCascade(ctx context.Context, brand, vehicleType, series string) ([]repository.Tonnage, error)
	CreateTonnage(ctx context.Context, value float64) (repository.Tonnage, error)
	DeleteTonnage(ctx context.Context, id int) error

	ListConfigOptionsByCascade(ctx context.Context, brand, vehicleType, series, tonnage string) ([]repository.ConfigOption, error)

	ListMastTypes(ctx context.Context) ([]repository.MastType, error)
	ListMastTypesByCascade(ctx context.Context, brand, vehicleType, series, tonnage, configType string) ([]repository.MastType, error)
	CreateMastType(ctx context.Context, name string) (repository.MastType, error)
	DeleteMastType(ctx context.Context, id int) error

	ListMastHeights(ctx context.Context) ([]repository.MastHeight, error)
	ListMastHeightsByCascade(ctx context.Context, brand, vehicleType, series, tonnage, configType, mastType string) ([]repository.MastHeight, error)
	CreateMastHeight(ctx context.Context, valueMM int) (repository.MastHeight, error)
	DeleteMastHeight(ctx context.Context, id int) error

	ListBatteryTypes(ctx context.Context) ([]repository.BatteryTypeDict, error)
	ListBatteryTypesByCascade(ctx context.Context, brand, vehicleType, series, tonnage string) ([]repository.BatteryTypeDict, error)
	CreateBatteryType(ctx context.Context, name string) (repository.BatteryTypeDict, error)
	DeleteBatteryType(ctx context.Context, id int) error

	ListTransmissionTypes(ctx context.Context) ([]repository.TransmissionType, error)
	CreateTransmissionType(ctx context.Context, name string) (repository.TransmissionType, error)
	DeleteTransmissionType(ctx context.Context, id int) error

	ListEngineTypes(ctx context.Context) ([]repository.EngineType, error)
	CreateEngineType(ctx context.Context, name string) (repository.EngineType, error)
	DeleteEngineType(ctx context.Context, id int) error

	ListConditionRatings(ctx context.Context) ([]repository.ConditionRating, error)
	CreateConditionRating(ctx context.Context, rating, label string, base float64) (repository.ConditionRating, error)
	UpdateConditionRating(ctx context.Context, id int, label string, base float64) error
	DeleteConditionRating(ctx context.Context, id int) error

	ListRegionCoefficients(ctx context.Context, province string) ([]repository.RegionCoefficient, error)
	ListProvinces(ctx context.Context) ([]string, error)
	ListCities(ctx context.Context, province string) ([]string, error)
	CreateRegionCoefficient(ctx context.Context, province, city string, coefficient float64) (repository.RegionCoefficient, error)
	UpdateRegionCoefficient(ctx context.Context, id int, coefficient float64) error
	DeleteRegionCoefficient(ctx context.Context, id int) error

	ListOriginalPrices(ctx context.Context, limit, offset int) ([]repository.OriginalPrice, int, error)
	CreateOriginalPrice(ctx context.Context, o *repository.OriginalPrice) (int64, error)
	UpdateOriginalPrice(ctx context.Context, o *repository.OriginalPrice) error
	DeleteOriginalPrice(ctx context.Context, id int64) error

	ListCoefficientConfigs(ctx context.Context) ([]repository.CoefficientConfig, error)
	UpdateCoefficientByKey(ctx context.Context, key string, value float64) (repository.CoefficientConfig, error)
	ListAlgorithmParameters(ctx context.Context) (repository.AlgorithmParameters, error)

	GetEarliestFactoryYearByCascade(ctx context.Context, brand, vehicleType, series string, tonnage float64) (int, error)

	// 缓存失效 pattern 访问方法（契约同源，见 repository/dict_cache_keys.go）。
	// handler 只组合这些 pattern，不再书写字面量。
	BrandInvalidationPatterns() []string
	VehicleTypeInvalidationPatterns() []string
	SeriesInvalidationPatterns() []string
	TonnageInvalidationPatterns() []string
	MastTypeInvalidationPatterns() []string
	MastHeightInvalidationPatterns() []string
	BatteryTypeInvalidationPatterns() []string
	TransmissionTypeInvalidationPatterns() []string
	EngineTypeInvalidationPatterns() []string
	ConditionRatingInvalidationPatterns() []string
	RegionCoefficientInvalidationPatterns() []string
	CoefficientInvalidationPatterns() []string
	OriginalPriceInvalidationPatterns() []string
}
