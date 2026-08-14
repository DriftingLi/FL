// specs 规格字典（吨位/门架类型/门架高度/电池类型/传动/发动机）：只读方法
// （写面已迁至 dictcrud 描述符驱动，见 ADR-0008）。
package repository

import (
	"context"
)

// ListTonnages 列出全部吨位
func (r *DictionaryRepository) ListTonnages(ctx context.Context) ([]Tonnage, error) {
	return readList[Tonnage](r, ctx, readSpecTonnagesList, CacheKeyTonnagesList, "查询吨位")
}

// ListMastTypes 列出全部门架类型
func (r *DictionaryRepository) ListMastTypes(ctx context.Context) ([]MastType, error) {
	return readList[MastType](r, ctx, readSpecMastTypesList, CacheKeyMastTypesList, "查询门架类型")
}

// ListMastHeights 列出全部门架高度
func (r *DictionaryRepository) ListMastHeights(ctx context.Context) ([]MastHeight, error) {
	return readList[MastHeight](r, ctx, readSpecMastHeightsList, CacheKeyMastHeightsList, "查询门架高度")
}

// ListBatteryTypes 列出全部电池类型
func (r *DictionaryRepository) ListBatteryTypes(ctx context.Context) ([]BatteryTypeDict, error) {
	return readList[BatteryTypeDict](r, ctx, readSpecBatteryTypesList, CacheKeyBatteryTypesList, "查询电池类型")
}

// ListTransmissionTypes 列出全部传动系统类型
func (r *DictionaryRepository) ListTransmissionTypes(ctx context.Context) ([]TransmissionType, error) {
	return readList[TransmissionType](r, ctx, readSpecTransmissionTypesList, CacheKeyTransmissionTypesList, "查询传动系统")
}

// ListEngineTypes 列出全部发动机类型
func (r *DictionaryRepository) ListEngineTypes(ctx context.Context) ([]EngineType, error) {
	return readList[EngineType](r, ctx, readSpecEngineTypesList, CacheKeyEngineTypesList, "查询发动机类型")
}
