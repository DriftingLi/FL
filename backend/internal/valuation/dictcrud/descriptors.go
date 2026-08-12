// 已迁移实体的描述符声明（#94 简单实体 + #95 复杂实体）。
// 每个描述符的字段/操作集/upsert/默认值/校验/失效声明与迁移前 handler+repository 行为逐项对应，
// HTTP 契约由 handler 侧契约测试锁定（region_crud_contract_test.go 的 prior art）。
package dictcrud

import (
	"time"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// #94 简单实体（规格/品牌家族，无描述符核心扩展）
// =====================================================

// BrandDescriptor 品牌：三字段 + (name) 唯一 DO UPDATE + update 为 k_brand/is_active 子集。
var BrandDescriptor = Descriptor{
	Name:            "brands",
	EntityLabel:     "品牌",
	NotFoundMessage: "品牌不存在",
	Table:           "brands",
	Path:            "brands",
	Fields: []Field{
		{Name: "name", Column: "name", Type: FieldString},
		{Name: "k_brand", Column: "k_brand", Type: FieldFloat, BindName: "KBrand"},
		{Name: "is_active", Column: "is_active", Type: FieldBool},
	},
	Create: OpSpec{
		Fields:   []string{"name", "k_brand", "is_active"},
		Required: []string{"name"},
	},
	Update: OpSpec{
		Fields:       []string{"k_brand", "is_active"},
		BindRequired: []string{"k_brand"},
	},
	Delete:           true,
	UniqueColumns:    []string{"name"},
	Upsert:           UpsertDoUpdate,
	InvalidateResult: true,
	RequiredMessage:  "name 必填",
}

// VehicleTypeDescriptor 车型：name/power_type 必填 + earliest_factory_year 默认 1980（双侧）。
var VehicleTypeDescriptor = Descriptor{
	Name:            "vehicle_types",
	EntityLabel:     "车型",
	NotFoundMessage: "车型不存在",
	Table:           "vehicle_types",
	Path:            "vehicle-types",
	Fields: []Field{
		{Name: "name", Column: "name", Type: FieldString},
		{Name: "power_type", Column: "power_type", Type: FieldString, BindName: "PowerType"},
		{Name: "earliest_factory_year", Column: "earliest_factory_year", Type: FieldInt, Default: 1980},
	},
	Create: OpSpec{
		Fields:   []string{"name", "power_type", "earliest_factory_year"},
		Required: []string{"name", "power_type"},
	},
	Update: OpSpec{
		Fields:       []string{"name", "power_type", "earliest_factory_year"},
		BindRequired: []string{"name", "power_type"},
	},
	Delete:           true,
	UniqueColumns:    []string{"name"},
	Upsert:           UpsertDoUpdate,
	InvalidateResult: true,
	RequiredMessage:  "name 与 power_type 必填",
}

// SeriesDescriptor 系列：(brand, name) 复合唯一 DO NOTHING + 默认 2000；
// 唯一不追加评估结果缓存 pattern 的实体（InvalidateResult=false，现状）。
var SeriesDescriptor = Descriptor{
	Name:            "series",
	EntityLabel:     "系列",
	NotFoundMessage: "系列不存在",
	Table:           "series",
	Path:            "series",
	Fields: []Field{
		{Name: "brand", Column: "brand", Type: FieldString},
		{Name: "name", Column: "name", Type: FieldString},
		{Name: "earliest_factory_year", Column: "earliest_factory_year", Type: FieldInt, Default: 2000},
	},
	Create: OpSpec{
		Fields:   []string{"brand", "name", "earliest_factory_year"},
		Required: []string{"brand", "name"},
	},
	Update: OpSpec{
		Fields:       []string{"brand", "name", "earliest_factory_year"},
		BindRequired: []string{"brand", "name"},
	},
	Delete:           true,
	UniqueColumns:    []string{"brand", "name"},
	Upsert:           UpsertDoNothing,
	InvalidateResult: false,
	RequiredMessage:  "brand 与 name 必填",
}

// TonnageDescriptor 吨位：单 float 唯一列 DO NOTHING + C/D。
var TonnageDescriptor = Descriptor{
	Name:            "tonnages",
	EntityLabel:     "吨位",
	NotFoundMessage: "吨位不存在",
	Table:           "tonnages",
	Path:            "tonnages",
	Fields: []Field{
		{Name: "value", Column: "value", Type: FieldFloat},
	},
	Create: OpSpec{
		Fields:       []string{"value"},
		BindRequired: []string{"value"},
	},
	Delete:           true,
	UniqueColumns:    []string{"value"},
	Upsert:           UpsertDoNothing,
	InvalidateResult: true,
}

// MastTypeDescriptor 门架类型：单 name 唯一列 DO NOTHING + C/D。
var MastTypeDescriptor = Descriptor{
	Name:            "mast_types",
	EntityLabel:     "门架类型",
	NotFoundMessage: "门架类型不存在",
	Table:           "mast_types",
	Path:            "mast-types",
	Fields: []Field{
		{Name: "name", Column: "name", Type: FieldString},
	},
	Create: OpSpec{
		Fields:       []string{"name"},
		BindRequired: []string{"name"},
	},
	Delete:           true,
	UniqueColumns:    []string{"name"},
	Upsert:           UpsertDoNothing,
	InvalidateResult: true,
}

// MastHeightDescriptor 门架高度：单 value_mm 唯一列 DO NOTHING + C/D。
var MastHeightDescriptor = Descriptor{
	Name:            "mast_heights",
	EntityLabel:     "门架高度",
	NotFoundMessage: "门架高度不存在",
	Table:           "mast_heights",
	Path:            "mast-heights",
	Fields: []Field{
		{Name: "value_mm", Column: "value_mm", Type: FieldInt, BindName: "ValueMM"},
	},
	Create: OpSpec{
		Fields:       []string{"value_mm"},
		BindRequired: []string{"value_mm"},
	},
	Delete:           true,
	UniqueColumns:    []string{"value_mm"},
	Upsert:           UpsertDoNothing,
	InvalidateResult: true,
}

// BatteryTypeDescriptor 电池类型：单 name 唯一列 DO NOTHING + C/D。
var BatteryTypeDescriptor = Descriptor{
	Name:            "battery_types",
	EntityLabel:     "电池类型",
	NotFoundMessage: "电池类型不存在",
	Table:           "battery_types",
	Path:            "battery-types",
	Fields: []Field{
		{Name: "name", Column: "name", Type: FieldString},
	},
	Create: OpSpec{
		Fields:       []string{"name"},
		BindRequired: []string{"name"},
	},
	Delete:           true,
	UniqueColumns:    []string{"name"},
	Upsert:           UpsertDoNothing,
	InvalidateResult: true,
}

// TransmissionTypeDescriptor 传动系统类型：单 name 唯一列 DO NOTHING + C/D。
var TransmissionTypeDescriptor = Descriptor{
	Name:            "transmission_types",
	EntityLabel:     "传动系统类型",
	NotFoundMessage: "传动系统类型不存在",
	Table:           "transmission_types",
	Path:            "transmission-types",
	Fields: []Field{
		{Name: "name", Column: "name", Type: FieldString},
	},
	Create: OpSpec{
		Fields:       []string{"name"},
		BindRequired: []string{"name"},
	},
	Delete:           true,
	UniqueColumns:    []string{"name"},
	Upsert:           UpsertDoNothing,
	InvalidateResult: true,
}

// EngineTypeDescriptor 发动机类型：单 name 唯一列 DO NOTHING + C/D。
var EngineTypeDescriptor = Descriptor{
	Name:            "engine_types",
	EntityLabel:     "发动机类型",
	NotFoundMessage: "发动机类型不存在",
	Table:           "engine_types",
	Path:            "engine-types",
	Fields: []Field{
		{Name: "name", Column: "name", Type: FieldString},
	},
	Create: OpSpec{
		Fields:       []string{"name"},
		BindRequired: []string{"name"},
	},
	Delete:           true,
	UniqueColumns:    []string{"name"},
	Upsert:           UpsertDoNothing,
	InvalidateResult: true,
}

// =====================================================
// #95 复杂实体
// =====================================================

// ConditionRatingDescriptor 车况评级：rating 唯一 DO UPDATE + update 无 rating（非对称子集）。
var ConditionRatingDescriptor = Descriptor{
	Name:            "condition_ratings",
	EntityLabel:     "车况评级",
	NotFoundMessage: "车况评级不存在",
	Table:           "condition_ratings",
	Path:            "condition-ratings",
	Fields: []Field{
		{Name: "rating", Column: "rating", Type: FieldString},
		{Name: "label", Column: "label", Type: FieldString},
		{Name: "base_coefficient", Column: "base_coefficient", Type: FieldFloat, BindName: "BaseCoefficient"},
	},
	Create: OpSpec{
		Fields:   []string{"rating", "label", "base_coefficient"},
		Required: []string{"rating", "label"},
	},
	Update: OpSpec{
		Fields:       []string{"label", "base_coefficient"},
		BindRequired: []string{"label", "base_coefficient"},
	},
	Delete:           true,
	UniqueColumns:    []string{"rating"},
	Upsert:           UpsertDoUpdate,
	InvalidateResult: true,
	RequiredMessage:  "rating 与 label 必填",
}

// RegionCoefficientDescriptor 区域系数描述符（迁移 tracer）。
// 形状：三字段平表 + (province, city) 复合唯一 + upsert 仅更新 coefficient +
// update 为单字段子集；create 应用层必填校验（province/city），update 为 bind 必填。
var RegionCoefficientDescriptor = Descriptor{
	Name:            "region_coefficients",
	EntityLabel:     "区域系数",
	NotFoundMessage: "区域系数不存在",
	Table:           "region_coefficients",
	Path:            "region-coefficients",
	Fields: []Field{
		{Name: "province", Column: "province", Type: FieldString},
		{Name: "city", Column: "city", Type: FieldString},
		{Name: "coefficient", Column: "coefficient", Type: FieldFloat, BindName: "Coefficient"},
	},
	Create: OpSpec{
		Fields:   []string{"province", "city", "coefficient"},
		Required: []string{"province", "city"},
	},
	Update: OpSpec{
		Fields:       []string{"coefficient"},
		BindRequired: []string{"coefficient"},
	},
	Delete:           true,
	UniqueColumns:    []string{"province", "city"},
	Upsert:           UpsertDoUpdate,
	InvalidateResult: true,
	RequiredMessage:  "province 与 city 必填",
}

// CoefficientConfigDescriptor 系数配置：只读 + 按 key 更新（U-only，PUT /:key）。
// 无 create/delete；update 返回完整行（RETURNING，description 可空 + updated_at 格式化）。
var CoefficientConfigDescriptor = Descriptor{
	Name:              "coefficient_configs",
	EntityLabel:       "系数配置",
	NotFoundMessage:   "系数 key 不存在",
	NotFoundWithValue: true,
	Table:             "coefficient_configs",
	Path:              "coefficient-configs",
	Fields: []Field{
		{Name: "key", Column: "key", Type: FieldString},
		{Name: "value", Column: "value", Type: FieldFloat, BindName: "Value"},
		{Name: "description", Column: "description", Type: FieldString},
		{Name: "updated_at", Column: "updated_at", Type: FieldString},
	},
	Update: OpSpec{
		Fields:       []string{"value"},
		BindRequired: []string{"value"},
	},
	UpdateTimestamp:   true,
	UpdateKeyField:    "key",
	UpdateKeyMessage:  "系数 key 不能为空",
	ResponseReturning: true,
	ResponseColumns:   []string{"key", "value", "description", "updated_at"},
	InvalidateResult:  true,
	ResponseScan: func(row pgx.Row) (map[string]any, error) {
		var id int32
		var key string
		var value float64
		var description *string
		var updatedAt time.Time
		if err := row.Scan(&id, &key, &value, &description, &updatedAt); err != nil {
			return nil, err
		}
		desc := ""
		if description != nil {
			desc = *description
		}
		return map[string]any{
			"id":          id,
			"key":         key,
			"value":       value,
			"description": desc,
			"updated_at":  updatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}, nil
	},
}

// OriginalPriceDescriptor 原价：10 字段宽行 + 7 字段复合唯一 DO UPDATE +
// earliest_factory_year 默认 2000（双侧）+ original_price>0 校验 +
// 写 SQL 追加 updated_at = NOW() + 响应恒含 updated_at:""。
var OriginalPriceDescriptor = Descriptor{
	Name:            "original_prices",
	EntityLabel:     "原价记录",
	NotFoundMessage: "原价记录不存在",
	Table:           "original_prices",
	Path:            "original-prices",
	Fields: []Field{
		{Name: "brand", Column: "brand", Type: FieldString},
		{Name: "vehicle_type", Column: "vehicle_type", Type: FieldString},
		{Name: "series", Column: "series", Type: FieldString},
		{Name: "tonnage", Column: "tonnage", Type: FieldFloat},
		{Name: "config_type", Column: "config_type", Type: FieldString},
		{Name: "mast_type", Column: "mast_type", Type: FieldString},
		{Name: "mast_height_mm", Column: "mast_height_mm", Type: FieldInt},
		{Name: "earliest_factory_year", Column: "earliest_factory_year", Type: FieldInt, Default: 2000},
		{Name: "original_price", Column: "original_price", Type: FieldFloat},
		{Name: "updated_at", Column: "updated_at", Type: FieldString},
	},
	Create: OpSpec{
		Fields: []string{"brand", "vehicle_type", "series", "tonnage", "config_type",
			"mast_type", "mast_height_mm", "earliest_factory_year", "original_price"},
		Required: []string{"brand", "vehicle_type", "series"},
	},
	Update: OpSpec{
		Fields: []string{"brand", "vehicle_type", "series", "tonnage", "config_type",
			"mast_type", "mast_height_mm", "earliest_factory_year", "original_price"},
		Required: []string{"brand", "vehicle_type", "series"},
	},
	Delete:           true,
	UniqueColumns:    []string{"brand", "vehicle_type", "series", "tonnage", "config_type", "mast_type", "mast_height_mm"},
	Upsert:           UpsertDoUpdate,
	UpdateTimestamp:  true,
	ValidatePositive: []string{"original_price"},
	PositiveMessage:  "original_price 必须大于 0",
	ResponseExtra:    []string{"updated_at"},
	InvalidateResult: true,
	RequiredMessage:  "brand/vehicle_type/series 必填",
}

// AllDescriptors 返回全部字典写面描述符（路由注册与缓存契约交叉校验共用同一列表）。
func AllDescriptors() []Descriptor {
	return []Descriptor{
		BrandDescriptor,
		VehicleTypeDescriptor,
		SeriesDescriptor,
		TonnageDescriptor,
		MastTypeDescriptor,
		MastHeightDescriptor,
		BatteryTypeDescriptor,
		TransmissionTypeDescriptor,
		EngineTypeDescriptor,
		ConditionRatingDescriptor,
		RegionCoefficientDescriptor,
		CoefficientConfigDescriptor,
		OriginalPriceDescriptor,
	}
}
