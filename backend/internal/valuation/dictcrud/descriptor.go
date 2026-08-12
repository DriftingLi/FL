// Package dictcrud 字典描述符驱动的机械写面核心（ADR-0008）。
//
// 每种字典用一个声明式描述符定义（字段、操作参与集、唯一约束、upsert 模式、
// 默认值、校验、缓存失效标记），CRUD 写 SQL 与响应形状由描述符纯函数生成，
// 异构处全部落在描述符声明里，核心代码无 if-branch 实体分支。
// 只读侧保持每实体 typed 方法（repository/dict_*.go），不进入本包。
package dictcrud

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// FieldType 描述符字段类型（body 解码与 SQL 参数类型）。
type FieldType int

const (
	// FieldString 字符串字段。
	FieldString FieldType = iota
	// FieldFloat 浮点字段（DECIMAL 列）。
	FieldFloat
	// FieldInt 整数字段。
	FieldInt
	// FieldBool 布尔字段。
	FieldBool
)

// Field 一个可写字段：JSON 名（body 与响应共用）→ DB 列映射 + 类型。
type Field struct {
	// Name JSON 字段名（body 接受与响应输出共用）。
	Name string
	// Column DB 列名。
	Column string
	// Type 字段类型。
	Type FieldType
	// BindName bind 必填错误消息里的 Go 字段名（复制 gin binding:"required"
	// 的输出 "Key: 'body.<BindName>' required"，缺省取大写 Name）。
	BindName string
	// Default 零值字段应用默认值（create/update 双侧，与现状一致）。
	Default any
}

// OpSpec 一次写操作（create/update）的字段参与声明。
type OpSpec struct {
	// Fields 参与该操作的字段名（顺序 = body 接受顺序 = 响应字段顺序）。
	Fields []string
	// BindRequired bind 层必填字段：缺失 → 400 "请求体格式错误: ..."。
	BindRequired []string
	// Required 应用层必填字段：缺失/空字符串 → 400 RequiredMessage。
	Required []string
}

// UpsertMode 唯一冲突处理模式。
type UpsertMode int

const (
	// UpsertNone 无 ON CONFLICT 子句。
	UpsertNone UpsertMode = iota
	// UpsertDoNothing 冲突时跳过（ON CONFLICT ... DO NOTHING）。
	UpsertDoNothing
	// UpsertDoUpdate 冲突时更新非唯一列（ON CONFLICT ... DO UPDATE SET col = EXCLUDED.col）。
	UpsertDoUpdate
)

// Descriptor 一个字典实体的声明式描述符。
// Name 与缓存契约名一致（repository.PatternsOf 查找失效 pattern 的键）。
type Descriptor struct {
	// Name 实体名（= 缓存契约名，registry key）。
	Name string
	// EntityLabel 中文实体名（日志与 500 消息："新增<label>失败"）。
	EntityLabel string
	// NotFoundMessage 更新/删除未命中时的 404 消息。
	// NotFoundWithValue 为 true 时消息追加 " <key 值>"（coefficient_configs 动态 key 消息）。
	NotFoundMessage string
	// NotFoundWithValue 404 消息后追加 " <更新 key 值>"。
	NotFoundWithValue bool
	// Table DB 表名。
	Table string
	// Path 管理端路由段（POST /admin/<path>；PUT/DELETE /admin/<path>/:id）。
	Path string
	// Fields 全部可写字段字典（Create/Update 按名引用；响应追加字段也在其中声明）。
	Fields []Field
	// Create 创建操作声明；Fields 为空表示该实体无 POST（不注册路由）。
	Create OpSpec
	// Update 更新操作声明；Fields 为空表示该实体无 PUT（不注册路由）。
	Update OpSpec
	// Delete 是否暴露 DELETE（false 不注册路由）。
	Delete bool
	// UniqueColumns ON CONFLICT 目标列（upsert 模式必需，且必须是已声明列）。
	UniqueColumns []string
	// Upsert 唯一冲突处理模式。
	Upsert UpsertMode
	// InvalidateResult 写操作是否追加失效评估结果缓存（ResultCachePattern）。
	// series 为 false（现状），其余实体为 true。
	InvalidateResult bool
	// RequiredMessage 应用层必填校验失败消息（如 "province 与 city 必填"）。
	RequiredMessage string
	// UpdateTimestamp 写 SQL 尾列追加 updated_at = NOW()
	// （DO UPDATE SET 与 UPDATE SET 双侧；original_prices / coefficient_configs）。
	UpdateTimestamp bool
	// ValidatePositive 数值字段必须 > 0（create/update 双侧应用层校验；
	// 失败 → 400 PositiveMessage；original_prices 的 original_price）。
	ValidatePositive []string
	// PositiveMessage ValidatePositive 校验失败消息。
	PositiveMessage string
	// ResponseExtra 响应追加字段（仅响应、不可写；按类型零值输出，尾随操作字段。
	// original_prices 的 updated_at 用——现状响应恒含 updated_at:""）。
	ResponseExtra []string
	// UpdateKeyField 非空 → 更新走 PUT /admin/<path>/:key（key 为该字段 JSON 名），
	// UPDATE WHERE <该列> = $1（coefficient_configs；区别于默认的 :id 路由）。
	UpdateKeyField string
	// UpdateKeyMessage key 参数缺失时的 400 消息。
	UpdateKeyMessage string
	// ResponseReturning 更新操作 RETURNING 全行（列 = id + ResponseColumns），
	// 行由 ResponseScan 扫描为响应（coefficient_configs：description 可空 + updated_at 格式化）。
	ResponseReturning bool
	// ResponseColumns ResponseReturning 的返回列（字段 JSON 名，按序；id 自动前置）。
	ResponseColumns []string
	// ResponseScan ResponseReturning 的行扫描器（返回响应 JSON 名 → 值）。
	ResponseScan func(row pgx.Row) (map[string]any, error)
}

// Field 按 JSON 名返回字段声明。
func (d Descriptor) Field(name string) (Field, bool) {
	for _, f := range d.Fields {
		if f.Name == name {
			return f, true
		}
	}
	return Field{}, false
}

// BindName 返回字段的 bind 错误消息名（缺省大写 JSON 名）。
func (f Field) BindNameOr() string {
	if f.BindName != "" {
		return f.BindName
	}
	if len(f.Name) == 0 {
		return ""
	}
	return string(f.Name[0]-32) + f.Name[1:]
}

// Validate 校验描述符声明一致性（注册时调用，非法即 panic）。
// 规则：Create/Update 至少一个非空；Create/Update/BindRequired/Required 引用的字段必须已声明；
// upsert 模式必须声明唯一列且唯一列必须是已声明列；默认值类型与字段类型一致；
// keyed 更新（UpdateKeyField）与全行响应（ResponseReturning）的声明一致。
func (d Descriptor) Validate() error {
	if d.Name == "" || d.Table == "" || d.Path == "" {
		return errors.New("Name/Table/Path 必填")
	}
	if len(d.Create.Fields) == 0 && len(d.Update.Fields) == 0 {
		return errors.New("Create.Fields 与 Update.Fields 至少一个非空")
	}
	for _, spec := range []struct {
		kind string
		spec OpSpec
	}{{"Create", d.Create}, {"Update", d.Update}} {
		for _, name := range spec.spec.Fields {
			if _, ok := d.Field(name); !ok {
				return fmt.Errorf("%s 引用未声明字段 %q", spec.kind, name)
			}
		}
		for _, name := range spec.spec.BindRequired {
			if _, ok := d.Field(name); !ok {
				return fmt.Errorf("%s.BindRequired 引用未声明字段 %q", spec.kind, name)
			}
		}
		for _, name := range spec.spec.Required {
			if _, ok := d.Field(name); !ok {
				return fmt.Errorf("%s.Required 引用未声明字段 %q", spec.kind, name)
			}
			if !contains(d.Create.Fields, name) {
				return fmt.Errorf("%s.Required 字段 %q 不在 %s.Fields 中", spec.kind, name, spec.kind)
			}
		}
	}
	switch d.Upsert {
	case UpsertDoNothing, UpsertDoUpdate:
		if len(d.UniqueColumns) == 0 {
			return errors.New("upsert 模式必须声明 UniqueColumns")
		}
		for _, col := range d.UniqueColumns {
			found := false
			for _, f := range d.Fields {
				if f.Column == col {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("UniqueColumns 列 %q 未在 Fields 中声明", col)
			}
		}
	}
	for _, name := range d.ValidatePositive {
		f, ok := d.Field(name)
		if !ok {
			return fmt.Errorf("ValidatePositive 引用未声明字段 %q", name)
		}
		if f.Type != FieldFloat && f.Type != FieldInt {
			return fmt.Errorf("ValidatePositive 字段 %q 必须为数值类型", name)
		}
	}
	for _, name := range d.ResponseExtra {
		if _, ok := d.Field(name); !ok {
			return fmt.Errorf("ResponseExtra 引用未声明字段 %q", name)
		}
	}
	if d.UpdateKeyField != "" {
		if _, ok := d.Field(d.UpdateKeyField); !ok {
			return fmt.Errorf("UpdateKeyField 引用未声明字段 %q", d.UpdateKeyField)
		}
		if d.UpdateKeyMessage == "" {
			return errors.New("UpdateKeyField 必须声明 UpdateKeyMessage")
		}
	}
	if d.ResponseReturning {
		if d.ResponseScan == nil {
			return errors.New("ResponseReturning 必须声明 ResponseScan")
		}
		if len(d.ResponseColumns) == 0 {
			return errors.New("ResponseReturning 必须声明 ResponseColumns")
		}
		for _, name := range d.ResponseColumns {
			if _, ok := d.Field(name); !ok {
				return fmt.Errorf("ResponseColumns 引用未声明字段 %q", name)
			}
		}
	}
	for _, f := range d.Fields {
		if f.Default != nil && !defaultMatchesType(f.Default, f.Type) {
			return fmt.Errorf("字段 %q 的 Default 类型与 FieldType 不符", f.Name)
		}
	}
	return nil
}

// Registry 描述符注册表：实体名 → 描述符（防重名、注册即校验）。
type Registry struct {
	m map[string]Descriptor
}

// NewRegistry 构造注册表；描述符非法或重名时 panic（注册期编程错误）。
func NewRegistry(descriptors ...Descriptor) *Registry {
	rg := &Registry{m: make(map[string]Descriptor, len(descriptors))}
	for _, d := range descriptors {
		if err := d.Validate(); err != nil {
			panic("dictcrud: 描述符 " + d.Name + " 非法: " + err.Error())
		}
		if _, dup := rg.m[d.Name]; dup {
			panic("dictcrud: 描述符重复注册: " + d.Name)
		}
		rg.m[d.Name] = d
	}
	return rg
}

// Get 按实体名取描述符。
func (rg *Registry) Get(name string) (Descriptor, bool) {
	d, ok := rg.m[name]
	return d, ok
}

// All 返回全部描述符（注册顺序）。
func (rg *Registry) All() []Descriptor {
	out := make([]Descriptor, 0, len(rg.m))
	for _, d := range rg.m {
		out = append(out, d)
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func defaultMatchesType(v any, t FieldType) bool {
	switch t {
	case FieldString:
		_, ok := v.(string)
		return ok
	case FieldFloat:
		_, ok := v.(float64)
		return ok
	case FieldInt:
		_, ok := v.(int)
		return ok
	case FieldBool:
		_, ok := v.(bool)
		return ok
	}
	return false
}
