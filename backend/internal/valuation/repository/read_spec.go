// 字典读面描述符引擎（ADR-0013 候选 1）：简单单表读（List/Get）的 SQL + scan
// 由 ReadSpec 声明、读面引擎生成，消除「SELECT 列顺序 + scan 列顺序」双处手工同步
// 的漂移面。字段 Name/Column/Type 复用写面 dictcrud.Field 为单点（+ 引擎自动补 id 列）。
//
// 读面独有的 SELECT 列顺序 / ORDER BY / WHERE 模板由 ReadSpec 声明；缓存 key 由
// facade 按既有契约常量（dict_cache_keys.go）+ cacheKey 构造后传入引擎——线上 key 不变。
// 特殊行解码（可空列 / 时间格式化，如 coefficient_configs）由 facade 继续传入既有
// scan 闭包走 listCached/getCached，仅 SQL 从 ReadSpec.selectSQL() 生成，保持单点。
package repository

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/jackc/pgx/v5"

	"forklift-training/internal/valuation/dictcrud"
)

// ReadSpec 一个简单单表读（List/Get）的描述符。
type ReadSpec struct {
	// Table 目标单表。
	Table string
	// Columns 有序的 SELECT 输出列（不含 id，引擎自动前置 id 列）。
	// 字段 Name 必须与目标 DTO 的 json tag 一致（反射按 Name 定位结构体字段）。
	Columns []dictcrud.Field
	// Where WHERE 模板（含 $n 占位符）；空字符串表示无过滤条件。
	Where string
	// OrderBy ORDER BY 子句（不含 "ORDER BY " 前缀）；空字符串表示无排序。
	OrderBy string
	// Distinct 是否 SELECT DISTINCT（ListProvinces 等去重读）。
	Distinct bool
	// NoID 标量读（ListProvinces/ListCities 返回 []string）不选择 id 列；
	// 默认 false 表示引擎自动前置 id 列。
	NoID bool
}

// selectSQL 由 ReadSpec 生成 SELECT SQL（id 自动前置，NoID 时不含）。
func (s ReadSpec) selectSQL() string {
	variant := "SELECT"
	if s.Distinct {
		variant = "SELECT DISTINCT"
	}
	cols := make([]string, 0, len(s.Columns)+1)
	if !s.NoID {
		cols = append(cols, "id")
	}
	for _, c := range s.Columns {
		cols = append(cols, c.Column)
	}
	q := variant + " " + strings.Join(cols, ", ") + " FROM " + s.Table
	if s.Where != "" {
		q += " WHERE " + s.Where
	}
	if s.OrderBy != "" {
		q += " ORDER BY " + s.OrderBy
	}
	return q
}

// fullColumns 返回扫描目的列（id + Columns，NoID 时仅 Columns），供反射 scan 定位字段。
func (s ReadSpec) fullColumns() []dictcrud.Field {
	cols := make([]dictcrud.Field, 0, len(s.Columns)+1)
	if !s.NoID {
		cols = append(cols, dictcrud.Field{Name: "id", Column: "id", Type: dictcrud.FieldInt})
	}
	cols = append(cols, s.Columns...)
	return cols
}

// readList 读面引擎的列表读分支：生成 SQL + 反射扫描，走 listCached 缓存。
func readList[T any](r *DictionaryRepository, ctx context.Context, spec ReadSpec, key, what string, args ...any) ([]T, error) {
	return listCached(r, ctx, key, what, spec.selectSQL(), scanRows[T](spec), args...)
}

// readGet 读面引擎的单行读分支：生成 SQL + 反射扫描，走 getCached 缓存。
func readGet[T any](r *DictionaryRepository, ctx context.Context, spec ReadSpec, key, what string, args ...any) (T, error) {
	return getCached(r, ctx, key, what, spec.selectSQL(), scanRow[T](spec), args...)
}

// readScalarList 标量列表读（[]string 等去省市）：生成 SQL + 逐行标量扫描。
func readScalarList(r *DictionaryRepository, ctx context.Context, spec ReadSpec, key, what string, args ...any) ([]string, error) {
	return listCached(r, ctx, key, what, spec.selectSQL(), func(rows pgx.Rows) (string, error) {
		var s string
		if err := rows.Scan(&s); err != nil {
			return "", err
		}
		return s, nil
	}, args...)
}

// scanRows 构造 listCached 的 scan 闭包：按 ReadSpec 列顺序反射扫描一行到 T。
func scanRows[T any](spec ReadSpec) func(pgx.Rows) (T, error) {
	return func(rows pgx.Rows) (T, error) {
		return scanInto[T](spec, rows)
	}
}

// scanRow 构造 getCached 的 scan 闭包：按 ReadSpec 列顺序反射扫描一行到 T。
func scanRow[T any](spec ReadSpec) func(pgx.Row) (T, error) {
	return func(row pgx.Row) (T, error) {
		return scanInto[T](spec, row)
	}
}

// scanInto 按 ReadSpec 全列顺序反射扫描到 T。
func scanInto[T any](spec ReadSpec, src interface{ Scan(dest ...any) error }) (T, error) {
	var v T
	cols := spec.fullColumns()
	dests := make([]any, len(cols))
	rv := reflect.ValueOf(&v).Elem()
	for i, c := range cols {
		f := fieldByJSON(rv, c.Name)
		if !f.IsValid() || !f.CanAddr() {
			return v, fmt.Errorf("读面：类型 %T 缺少字段 %q（json tag 需与 ReadSpec 列名一致）", v, c.Name)
		}
		dests[i] = f.Addr().Interface()
	}
	if err := src.Scan(dests...); err != nil {
		return v, err
	}
	return v, nil
}

// fieldByJSON 按 json tag 名定位结构体字段。
func fieldByJSON(rv reflect.Value, jsonName string) reflect.Value {
	t := rv.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name := f.Tag.Get("json")
		if idx := strings.Index(name, ","); idx >= 0 {
			name = name[:idx]
		}
		if name == jsonName {
			return rv.Field(i)
		}
	}
	return reflect.Value{}
}
