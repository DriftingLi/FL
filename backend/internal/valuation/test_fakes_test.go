// 公式测试共享的内存替身：系数表读取与系数键读取
// 值对齐 migrations/000001_init_baseline.up.sql 种子，公式测试不再依赖真实 Postgres。
package valuation

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// memConfigReader 内存版系数键读取。
type memConfigReader struct {
	values map[string]float64
}

func (m *memConfigReader) Get(_ context.Context, key string) (float64, error) {
	if v, ok := m.values[key]; ok {
		return v, nil
	}
	return 0, errors.New("coefficient not found: " + key)
}

func (m *memConfigReader) ReadFloat(ctx context.Context, key string, fallback float64) float64 {
	return coefReadFloat(ctx, m, key, fallback)
}

// newDefaultConfigReader 与迁移种子一致的默认系数。
func newDefaultConfigReader() *memConfigReader {
	return &memConfigReader{values: map[string]float64{
		KeyLambdaElectric:             0.12,
		KeyLambdaCombustion:           0.10,
		KeyAnnualUsageHours:           1750,
		KeyKHoursRatioLow:             0.7,
		KeyKHoursRatioMid:             1.0,
		KeyKHoursRatioHigh:            1.3,
		KeyKHoursRatioMax:             1.6,
		KeyKcPaintBonus:               0.02,
		KeyKcMaintenanceBonus:         0.02,
		KeyKcNoLicensePenaltyPct:      0.10,
		KeyKcNoRegistrationPenaltyPct: 0.10,
	}}
}

// memDictReader 内存版系数表读取（condition_ratings / region / brand）。
type memDictReader struct {
	conditions map[string]ConditionRating
	regions    map[string]RegionCoefficient
	brands     map[string]Brand
}

// newDefaultMemDict 与迁移种子一致的默认系数表。
func newDefaultMemDict() *memDictReader {
	return &memDictReader{
		conditions: map[string]ConditionRating{
			"A": {ID: 1, Rating: "A", Label: "优秀", BaseCoefficient: 1.00},
			"B": {ID: 2, Rating: "B", Label: "良好", BaseCoefficient: 0.90},
			"C": {ID: 3, Rating: "C", Label: "一般", BaseCoefficient: 0.78},
			"D": {ID: 4, Rating: "D", Label: "较差", BaseCoefficient: 0.65},
			"E": {ID: 5, Rating: "E", Label: "差", BaseCoefficient: 0.50},
		},
		regions: map[string]RegionCoefficient{},
		brands:  map[string]Brand{},
	}
}

func (m *memDictReader) GetConditionRating(_ context.Context, rating string) (ConditionRating, error) {
	if c, ok := m.conditions[rating]; ok {
		return c, nil
	}
	return ConditionRating{}, pgx.ErrNoRows
}

func (m *memDictReader) GetRegionCoefficient(_ context.Context, province, city string) (RegionCoefficient, error) {
	if r, ok := m.regions[province+"|"+city]; ok {
		return r, nil
	}
	return RegionCoefficient{}, pgx.ErrNoRows
}

func (m *memDictReader) GetBrandByName(_ context.Context, name string) (Brand, error) {
	if b, ok := m.brands[name]; ok {
		return b, nil
	}
	return Brand{}, pgx.ErrNoRows
}

func (m *memDictReader) GetVehicleTypeByName(context.Context, string) (VehicleType, error) {
	return VehicleType{}, pgx.ErrNoRows
}

func (m *memDictReader) FindOriginalPriceMatch(context.Context, string, string, string, float64, string, string, int) (OriginalPrice, error) {
	return OriginalPrice{}, pgx.ErrNoRows
}

func (m *memDictReader) FindOriginalPriceFuzzy(context.Context, string, string, string, float64) (OriginalPrice, error) {
	return OriginalPrice{}, pgx.ErrNoRows
}

func (m *memDictReader) GetCoefficientByKey(context.Context, string) (CoefficientConfig, error) {
	return CoefficientConfig{}, pgx.ErrNoRows
}

func (m *memDictReader) ListCoefficientConfigs(context.Context) ([]CoefficientConfig, error) {
	return []CoefficientConfig{}, nil
}
