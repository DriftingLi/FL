// 规格维度字典：tonnages / mast_types / mast_heights / battery_types / transmission_types / engine_types（从 dictionaries.go 拆分）
// 手写 pgx 仓储，统一使用 *pgxpool.Pool 直接操作
package repository

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
)

// ListTonnages 列出全部吨位
func (r *DictionaryRepository) ListTonnages(ctx context.Context) ([]Tonnage, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, value FROM tonnages ORDER BY value ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询吨位失败: %w", err)
	}
	defer rows.Close()
	out := make([]Tonnage, 0, 16)
	for rows.Next() {
		var t Tonnage
		if err := rows.Scan(&t.ID, &t.Value); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// CreateTonnage 新增吨位
func (r *DictionaryRepository) CreateTonnage(ctx context.Context, value float64) (Tonnage, error) {
	var id int
	err := r.pool.QueryRow(ctx,
		`INSERT INTO tonnages (value) VALUES ($1) ON CONFLICT (value) DO NOTHING RETURNING id`, value).Scan(&id)
	if err != nil {
		return Tonnage{}, fmt.Errorf("新增吨位失败: %w", err)
	}
	return Tonnage{ID: id, Value: value}, nil
}

// DeleteTonnage 删除吨位
func (r *DictionaryRepository) DeleteTonnage(ctx context.Context, id int) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM tonnages WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除吨位失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ListMastTypes 列出全部门架类型
func (r *DictionaryRepository) ListMastTypes(ctx context.Context) ([]MastType, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name FROM mast_types ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询门架类型失败: %w", err)
	}
	defer rows.Close()
	out := make([]MastType, 0, 8)
	for rows.Next() {
		var m MastType
		if err := rows.Scan(&m.ID, &m.Name); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// CreateMastType 新增门架类型
func (r *DictionaryRepository) CreateMastType(ctx context.Context, name string) (MastType, error) {
	var id int
	err := r.pool.QueryRow(ctx,
		`INSERT INTO mast_types (name) VALUES ($1) ON CONFLICT (name) DO NOTHING RETURNING id`, name).Scan(&id)
	if err != nil {
		return MastType{}, fmt.Errorf("新增门架类型失败: %w", err)
	}
	return MastType{ID: id, Name: name}, nil
}

// DeleteMastType 删除门架类型
func (r *DictionaryRepository) DeleteMastType(ctx context.Context, id int) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM mast_types WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除门架类型失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ListMastHeights 列出全部门架高度
func (r *DictionaryRepository) ListMastHeights(ctx context.Context) ([]MastHeight, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, value_mm FROM mast_heights ORDER BY value_mm ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询门架高度失败: %w", err)
	}
	defer rows.Close()
	out := make([]MastHeight, 0, 8)
	for rows.Next() {
		var m MastHeight
		if err := rows.Scan(&m.ID, &m.ValueMM); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// CreateMastHeight 新增门架高度
func (r *DictionaryRepository) CreateMastHeight(ctx context.Context, valueMM int) (MastHeight, error) {
	var id int
	err := r.pool.QueryRow(ctx,
		`INSERT INTO mast_heights (value_mm) VALUES ($1) ON CONFLICT (value_mm) DO NOTHING RETURNING id`, valueMM).Scan(&id)
	if err != nil {
		return MastHeight{}, fmt.Errorf("新增门架高度失败: %w", err)
	}
	return MastHeight{ID: id, ValueMM: valueMM}, nil
}

// DeleteMastHeight 删除门架高度
func (r *DictionaryRepository) DeleteMastHeight(ctx context.Context, id int) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM mast_heights WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除门架高度失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ListBatteryTypes 列出全部电池类型
func (r *DictionaryRepository) ListBatteryTypes(ctx context.Context) ([]BatteryTypeDict, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name FROM battery_types ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询电池类型失败: %w", err)
	}
	defer rows.Close()
	out := make([]BatteryTypeDict, 0, 8)
	for rows.Next() {
		var b BatteryTypeDict
		if err := rows.Scan(&b.ID, &b.Name); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// CreateBatteryType 新增电池类型
func (r *DictionaryRepository) CreateBatteryType(ctx context.Context, name string) (BatteryTypeDict, error) {
	var id int
	err := r.pool.QueryRow(ctx,
		`INSERT INTO battery_types (name) VALUES ($1) ON CONFLICT (name) DO NOTHING RETURNING id`, name).Scan(&id)
	if err != nil {
		return BatteryTypeDict{}, fmt.Errorf("新增电池类型失败: %w", err)
	}
	return BatteryTypeDict{ID: id, Name: name}, nil
}

// DeleteBatteryType 删除电池类型
func (r *DictionaryRepository) DeleteBatteryType(ctx context.Context, id int) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM battery_types WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除电池类型失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ListTransmissionTypes 列出全部传动系统类型
func (r *DictionaryRepository) ListTransmissionTypes(ctx context.Context) ([]TransmissionType, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name FROM transmission_types ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询传动系统类型失败: %w", err)
	}
	defer rows.Close()
	out := make([]TransmissionType, 0, 8)
	for rows.Next() {
		var t TransmissionType
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListEngineTypes 列出全部发动机类型
func (r *DictionaryRepository) ListEngineTypes(ctx context.Context) ([]EngineType, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name FROM engine_types ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询发动机类型失败: %w", err)
	}
	defer rows.Close()
	out := make([]EngineType, 0, 8)
	for rows.Next() {
		var e EngineType
		if err := rows.Scan(&e.ID, &e.Name); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CreateTransmissionType 新增传动系统类型
func (r *DictionaryRepository) CreateTransmissionType(ctx context.Context, name string) (TransmissionType, error) {
	var id int
	err := r.pool.QueryRow(ctx,
		`INSERT INTO transmission_types (name) VALUES ($1) ON CONFLICT (name) DO NOTHING RETURNING id`, name).Scan(&id)
	if err != nil {
		return TransmissionType{}, fmt.Errorf("新增传动系统类型失败: %w", err)
	}
	return TransmissionType{ID: id, Name: name}, nil
}

// DeleteTransmissionType 删除传动系统类型
func (r *DictionaryRepository) DeleteTransmissionType(ctx context.Context, id int) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM transmission_types WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除传动系统类型失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// CreateEngineType 新增发动机类型
func (r *DictionaryRepository) CreateEngineType(ctx context.Context, name string) (EngineType, error) {
	var id int
	err := r.pool.QueryRow(ctx,
		`INSERT INTO engine_types (name) VALUES ($1) ON CONFLICT (name) DO NOTHING RETURNING id`, name).Scan(&id)
	if err != nil {
		return EngineType{}, fmt.Errorf("新增发动机类型失败: %w", err)
	}
	return EngineType{ID: id, Name: name}, nil
}

// DeleteEngineType 删除发动机类型
func (r *DictionaryRepository) DeleteEngineType(ctx context.Context, id int) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM engine_types WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除发动机类型失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
