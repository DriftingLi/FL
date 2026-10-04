// 简单单表读的 ReadSpec 声明（ADR-0013 候选 1）：每实体一份描述符，
// 字段复用写面 dictcrud 的 Field（Name/Column/Type 单点）。
package repository

import "forklift-training/internal/valuation"

var (
	// brands
	readSpecBrandsList = ReadSpec{
		Table: "brands",
		Columns: []valuation.DictField{
			{Name: "name", Column: "name", Type: valuation.DictFieldString},
			{Name: "k_brand", Column: "k_brand", Type: valuation.DictFieldFloat},
			{Name: "is_active", Column: "is_active", Type: valuation.DictFieldBool},
		},
		OrderBy: "k_brand DESC, name ASC",
	}
	readSpecBrandsGet = ReadSpec{
		Table:   "brands",
		Columns: readSpecBrandsList.Columns,
		Where:   "name = $1",
	}

	// condition_ratings
	readSpecConditionList = ReadSpec{
		Table: "condition_ratings",
		Columns: []valuation.DictField{
			{Name: "rating", Column: "rating", Type: valuation.DictFieldString},
			{Name: "label", Column: "label", Type: valuation.DictFieldString},
			{Name: "base_coefficient", Column: "base_coefficient", Type: valuation.DictFieldFloat},
		},
		OrderBy: "base_coefficient DESC",
	}
	readSpecConditionGet = ReadSpec{
		Table:   "condition_ratings",
		Columns: readSpecConditionList.Columns,
		Where:   "rating = $1",
	}

	// vehicle_types
	readSpecVtList = ReadSpec{
		Table: "vehicle_types",
		Columns: []valuation.DictField{
			{Name: "name", Column: "name", Type: valuation.DictFieldString},
			{Name: "power_type", Column: "power_type", Type: valuation.DictFieldString},
			{Name: "earliest_factory_year", Column: "earliest_factory_year", Type: valuation.DictFieldInt},
		},
		OrderBy: "id ASC",
	}
	readSpecVtGet = ReadSpec{
		Table:   "vehicle_types",
		Columns: readSpecVtList.Columns,
		Where:   "name = $1",
	}

	// series
	readSpecSeriesList = ReadSpec{
		Table: "series",
		Columns: []valuation.DictField{
			{Name: "brand", Column: "brand", Type: valuation.DictFieldString},
			{Name: "name", Column: "name", Type: valuation.DictFieldString},
			{Name: "earliest_factory_year", Column: "earliest_factory_year", Type: valuation.DictFieldInt},
		},
		OrderBy: "id ASC",
	}

	// specs 家族
	readSpecTonnagesList = ReadSpec{
		Table:   "tonnages",
		Columns: []valuation.DictField{{Name: "value", Column: "value", Type: valuation.DictFieldFloat}},
		OrderBy: "value ASC",
	}
	readSpecMastTypesList = ReadSpec{
		Table:   "mast_types",
		Columns: []valuation.DictField{{Name: "name", Column: "name", Type: valuation.DictFieldString}},
		OrderBy: "id ASC",
	}
	readSpecMastHeightsList = ReadSpec{
		Table:   "mast_heights",
		Columns: []valuation.DictField{{Name: "value_mm", Column: "value_mm", Type: valuation.DictFieldInt}},
		OrderBy: "value_mm ASC",
	}
	readSpecBatteryTypesList = ReadSpec{
		Table:   "battery_types",
		Columns: []valuation.DictField{{Name: "name", Column: "name", Type: valuation.DictFieldString}},
		OrderBy: "id ASC",
	}
	readSpecTransmissionTypesList = ReadSpec{
		Table:   "transmission_types",
		Columns: []valuation.DictField{{Name: "name", Column: "name", Type: valuation.DictFieldString}},
		OrderBy: "id ASC",
	}
	readSpecEngineTypesList = ReadSpec{
		Table:   "engine_types",
		Columns: []valuation.DictField{{Name: "name", Column: "name", Type: valuation.DictFieldString}},
		OrderBy: "id ASC",
	}

	// region_coefficients
	readSpecRegionList = ReadSpec{
		Table: "region_coefficients",
		Columns: []valuation.DictField{
			{Name: "province", Column: "province", Type: valuation.DictFieldString},
			{Name: "city", Column: "city", Type: valuation.DictFieldString},
			{Name: "coefficient", Column: "coefficient", Type: valuation.DictFieldFloat},
		},
		OrderBy: "id ASC",
	}
	readSpecRegionProvinces = ReadSpec{
		Table:    "region_coefficients",
		Columns:  []valuation.DictField{{Name: "province", Column: "province", Type: valuation.DictFieldString}},
		Distinct: true,
		OrderBy:  "province ASC",
		NoID:     true,
	}
	readSpecRegionCities = ReadSpec{
		Table:   "region_coefficients",
		Columns: []valuation.DictField{{Name: "city", Column: "city", Type: valuation.DictFieldString}},
		Where:   "province = $1",
		OrderBy: "city ASC",
		NoID:    true,
	}
	readSpecRegionGet = ReadSpec{
		Table:   "region_coefficients",
		Columns: readSpecRegionList.Columns,
		Where:   "province = $1 AND city = $2",
	}

	// coefficient_configs（特殊解码：description 可空 + updated_at 格式化；SQL 仍由 spec 单点生成）
	readSpecCoefList = ReadSpec{
		Table: "coefficient_configs",
		Columns: []valuation.DictField{
			{Name: "key", Column: "key", Type: valuation.DictFieldString},
			{Name: "value", Column: "value", Type: valuation.DictFieldFloat},
			{Name: "description", Column: "description", Type: valuation.DictFieldString},
			{Name: "updated_at", Column: "updated_at", Type: valuation.DictFieldString},
		},
		OrderBy: "key ASC",
	}
	readSpecCoefGet = ReadSpec{
		Table:   "coefficient_configs",
		Columns: readSpecCoefList.Columns,
		Where:   "key = $1",
	}
)
