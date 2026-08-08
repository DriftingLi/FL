// specs 规格字典（吨位/门架类型/门架高度/电池类型/传动/发动机）：只读方法
// （写面已迁至 dictcrud 描述符驱动，见 ADR-0008）。
package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// ListTonnages 列出全部吨位
func (r *DictionaryRepository) ListTonnages(ctx context.Context) ([]Tonnage, error) {
	return listCached(r, ctx, CacheKeyTonnagesList, "查询吨位",
		`SELECT id, value FROM tonnages ORDER BY value ASC`,
		func(rows pgx.Rows) (Tonnage, error) {
			var t Tonnage
			err := rows.Scan(&t.ID, &t.Value)
			return t, err
		})
}

// ListMastTypes 列出全部门架类型
func (r *DictionaryRepository) ListMastTypes(ctx context.Context) ([]MastType, error) {
	return listCached(r, ctx, CacheKeyMastTypesList, "查询门架类型",
		`SELECT id, name FROM mast_types ORDER BY id ASC`,
		func(rows pgx.Rows) (MastType, error) {
			var m MastType
			err := rows.Scan(&m.ID, &m.Name)
			return m, err
		})
}

// ListMastHeights 列出全部门架高度
func (r *DictionaryRepository) ListMastHeights(ctx context.Context) ([]MastHeight, error) {
	return listCached(r, ctx, CacheKeyMastHeightsList, "查询门架高度",
		`SELECT id, value_mm FROM mast_heights ORDER BY value_mm ASC`,
		func(rows pgx.Rows) (MastHeight, error) {
			var m MastHeight
			err := rows.Scan(&m.ID, &m.ValueMM)
			return m, err
		})
}

// ListBatteryTypes 列出全部电池类型
func (r *DictionaryRepository) ListBatteryTypes(ctx context.Context) ([]BatteryTypeDict, error) {
	return listCached(r, ctx, CacheKeyBatteryTypesList, "查询电池类型",
		`SELECT id, name FROM battery_types ORDER BY id ASC`,
		func(rows pgx.Rows) (BatteryTypeDict, error) {
			var b BatteryTypeDict
			err := rows.Scan(&b.ID, &b.Name)
			return b, err
		})
}

// ListTransmissionTypes 列出全部传动系统类型
func (r *DictionaryRepository) ListTransmissionTypes(ctx context.Context) ([]TransmissionType, error) {
	return listCached(r, ctx, CacheKeyTransmissionTypesList, "查询传动系统",
		`SELECT id, name FROM transmission_types ORDER BY id ASC`,
		func(rows pgx.Rows) (TransmissionType, error) {
			var t TransmissionType
			err := rows.Scan(&t.ID, &t.Name)
			return t, err
		})
}

// ListEngineTypes 列出全部发动机类型
func (r *DictionaryRepository) ListEngineTypes(ctx context.Context) ([]EngineType, error) {
	return listCached(r, ctx, CacheKeyEngineTypesList, "查询发动机类型",
		`SELECT id, name FROM engine_types ORDER BY id ASC`,
		func(rows pgx.Rows) (EngineType, error) {
			var e EngineType
			err := rows.Scan(&e.ID, &e.Name)
			return e, err
		})
}
