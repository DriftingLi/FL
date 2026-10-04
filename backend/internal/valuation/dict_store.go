// HTTP 出口（原 internal/valuation/handler 子包，#1514 波 9 并回域包）：实现 HTTP 处理器
// 本文件：字典配置存储接口（ConfigHandler 消费的字典契约，ADR-0008 收窄后的最终面）。
package valuation

import (
	"context"
)

// DictWriter 描述符驱动字典写面（ADR-0008）：机械 CRUD 塌缩后的窄面。
// 生产为 DictStore（DictionaryRepository 嵌入），测试为内存替身。
// UpdateByKey 供按 key 更新的实体使用（coefficient_configs，PUT /:key，返回完整行）。
type DictWriter interface {
	Create(ctx context.Context, d DictDescriptor, fields map[string]any) (int64, error)
	Update(ctx context.Context, d DictDescriptor, id int64, fields map[string]any) error
	UpdateByKey(ctx context.Context, d DictDescriptor, key string, fields map[string]any) (map[string]any, error)
	Delete(ctx context.Context, d DictDescriptor, id int64) error
}

// DictionaryConfigStore 字典配置存储接口（ConfigHandler 消费；生产为 pgx 仓储，测试为内存替身）。
// 读面 = DictionaryReader + 学生端字典查询 typed 方法（30+，形状异构不强求通用化）；
// 写面 = DictWriter（描述符驱动，全部实体的 CRUD 写操作塌缩到 4 个方法）。
type DictionaryConfigStore interface {
	DictionaryReader
	DictWriter

	ListBrands(ctx context.Context) ([]Brand, error)
	ListVehicleTypes(ctx context.Context) ([]VehicleType, error)
	ListVehicleTypesByBrand(ctx context.Context, brand string) ([]VehicleType, error)
	ListSeries(ctx context.Context, brand string) ([]Series, error)
	ListSeriesByCascade(ctx context.Context, brand, vehicleType string) ([]Series, error)
	ListSeriesConfigOptions(ctx context.Context, brand, series string) (SeriesConfigOptions, error)
	ListTonnages(ctx context.Context) ([]Tonnage, error)
	ListTonnagesByCascade(ctx context.Context, brand, vehicleType, series string) ([]Tonnage, error)
	ListConfigOptionsByCascade(ctx context.Context, brand, vehicleType, series, tonnage string) ([]ConfigOption, error)
	ListMastTypes(ctx context.Context) ([]MastType, error)
	ListMastTypesByCascade(ctx context.Context, brand, vehicleType, series, tonnage, configType string) ([]MastType, error)
	ListMastHeights(ctx context.Context) ([]MastHeight, error)
	ListMastHeightsByCascade(ctx context.Context, brand, vehicleType, series, tonnage, configType, mastType string) ([]MastHeight, error)
	ListBatteryTypes(ctx context.Context) ([]BatteryTypeDict, error)
	ListBatteryTypesByCascade(ctx context.Context, brand, vehicleType, series, tonnage string) ([]BatteryTypeDict, error)
	ListTransmissionTypes(ctx context.Context) ([]TransmissionType, error)
	ListEngineTypes(ctx context.Context) ([]EngineType, error)
	ListConditionRatings(ctx context.Context) ([]ConditionRating, error)
	ListRegionCoefficients(ctx context.Context, province string) ([]RegionCoefficient, error)
	ListProvinces(ctx context.Context) ([]string, error)
	ListCities(ctx context.Context, province string) ([]string, error)
	ListOriginalPrices(ctx context.Context, limit, offset int) ([]OriginalPrice, int, error)
	ListCoefficientConfigs(ctx context.Context) ([]CoefficientConfig, error)
	ListAlgorithmParameters(ctx context.Context) (AlgorithmParameters, error)
	GetEarliestFactoryYearByCascade(ctx context.Context, brand, vehicleType, series string, tonnage float64) (int, error)
}
