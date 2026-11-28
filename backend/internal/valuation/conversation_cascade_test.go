package valuation

import (
	"context"
	"fmt"
)

type cascadeVehicleTypeGapDictionary struct {
	*memDictStore
}

func (m *cascadeVehicleTypeGapDictionary) ListVehicleTypesByBrand(ctx context.Context, brand string) ([]VehicleType, error) {
	if brand == "丰田" {
		return []VehicleType{{ID: 1, Name: "内燃叉车", PowerType: "combustion"}}, nil
	}
	return m.memDictStore.ListVehicleTypesByBrand(ctx, brand)
}

func (m *memDictStore) ListVehicleTypesByBrand(_ context.Context, brand string) ([]VehicleType, error) {
	seen := map[string]bool{}
	rows := make([]VehicleType, 0)
	for _, price := range m.originalPrices {
		if price.Brand != brand || seen[price.VehicleType] {
			continue
		}
		seen[price.VehicleType] = true
		rows = append(rows, VehicleType{ID: len(rows) + 1, Name: price.VehicleType, PowerType: "electric"})
	}
	return rows, nil
}

func (m *memDictStore) ListSeriesByCascade(_ context.Context, brand, vehicleType string) ([]Series, error) {
	seen := map[string]bool{}
	rows := make([]Series, 0)
	for _, price := range m.originalPrices {
		if price.Brand != brand || price.VehicleType != vehicleType || seen[price.Series] {
			continue
		}
		seen[price.Series] = true
		rows = append(rows, Series{ID: len(rows) + 1, Brand: brand, Name: price.Series})
	}
	return rows, nil
}

func (m *memDictStore) ListTonnagesByCascade(_ context.Context, brand, vehicleType, series string) ([]Tonnage, error) {
	seen := map[float64]bool{}
	rows := make([]Tonnage, 0)
	for _, price := range m.originalPrices {
		if price.Brand != brand || price.VehicleType != vehicleType || price.Series != series || seen[price.Tonnage] {
			continue
		}
		seen[price.Tonnage] = true
		rows = append(rows, Tonnage{ID: len(rows) + 1, Value: price.Tonnage})
	}
	return rows, nil
}

func (m *memDictStore) ListConfigOptionsByCascade(_ context.Context, brand, vehicleType, series, tonnage string) ([]ConfigOption, error) {
	seen := map[string]bool{}
	rows := make([]ConfigOption, 0)
	for _, price := range m.originalPrices {
		if price.Brand != brand || price.VehicleType != vehicleType || price.Series != series ||
			fmt.Sprint(price.Tonnage) != tonnage || seen[price.ConfigType] {
			continue
		}
		seen[price.ConfigType] = true
		rows = append(rows, ConfigOption{ID: len(rows) + 1, Name: price.ConfigType})
	}
	return rows, nil
}

func (m *memDictStore) ListMastTypesByCascade(_ context.Context, brand, vehicleType, series, tonnage, configType string) ([]MastType, error) {
	seen := map[string]bool{}
	rows := make([]MastType, 0)
	for _, price := range m.originalPrices {
		if price.Brand != brand || price.VehicleType != vehicleType || price.Series != series ||
			fmt.Sprint(price.Tonnage) != tonnage || price.ConfigType != configType || seen[price.MastType] {
			continue
		}
		seen[price.MastType] = true
		rows = append(rows, MastType{ID: len(rows) + 1, Name: price.MastType})
	}
	return rows, nil
}

func (m *memDictStore) ListMastHeightsByCascade(_ context.Context, brand, vehicleType, series, tonnage, configType, mastType string) ([]MastHeight, error) {
	seen := map[int]bool{}
	rows := make([]MastHeight, 0)
	for _, price := range m.originalPrices {
		if price.Brand != brand || price.VehicleType != vehicleType || price.Series != series ||
			fmt.Sprint(price.Tonnage) != tonnage || price.ConfigType != configType || price.MastType != mastType || seen[price.MastHeightMM] {
			continue
		}
		seen[price.MastHeightMM] = true
		rows = append(rows, MastHeight{ID: len(rows) + 1, ValueMM: price.MastHeightMM})
	}
	return rows, nil
}
