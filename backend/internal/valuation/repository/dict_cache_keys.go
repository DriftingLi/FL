// 字典缓存 key 契约：读 key 与失效 pattern 单点定义。
//
// 不变式（static 测试锁定，见 dict_cache_keys_test.go）：
// 每实体的读 key 前缀 ⊆ 该实体写操作的失效 pattern 集。
// 历史上读 key（dict:*:cascade:*）与失效 pattern（dict:specs:*）两套命名互不包含，
// 导致级联数据在 TTL 窗口内陈旧——本文件让契约只有一个来源。
package repository

import (
	"forklift-training/internal/cache"
)

// 读 key 常量（精确 key，listCached 使用）。
const (
	CacheKeyBrandsList            = "dict:brands:list"
	CacheKeyCoefList              = "dict:coef:list"
	CacheKeyAlgoParams            = "dict:algo_params"
	CacheKeyTonnagesList          = "dict:specs:tonnages:list"
	CacheKeyMastTypesList         = "dict:specs:mast_types:list"
	CacheKeyMastHeightsList       = "dict:specs:mast_heights:list"
	CacheKeyBatteryTypesList      = "dict:specs:battery_types:list"
	CacheKeyTransmissionTypesList = "dict:specs:transmission_types:list"
	CacheKeyEngineTypesList       = "dict:specs:engine_types:list"
)

// 读 key 前缀常量（SafeKey 构造 key 的公共前缀）。
const (
	CachePrefixBrandGet           = "dict:brand:get"
	CachePrefixVtGet              = "dict:vt:get"
	CachePrefixVtByBrand          = "dict:vt:bybrand"
	CachePrefixSeriesByBrand      = "dict:series:bybrand"
	CachePrefixSeriesCascade      = "dict:series:cascade"
	CachePrefixSco                = "dict:sco"
	CachePrefixTonnagesCascade    = "dict:tonnages:cascade"
	CachePrefixConfigCascade      = "dict:config:cascade"
	CachePrefixMastTypesCascade   = "dict:mast_types:cascade"
	CachePrefixMastHeightsCascade = "dict:mast_heights:cascade"
	CachePrefixBatteryCascade     = "dict:battery:cascade"
	CachePrefixEarliestYear       = "dict:earliest_year"
	CachePrefixCoefGet            = "dict:coef:get"
	CachePrefixConditionGet       = "dict:condition:get"
	CachePrefixOpMatch            = "dict:op:match"
	CachePrefixOpFuzzy            = "dict:op:fuzzy"
	CachePrefixRegionList         = "dict:region:list"
	CachePrefixRegionCities       = "dict:region:cities"
	CachePrefixRegionGet          = "dict:region:get"
)

// cacheKey 按前缀 + 剩余部分构造缓存 key（与历史 SafeKey 产物一致）。
func cacheKey(prefix string, parts ...string) string {
	return prefix + ":" + cache.SafeKey(parts...)
}

// cacheContract 一个字典实体的缓存契约。
type cacheContract struct {
	// Name 实体名（失效方法按名查找）。
	Name string
	// ReadKeyPrefixes 该实体全部读 key 的公共前缀（精确 key 也表达为自身前缀）。
	ReadKeyPrefixes []string
	// InvalidatePatterns 该实体写操作必须失效的 pattern 集（读前缀的超集）。
	InvalidatePatterns []string
}

// dictCacheContracts 全部字典实体缓存契约。
// 注意：original_prices 用兜底 pattern "dict:*"（覆盖全部读 key），
// 其余实体各自收敛级联读前缀到失效集，不依赖兜底。
var dictCacheContracts = []cacheContract{
	{
		Name:               "brands",
		ReadKeyPrefixes:    []string{CacheKeyBrandsList, CachePrefixBrandGet},
		InvalidatePatterns: []string{"dict:brands:*", "dict:brand:get:*"},
	},
	{
		Name:               "vehicle_types",
		ReadKeyPrefixes:    []string{CachePrefixVtGet, CachePrefixVtByBrand},
		InvalidatePatterns: []string{"dict:vt:*"},
	},
	{
		Name:               "series",
		ReadKeyPrefixes:    []string{CachePrefixSeriesByBrand, CachePrefixSeriesCascade, CachePrefixSco},
		InvalidatePatterns: []string{"dict:series:*", "dict:sco:*", "dict:battery:cascade:*"},
	},
	{
		Name:               "tonnages",
		ReadKeyPrefixes:    []string{CacheKeyTonnagesList, CachePrefixTonnagesCascade},
		InvalidatePatterns: []string{"dict:specs:tonnages:*", "dict:tonnages:cascade:*"},
	},
	{
		Name:               "mast_types",
		ReadKeyPrefixes:    []string{CacheKeyMastTypesList, CachePrefixMastTypesCascade},
		InvalidatePatterns: []string{"dict:specs:mast_types:*", "dict:mast_types:cascade:*"},
	},
	{
		Name:               "mast_heights",
		ReadKeyPrefixes:    []string{CacheKeyMastHeightsList, CachePrefixMastHeightsCascade},
		InvalidatePatterns: []string{"dict:specs:mast_heights:*", "dict:mast_heights:cascade:*"},
	},
	{
		Name:               "battery_types",
		ReadKeyPrefixes:    []string{CacheKeyBatteryTypesList, CachePrefixBatteryCascade},
		InvalidatePatterns: []string{"dict:specs:battery_types:*", "dict:battery:cascade:*"},
	},
	{
		Name:               "transmission_types",
		ReadKeyPrefixes:    []string{CacheKeyTransmissionTypesList},
		InvalidatePatterns: []string{"dict:specs:transmission_types:*"},
	},
	{
		Name:               "engine_types",
		ReadKeyPrefixes:    []string{CacheKeyEngineTypesList},
		InvalidatePatterns: []string{"dict:specs:engine_types:*"},
	},
	{
		Name:               "condition_ratings",
		ReadKeyPrefixes:    []string{CachePrefixConditionGet},
		InvalidatePatterns: []string{"dict:condition:*"},
	},
	{
		Name:               "region_coefficients",
		ReadKeyPrefixes:    []string{CachePrefixRegionList, CachePrefixRegionCities, CachePrefixRegionGet},
		InvalidatePatterns: []string{"dict:region:*"},
	},
	{
		Name:               "coefficient_configs",
		ReadKeyPrefixes:    []string{CachePrefixCoefGet, CacheKeyCoefList, CacheKeyAlgoParams},
		InvalidatePatterns: []string{"dict:coef:*", CacheKeyAlgoParams},
	},
	{
		Name:               "original_prices",
		ReadKeyPrefixes:    []string{CachePrefixOpMatch, CachePrefixOpFuzzy, CachePrefixEarliestYear, CachePrefixConfigCascade},
		InvalidatePatterns: []string{"dict:*"},
	},
}

// PatternsOf 按实体名返回失效 pattern 集（契约同源，handler 不再书写 pattern 字面量）。
// 描述符驱动的写面按 Descriptor.Name 查找同一契约。
func PatternsOf(name string) []string {
	for _, c := range dictCacheContracts {
		if c.Name == name {
			return c.InvalidatePatterns
		}
	}
	return nil
}

// ResultCachePattern 评估结果缓存 pattern（评估结果依赖系数配置，字典写操作需一并失效）。
// 跨模块依赖的唯一缓存 pattern 定义点，handler 不再书写字面量。
const ResultCachePattern = "valuation:result:*"
