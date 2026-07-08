// Package repository - 字典表与 original_prices 数据访问
// 手写 pgx 仓储，覆盖学生端只读与管理员 CRUD 接口
// 设计参考 battery.go，统一使用 *pgxpool.Pool 直接操作
package repository

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

// =====================================================
// 字典 DTO 定义
// =====================================================

// Brand 品牌
type Brand struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	KBrand   float64 `json:"k_brand"`
	IsActive bool    `json:"is_active"`
}

// VehicleType 车型（含动力类型 electric / combustion）
type VehicleType struct {
	ID                  int    `json:"id"`
	Name                string `json:"name"`
	PowerType           string `json:"power_type"`
	EarliestFactoryYear int    `json:"earliest_factory_year"`
}

// Series 系列
type Series struct {
	ID                  int    `json:"id"`
	Brand               string `json:"brand"`
	Name                string `json:"name"`
	EarliestFactoryYear int    `json:"earliest_factory_year"`
}

// Tonnage 吨位
type Tonnage struct {
	ID    int     `json:"id"`
	Value float64 `json:"value"`
}

// MastType 门架类型
type MastType struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// MastHeight 门架高度
type MastHeight struct {
	ID      int `json:"id"`
	ValueMM int `json:"value_mm"`
}

// BatteryTypeDict 电池类型字典
type BatteryTypeDict struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// TransmissionType 传动系统字典（手波/自波/无级变速/无）
type TransmissionType struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// EngineType 发动机类型字典（国产发动机/进口发动机/混合动力/无）
type EngineType struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// SeriesConfigOptions 某 series 支持的配置维度及可选项
// 每个维度为该 series 在该维度上可选的选项列表；列表为空表示该 series 不支持此维度
type SeriesConfigOptions struct {
	Transmission []string `json:"transmission"`
	Engine       []string `json:"engine"`
	Battery      []string `json:"battery"`
}

// ConditionRating 车况评级
type ConditionRating struct {
	ID              int     `json:"id"`
	Rating          string  `json:"rating"`
	Label           string  `json:"label"`
	BaseCoefficient float64 `json:"base_coefficient"`
}

// RegionCoefficient 区域系数
type RegionCoefficient struct {
	ID          int     `json:"id"`
	Province    string  `json:"province"`
	City        string  `json:"city"`
	Coefficient float64 `json:"coefficient"`
}

// OriginalPrice 车辆原价
type OriginalPrice struct {
	ID                  int64   `json:"id"`
	Brand               string  `json:"brand"`
	VehicleType         string  `json:"vehicle_type"`
	Series              string  `json:"series"`
	Tonnage             float64 `json:"tonnage"`
	ConfigType          string  `json:"config_type"`
	MastType            string  `json:"mast_type"`
	MastHeightMM        int     `json:"mast_height_mm"`
	EarliestFactoryYear int     `json:"earliest_factory_year"`
	OriginalPrice       float64 `json:"original_price"`
	UpdatedAt           string  `json:"updated_at"`
}

// ConfigOption 配置类型选项（从 original_prices DISTINCT 派生，非字典表实体）
type ConfigOption struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// CoefficientConfig 系数配置（全局可调参数）
type CoefficientConfig struct {
	ID          int32   `json:"id"`
	Key         string  `json:"key"`
	Value       float64 `json:"value"`
	Description string  `json:"description"`
	UpdatedAt   string  `json:"updated_at"`
}

// =====================================================
// 仓储入口
// =====================================================

// DictionaryRepository 字典与原价仓储
// 持有 *pgxpool.Pool，所有方法均为线程安全（pgx 连接池内置并发控制）
type DictionaryRepository struct {
	pool *pgxpool.Pool
}

// NewDictionaryRepository 构造字典仓储
func NewDictionaryRepository(pool *pgxpool.Pool) *DictionaryRepository {
	return &DictionaryRepository{pool: pool}
}

// nullableStrPtr 把 *string 转为 SQL 占位符（nil → NULL）
// 与 battery.go 的 nullableString 不同，此处用于 *string 类型字段
func nullableStrPtr(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}
