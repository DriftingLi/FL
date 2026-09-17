// 描述符 → 注解响应字段表的派生单点（ADR-0056 §11 / issue #1100）。
//
// 管理端写面「路径 + 方法 + 响应字段集」此前有三份手抄：描述符、dictcrud_docs.go 的 37 条
// @Success 内联字段表、apitypes/domains.go 的端点清单。本文件把注解那份变成描述符的**纯函数派生**——
// 响应构造规则（builder.go 的 buildResult / BuildUpdateKeySQL 的 RETURNING）与注解字段表同源，
// 漂移由 handler 侧的全等锁测试（dictcrud_docs_lock_test.go）当场变红，运行期路由与 swagger
// 生成物不会再静默分叉。
//
// 注：响应体在运行期是 map[string]any（encoding/json 按字典序输出键），故这里的顺序只服务
// 注解与生成物的可读性——它必须逐字等于既有注解顺序，改顺序就是改生成物 diff。
package dictcrud

// Op 描述符的一条写路由（注册条件见 handler 侧 registerDictCRUDRoutes）。
type Op int

const (
	// OpCreate POST /<Path>（Create.Fields 非空时注册）。
	OpCreate Op = iota
	// OpUpdate PUT /<Path>/:<param>（Update.Fields 非空时注册；param = UpdateKeyField 或 id）。
	OpUpdate
	// OpDelete DELETE /<Path>/:id（Delete 为真时注册）。
	OpDelete
)

// Method 该操作注册的 HTTP 方法（= 注解 @Router 的动词）。
func (o Op) Method() string {
	switch o {
	case OpCreate:
		return "POST"
	case OpUpdate:
		return "PUT"
	case OpDelete:
		return "DELETE"
	}
	return ""
}

// RouteParam 该操作的路由参数名：create 无参数（返回空串）；
// update 取 UpdateKeyField（缺省 id）；delete 恒为 id。
func RouteParam(d Descriptor, op Op) string {
	switch op {
	case OpUpdate:
		if d.UpdateKeyField != "" {
			return d.UpdateKeyField
		}
		return "id"
	case OpDelete:
		return "id"
	}
	return ""
}

// RoutePath 该操作在管理端组内的路由段，gin 形态（:id / :key）：create → <Path>；update/delete → <Path>/:<param>。
// 路由注册用这一份；注解形态见 SwaggerPath。
func RoutePath(d Descriptor, op Op) string {
	if p := RouteParam(d, op); p != "" {
		return d.Path + "/:" + p
	}
	return d.Path
}

// SwaggerPath 同一路由的注解/生成物形态（{id} / {key}）：与 RoutePath 共用同一个参数名派生，
// 故「描述符 Path / UpdateKeyField 改了而注解没改」会被锁测试抓住。
func SwaggerPath(d Descriptor, op Op) string {
	if p := RouteParam(d, op); p != "" {
		return d.Path + "/{" + p + "}"
	}
	return d.Path
}

// ResponseField 响应里的一个字段：JSON 名 + swag 内联 object{} 的类型名。
type ResponseField struct {
	// Name JSON 字段名（body 与响应共用）。
	Name string
	// Type swag 类型名（integer / number / string / boolean），由 FieldType 派生。
	Type string
}

// ResponseFields 由描述符派生一次写操作的响应字段表（顺序 = 既有注解顺序）：
//
//	create → {id} ∪ Create.Fields ∪ ResponseExtra
//	update → ResponseReturning 时 {id} ∪ ResponseColumns；否则 {id} ∪ Update.Fields ∪ ResponseExtra
//	delete → {id}
//
// 与 builder.go 的响应构造一一对应（buildResult / ResponseScan 的 id 前置 + ResponseColumns）。
func ResponseFields(d Descriptor, op Op) []ResponseField {
	out := []ResponseField{{Name: "id", Type: swaggerTypeOf(FieldInt)}}
	switch op {
	case OpCreate:
		out = append(out, swaggerFields(d, d.Create.Fields)...)
	case OpUpdate:
		if d.ResponseReturning {
			return append(out, swaggerFields(d, d.ResponseColumns)...)
		}
		out = append(out, swaggerFields(d, d.Update.Fields)...)
	case OpDelete:
		return out
	}
	return append(out, swaggerFields(d, d.ResponseExtra)...)
}

// swaggerFields 按名取描述符字段并映射为注解字段（未声明的名字跳过，与响应构造的 if ok 一致）。
func swaggerFields(d Descriptor, names []string) []ResponseField {
	out := make([]ResponseField, 0, len(names))
	for _, name := range names {
		if f, ok := d.Field(name); ok {
			out = append(out, ResponseField{Name: f.Name, Type: swaggerTypeOf(f.Type)})
		}
	}
	return out
}

// swaggerTypeOf FieldType → swag 内联 object{} 的类型名。
func swaggerTypeOf(t FieldType) string {
	switch t {
	case FieldString:
		return "string"
	case FieldFloat:
		return "number"
	case FieldInt:
		return "integer"
	case FieldBool:
		return "boolean"
	}
	return ""
}
