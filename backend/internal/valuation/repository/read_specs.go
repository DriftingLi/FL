// 简单单表读的 ReadSpec 声明（ADR-0013 候选 1）：每实体一份描述符，
// 字段复用写面 dictcrud 的 Field（Name/Column/Type 单点）。
package repository

import "forklift-training/internal/valuation/dictcrud"

var (
	// brands
	readSpecBrandsList = ReadSpec{
		Table: "brands",
		Columns: []dictcrud.Field{
			{Name: "name", Column: "name", Type: dictcrud.FieldString},
			{Name: "k_brand", Column: "k_brand", Type: dictcrud.FieldFloat},
			{Name: "is_active", Column: "is_active", Type: dictcrud.FieldBool},
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
		Columns: []dictcrud.Field{
			{Name: "rating", Column: "rating", Type: dictcrud.FieldString},
			{Name: "label", Column: "label", Type: dictcrud.FieldString},
			{Name: "base_coefficient", Column: "base_coefficient", Type: dictcrud.FieldFloat},
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
		Columns: []dictcrud.Field{
			{Name: "name", Column: "name", Type: dictcrud.FieldString},
			{Name: "power_type", Column: "power_type", Type: dictcrud.FieldString},
			{Name: "earliest_factory_year", Column: "earliest_factory_year", Type: dictcrud.FieldInt},
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
		Columns: []dictcrud.Field{
			{Name: "brand", Column: "brand", Type: dictcrud.FieldString},
			{Name: "name", Column: "name", Type: dictcrud.FieldString},
			{Name: "earliest_factory_year", Column: "earliest_factory_year", Type: dictcrud.FieldInt},
		},
		OrderBy: "id ASC",
	}

	// specs 家族
	readSpecTonnagesList = ReadSpec{
		Table:   "tonnages",
		Columns: []dictcrud.Field{{Name: "value", Column: "value", Type: dictcrud.FieldFloat}},
		OrderBy: "value ASC",
	}
	readSpecMastTypesList = ReadSpec{
		Table:   "mast_types",
		Columns: []dictcrud.Field{{Name: "name", Column: "name", Type: dictcrud.FieldString}},
		OrderBy: "id ASC",
	}
	readSpecMastHeightsList = ReadSpec{
		Table:   "mast_heights",
		Columns: []dictcrud.Field{{Name: "value_mm", Column: "value_mm", Type: dictcrud.FieldInt}},
		OrderBy: "value_mm ASC",
	}
	readSpecBatteryTypesList = ReadSpec{
		Table:   "battery_types",
		Columns: []dictcrud.Field{{Name: "name", Column: "name", Type: dictcrud.FieldString}},
		OrderBy: "id ASC",
	}
	readSpecTransmissionTypesList = ReadSpec{
		Table:   "transmission_types",
		Columns: []dictcrud.Field{{Name: "name", Column: "name", Type: dictcrud.FieldString}},
		OrderBy: "id ASC",
	}
	readSpecEngineTypesList = ReadSpec{
		Table:   "engine_types",
		Columns: []dictcrud.Field{{Name: "name", Column: "name", Type: dictcrud.FieldString}},
		OrderBy: "id ASC",
	}

	// region_coefficients
	readSpecRegionList = ReadSpec{
		Table: "region_coefficients",
		Columns: []dictcrud.Field{
			{Name: "province", Column: "province", Type: dictcrud.FieldString},
			{Name: "city", Column: "city", Type: dictcrud.FieldString},
			{Name: "coefficient", Column: "coefficient", Type: dictcrud.FieldFloat},
		},
		OrderBy: "id ASC",
	}
	readSpecRegionProvinces = ReadSpec{
		Table:    "region_coefficients",
		Columns:  []dictcrud.Field{{Name: "province", Column: "province", Type: dictcrud.FieldString}},
		Distinct: true,
		OrderBy:  "province ASC",
		NoID:     true,
	}
	readSpecRegionCities = ReadSpec{
		Table:   "region_coefficients",
		Columns: []dictcrud.Field{{Name: "city", Column: "city", Type: dictcrud.FieldString}},
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
		Columns: []dictcrud.Field{
			{Name: "key", Column: "key", Type: dictcrud.FieldString},
			{Name: "value", Column: "value", Type: dictcrud.FieldFloat},
			{Name: "description", Column: "description", Type: dictcrud.FieldString},
			{Name: "updated_at", Column: "updated_at", Type: dictcrud.FieldString},
		},
		OrderBy: "key ASC",
	}
	readSpecCoefGet = ReadSpec{
		Table:   "coefficient_configs",
		Columns: readSpecCoefList.Columns,
		Where:   "key = $1",
	}
)
