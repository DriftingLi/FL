// 描述符驱动的 SQL/响应纯函数生成器：CRUD 写 SQL、参数、响应形状。
// 生成规则（测试逐字符锁定，见 builder_test.go）：
//
//	INSERT INTO <table> (cols...) VALUES ($1,...)
//	  + ON CONFLICT (<unique>) DO NOTHING                       [UpsertDoNothing]
//	  + ON CONFLICT (<unique>) DO UPDATE SET <非唯一 create 列> = EXCLUDED.<col>...
//	    RETURNING id                                            [UpsertDoUpdate]
//	UPDATE <table> SET <update 列> = $n... WHERE id = $1
//	DELETE FROM <table> WHERE id = $1
//
// 响应：create → {"id"} + Create.Fields；update → {"id"} + Update.Fields；delete → {"id"}。
package dictcrud

import (
	"strconv"
	"strings"
)

// columnsOf 返回操作字段按声明顺序的 DB 列名。
func columnsOf(d Descriptor, names []string) []string {
	cols := make([]string, 0, len(names))
	for _, name := range names {
		if f, ok := d.Field(name); ok {
			cols = append(cols, f.Column)
		}
	}
	return cols
}

// BuildInsertSQL 由描述符生成 INSERT SQL（含可选 ON CONFLICT 子句 + RETURNING id）。
// UpdateTimestamp 时 DO UPDATE SET 尾列追加 updated_at = NOW()。
func BuildInsertSQL(d Descriptor) string {
	cols := columnsOf(d, d.Create.Fields)
	var b strings.Builder
	b.WriteString("INSERT INTO " + d.Table + " (" + strings.Join(cols, ", ") + ") VALUES ($1")
	for i := 1; i < len(cols); i++ {
		b.WriteString(", $" + strconv.Itoa(i+1))
	}
	b.WriteString(")")
	switch d.Upsert {
	case UpsertDoNothing:
		b.WriteString(" ON CONFLICT (" + strings.Join(d.UniqueColumns, ", ") + ") DO NOTHING")
	case UpsertDoUpdate:
		b.WriteString(" ON CONFLICT (" + strings.Join(d.UniqueColumns, ", ") + ") DO UPDATE SET ")
		updates := make([]string, 0, len(d.Create.Fields))
		for _, name := range d.Create.Fields {
			f, ok := d.Field(name)
			if !ok || contains(d.UniqueColumns, f.Column) {
				continue
			}
			updates = append(updates, f.Column+" = EXCLUDED."+f.Column)
		}
		b.WriteString(strings.Join(updates, ", "))
		if d.UpdateTimestamp {
			b.WriteString(", updated_at = NOW()")
		}
	}
	b.WriteString(" RETURNING id")
	return b.String()
}

// BuildInsertArgs 由描述符生成 INSERT 参数（create 字段顺序，按类型强转）。
func BuildInsertArgs(d Descriptor, values map[string]any) []any {
	args := make([]any, 0, len(d.Create.Fields))
	for _, name := range d.Create.Fields {
		f, _ := d.Field(name)
		args = append(args, coerce(f.Type, values[name]))
	}
	return args
}

// BuildUpdateSQL 由描述符生成 UPDATE SQL（只更新 Update.Fields 列）。
// UpdateTimestamp 时 SET 尾列追加 updated_at = NOW()。
func BuildUpdateSQL(d Descriptor) string {
	cols := columnsOf(d, d.Update.Fields)
	sets := make([]string, len(cols))
	for i, c := range cols {
		sets[i] = c + " = $" + strconv.Itoa(i+2)
	}
	q := "UPDATE " + d.Table + " SET " + strings.Join(sets, ", ")
	if d.UpdateTimestamp {
		q += ", updated_at = NOW()"
	}
	return q + " WHERE id = $1"
}

// BuildUpdateKeySQL 按唯一 key 列更新（$1 = key 值；coefficient_configs）：
// UPDATE <table> SET <update 列> = $n... [ , updated_at = NOW()] WHERE <key 列> = $1
// [+ RETURNING id, <ResponseColumns>]。
func BuildUpdateKeySQL(d Descriptor) string {
	cols := columnsOf(d, d.Update.Fields)
	sets := make([]string, len(cols))
	for i, c := range cols {
		sets[i] = c + " = $" + strconv.Itoa(i+2)
	}
	var b strings.Builder
	b.WriteString("UPDATE " + d.Table + " SET " + strings.Join(sets, ", "))
	if d.UpdateTimestamp {
		b.WriteString(", updated_at = NOW()")
	}
	f, _ := d.Field(d.UpdateKeyField)
	b.WriteString(" WHERE " + f.Column + " = $1")
	if d.ResponseReturning {
		b.WriteString(" RETURNING id, " + strings.Join(columnsOf(d, d.ResponseColumns), ", "))
	}
	return b.String()
}

// BuildUpdateArgs 由描述符生成 UPDATE 参数（id + update 字段）。
func BuildUpdateArgs(d Descriptor, id int64, values map[string]any) []any {
	args := make([]any, 0, len(d.Update.Fields)+1)
	args = append(args, id)
	for _, name := range d.Update.Fields {
		f, _ := d.Field(name)
		args = append(args, coerce(f.Type, values[name]))
	}
	return args
}

// BuildUpdateKeyArgs 按 key 更新的参数（key 值 + update 字段）。
func BuildUpdateKeyArgs(d Descriptor, key string, values map[string]any) []any {
	args := make([]any, 0, len(d.Update.Fields)+1)
	args = append(args, key)
	for _, name := range d.Update.Fields {
		f, _ := d.Field(name)
		args = append(args, coerce(f.Type, values[name]))
	}
	return args
}

// BuildDeleteSQL 由描述符生成 DELETE SQL。
func BuildDeleteSQL(d Descriptor) string {
	return "DELETE FROM " + d.Table + " WHERE id = $1"
}

// BuildCreateResult create 响应：{"id": id} + create 字段值（声明顺序）。
func BuildCreateResult(d Descriptor, id int64, values map[string]any) map[string]any {
	return buildResult(d, d.Create.Fields, id, values)
}

// BuildUpdateResult update 响应：{"id": id} + update 字段值（声明顺序）。
func BuildUpdateResult(d Descriptor, id int64, values map[string]any) map[string]any {
	return buildResult(d, d.Update.Fields, id, values)
}

// buildResult 响应构造：{"id"} + 操作字段（声明顺序）+ ResponseExtra 零值字段。
func buildResult(d Descriptor, names []string, id int64, values map[string]any) map[string]any {
	out := make(map[string]any, len(names)+len(d.ResponseExtra)+1)
	out["id"] = id
	for _, name := range names {
		if v, ok := values[name]; ok {
			out[name] = v
		}
	}
	for _, name := range d.ResponseExtra {
		if f, ok := d.Field(name); ok {
			out[name] = ZeroValue(f)
		}
	}
	return out
}

// ApplyDefaults 将 op 参与字段中的零值替换为描述符默认值（create/update 双侧，
// 与迁移前 handler 的 "if x == 0 { x = default }" 行为一致）。
func ApplyDefaults(d Descriptor, values map[string]any) {
	for _, name := range d.Create.Fields {
		applyDefault(d, name, values)
	}
	for _, name := range d.Update.Fields {
		applyDefault(d, name, values)
	}
}

func applyDefault(d Descriptor, name string, values map[string]any) {
	f, ok := d.Field(name)
	if !ok || f.Default == nil {
		return
	}
	if isZeroValue(values[name]) {
		values[name] = f.Default
	}
}

// ZeroValue 返回字段类型的零值（body 缺失字段按 struct 零值语义绑定）。
func ZeroValue(f Field) any {
	switch f.Type {
	case FieldString:
		return ""
	case FieldFloat:
		return 0.0
	case FieldInt:
		return 0
	case FieldBool:
		return false
	}
	return nil
}

func isZeroValue(v any) bool {
	switch v := v.(type) {
	case string:
		return v == ""
	case float64:
		return v == 0
	case int:
		return v == 0
	case bool:
		return !v
	}
	return v == nil
}

func coerce(t FieldType, v any) any {
	switch t {
	case FieldString:
		return v.(string)
	case FieldFloat:
		return v.(float64)
	case FieldInt:
		return v.(int)
	case FieldBool:
		return v.(bool)
	}
	return v
}
