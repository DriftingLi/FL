// specs 规格字典（吨位/门架类型/门架高度/电池类型/传动/发动机）CRUD
// 骨架走 dict_helpers 共享实现
package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// tonnages
// =====================================================

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

// CreateTonnage 新增吨位
func (r *DictionaryRepository) CreateTonnage(ctx context.Context, value float64) (Tonnage, error) {
	id, err := insertReturningID(r, ctx, "新增吨位",
		`INSERT INTO tonnages (value) VALUES ($1) ON CONFLICT (value) DO NOTHING RETURNING id`, value)
	if err != nil {
		return Tonnage{}, err
	}
	return Tonnage{ID: int(id), Value: value}, nil
}

// DeleteTonnage 删除吨位
func (r *DictionaryRepository) DeleteTonnage(ctx context.Context, id int) error {
	return execWrite(r, ctx, "删除吨位", `DELETE FROM tonnages WHERE id = $1`, id)
}

// =====================================================
// mast_types
// =====================================================

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

// CreateMastType 新增门架类型
func (r *DictionaryRepository) CreateMastType(ctx context.Context, name string) (MastType, error) {
	id, err := insertReturningID(r, ctx, "新增门架类型",
		`INSERT INTO mast_types (name) VALUES ($1) ON CONFLICT (name) DO NOTHING RETURNING id`, name)
	if err != nil {
		return MastType{}, err
	}
	return MastType{ID: int(id), Name: name}, nil
}

// DeleteMastType 删除门架类型
func (r *DictionaryRepository) DeleteMastType(ctx context.Context, id int) error {
	return execWrite(r, ctx, "删除门架类型", `DELETE FROM mast_types WHERE id = $1`, id)
}

// =====================================================
// mast_heights
// =====================================================

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

// CreateMastHeight 新增门架高度
func (r *DictionaryRepository) CreateMastHeight(ctx context.Context, valueMM int) (MastHeight, error) {
	id, err := insertReturningID(r, ctx, "新增门架高度",
		`INSERT INTO mast_heights (value_mm) VALUES ($1) ON CONFLICT (value_mm) DO NOTHING RETURNING id`, valueMM)
	if err != nil {
		return MastHeight{}, err
	}
	return MastHeight{ID: int(id), ValueMM: valueMM}, nil
}

// DeleteMastHeight 删除门架高度
func (r *DictionaryRepository) DeleteMastHeight(ctx context.Context, id int) error {
	return execWrite(r, ctx, "删除门架高度", `DELETE FROM mast_heights WHERE id = $1`, id)
}

// =====================================================
// battery_types
// =====================================================

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

// CreateBatteryType 新增电池类型
func (r *DictionaryRepository) CreateBatteryType(ctx context.Context, name string) (BatteryTypeDict, error) {
	id, err := insertReturningID(r, ctx, "新增电池类型",
		`INSERT INTO battery_types (name) VALUES ($1) ON CONFLICT (name) DO NOTHING RETURNING id`, name)
	if err != nil {
		return BatteryTypeDict{}, err
	}
	return BatteryTypeDict{ID: int(id), Name: name}, nil
}

// DeleteBatteryType 删除电池类型
func (r *DictionaryRepository) DeleteBatteryType(ctx context.Context, id int) error {
	return execWrite(r, ctx, "删除电池类型", `DELETE FROM battery_types WHERE id = $1`, id)
}

// =====================================================
// transmission_types
// =====================================================

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

// CreateTransmissionType 新增传动系统类型
func (r *DictionaryRepository) CreateTransmissionType(ctx context.Context, name string) (TransmissionType, error) {
	id, err := insertReturningID(r, ctx, "新增传动系统类型",
		`INSERT INTO transmission_types (name) VALUES ($1) ON CONFLICT (name) DO NOTHING RETURNING id`, name)
	if err != nil {
		return TransmissionType{}, err
	}
	return TransmissionType{ID: int(id), Name: name}, nil
}

// DeleteTransmissionType 删除传动系统类型
func (r *DictionaryRepository) DeleteTransmissionType(ctx context.Context, id int) error {
	return execWrite(r, ctx, "删除传动系统类型", `DELETE FROM transmission_types WHERE id = $1`, id)
}

// =====================================================
// engine_types
// =====================================================

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

// CreateEngineType 新增发动机类型
func (r *DictionaryRepository) CreateEngineType(ctx context.Context, name string) (EngineType, error) {
	id, err := insertReturningID(r, ctx, "新增发动机类型",
		`INSERT INTO engine_types (name) VALUES ($1) ON CONFLICT (name) DO NOTHING RETURNING id`, name)
	if err != nil {
		return EngineType{}, err
	}
	return EngineType{ID: int(id), Name: name}, nil
}

// DeleteEngineType 删除发动机类型
func (r *DictionaryRepository) DeleteEngineType(ctx context.Context, id int) error {
	return execWrite(r, ctx, "删除发动机类型", `DELETE FROM engine_types WHERE id = $1`, id)
}
