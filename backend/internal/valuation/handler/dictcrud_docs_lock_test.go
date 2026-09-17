package handler

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/valuation/dictcrud"
)

// 管理端 37 条 CRUD 注解的「路径 + 方法 + 响应字段集」== AllDescriptors() 的派生集合
// （ADR-0056 §11 的锁 / issue #1100）。
//
// 此前注解是描述符之外的第二份手抄：改描述符 Path、加一个字段、换更新子集，运行期路由与
// swagger 生成物都会静默分叉，而四条既有锁（apitypes/codegen_test.go）只做「声明表 vs swagger」
// 单向校验。本文件把三方钉死：
//
//	1. 具名分派表（dictcrud_dispatch.go）↔ AllDescriptors()：声明了操作就必须有具名方法，
//	   表里不得有孤儿；注解宿主必须全部落在分派表里（死注解 = 零引用代码）；
//	2. 每条注解的 @Router 反解回 (描述符, 操作)：路径/参数名必须等于派生值；
//	3. @Success 的 object{} 字段表 ↔ dictcrud.ResponseFields：逐字段（名 + 类型）全等。
//
// 判定面是**源码文本**（dictcrud_docs.go 的注释）—— swag 只认函数级注释，注解无法在运行期
// 派生，故这里用「派生期望 + 全等断言」把「手抄」变成可执行判据。
//
// 实测发现（#1100）：37 条注解里有 **6 条是幻影** —— 描述符从未声明 Update、路由从未注册，
// 但注解与 apitypes 域声明表都写了 PUT（swagger 随之撒谎）。逐条登记在 phantomAnnotations，
// 本票不做行为变更（见该表注释）。

// adminSwaggerPrefix 注解侧路径前缀 = 路由组前缀去掉 swag 的 /api（swag paths 不含 basePath）。
func adminSwaggerPrefix() string {
	return strings.TrimPrefix(valuationAdminGroupPath, "/api")
}

// phantomAnnotations 描述符未声明该操作、路由从未注册，但注解与域声明表都存在的幻影注解
// （键 = "<描述符名>/<HTTP 方法>"）。
//
// 证据（#1100 实测）：
//   - descriptors.go 的 6 个规格族描述符（tonnages / mast_types / mast_heights / battery_types /
//     transmission_types / engine_types）只声明 Create + Delete —— 文件头口径「单字段唯一列 + C/D」；
//   - specs_crud_contract_test.go 的 TestSpecsCrud_NoPutRoute 逐条断言这 6 条 PUT 必须是 404；
//   - 于是「注解/swagger/域声明表」比运行期多 6 条 PUT（前端 valuation/admin.ts 的
//     createCrud('tonnages').update() 一类调用打的是不存在的路由）。
//
// 处置归另一票（二选一：删注解与域表条目 + 前端收口；或给描述符补 Update、同步契约测试与前端）。
// 本表的作用是让「幻影」不可能悄悄多出来：条目必须仍然是「未声明」状态，一旦描述符补上 Update，
// 本锁会要求把它移出本表并登记进分派表。
var phantomAnnotations = map[string]string{
	"tonnages/put":           "规格族描述符无 Update；specs 契约测试断言 PUT tonnages/{id} = 404（证据见上）",
	"mast_types/put":         "规格族描述符无 Update；specs 契约测试断言 PUT mast-types/{id} = 404",
	"mast_heights/put":       "规格族描述符无 Update；specs 契约测试断言 PUT mast-heights/{id} = 404",
	"battery_types/put":      "规格族描述符无 Update；specs 契约测试断言 PUT battery-types/{id} = 404",
	"transmission_types/put": "规格族描述符无 Update；specs 契约测试断言 PUT transmission-types/{id} = 404",
	"engine_types/put":       "规格族描述符无 Update；specs 契约测试断言 PUT engine-types/{id} = 404",
}

// dictOpFunc 取一个描述符在某操作上的具名方法（nil = 未声明/未登记）。
func dictOpFunc(routes dictRoute, op dictcrud.Op) gin.HandlerFunc {
	switch op {
	case dictcrud.OpCreate:
		return routes.create
	case dictcrud.OpUpdate:
		return routes.update
	case dictcrud.OpDelete:
		return routes.delete
	}
	return nil
}

// dictDeclaresOp 描述符是否声明该操作（= 路由注册条件，见 registerDictCRUDRoutes）。
func dictDeclaresOp(d dictcrud.Descriptor, op dictcrud.Op) bool {
	switch op {
	case dictcrud.OpCreate:
		return len(d.Create.Fields) > 0
	case dictcrud.OpUpdate:
		return len(d.Update.Fields) > 0
	case dictcrud.OpDelete:
		return d.Delete
	}
	return false
}

// dictOps 三个操作（遍历顺序固定）。
func dictOps() []dictcrud.Op {
	return []dictcrud.Op{dictcrud.OpCreate, dictcrud.OpUpdate, dictcrud.OpDelete}
}

// funcName 取方法值的具名方法名（方法值是编译器生成的 -fm 包装，去掉后缀）。
func funcName(fn gin.HandlerFunc) string {
	if fn == nil {
		return ""
	}
	f := runtime.FuncForPC(reflect.ValueOf(fn).Pointer())
	if f == nil {
		return ""
	}
	name := f.Name()
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	return strings.TrimSuffix(name, "-fm")
}

// docAnnotation 一条注解里本锁关心的事实。
type docAnnotation struct {
	// method 注解宿主方法名（= 函数名）。
	method string
	// router 规范形态："POST /valuation/admin/brands"。
	router string
	// success @Success 200 的 data=object{...} 字段表（声明序）。
	success []dictcrud.ResponseField
}

// loadDictDocAnnotations 解析 dictcrud_docs.go 的函数级注释（go test 的 CWD = 包目录）。
func loadDictDocAnnotations(t *testing.T) map[string]docAnnotation {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "dictcrud_docs.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("解析 dictcrud_docs.go 失败: %v", err)
	}
	out := map[string]docAnnotation{}
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Doc == nil {
			continue
		}
		text := fd.Doc.Text()
		if !strings.Contains(text, "@Router ") {
			continue // 非路由注解块（descriptorByName 等辅助函数）
		}
		ann, err := parseDocAnnotation(text)
		if err != nil {
			t.Fatalf("%s 的注解不可解析: %v", fd.Name.Name, err)
		}
		ann.method = fd.Name.Name
		out[fd.Name.Name] = ann
	}
	if len(out) == 0 {
		t.Fatal("dictcrud_docs.go 里没有解析出任何 @Router 注解（判据坏了）")
	}
	return out
}

// parseDocAnnotation 从注释文本（已去掉 // 前缀）提 @Router 与 @Success 的 data 字段表。
func parseDocAnnotation(text string) (docAnnotation, error) {
	var out docAnnotation
	successSeen := false
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "@Router "):
			fields := strings.Fields(strings.TrimPrefix(line, "@Router "))
			if len(fields) != 2 || !strings.HasPrefix(fields[1], "[") {
				return out, fmt.Errorf("@Router 形态异常: %s", line)
			}
			out.router = strings.ToUpper(strings.Trim(fields[1], "[]")) + " " + fields[0]
		case strings.HasPrefix(line, "@Success "):
			if successSeen {
				return out, fmt.Errorf("出现多条 @Success（本锁只认成功响应的 data 字段表）: %s", line)
			}
			successSeen = true
			fields, err := parseSuccessFields(line)
			if err != nil {
				return out, err
			}
			out.success = fields
		}
	}
	if out.router == "" {
		return out, fmt.Errorf("缺 @Router")
	}
	if !successSeen {
		return out, fmt.Errorf("缺 @Success")
	}
	return out, nil
}

// parseSuccessFields 取 "@Success 200 {object} response.R{data=object{name=type,...}} ..." 的字段表。
func parseSuccessFields(line string) ([]dictcrud.ResponseField, error) {
	const marker = "data=object{"
	i := strings.Index(line, marker)
	if i < 0 {
		return nil, fmt.Errorf("缺 data=object{...} 字段表（本票要求字段表由描述符派生）: %s", line)
	}
	rest := line[i+len(marker):]
	depth := 1
	end := -1
	for j, r := range rest {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = j
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return nil, fmt.Errorf("object{} 括号不配平: %s", line)
	}
	body := rest[:end]
	if strings.TrimSpace(body) == "" {
		return []dictcrud.ResponseField{}, nil
	}
	parts := strings.Split(body, ",")
	out := make([]dictcrud.ResponseField, 0, len(parts))
	for _, p := range parts {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			return nil, fmt.Errorf("字段形态异常: %s", p)
		}
		out = append(out, dictcrud.ResponseField{Name: kv[0], Type: kv[1]})
	}
	return out, nil
}

// resolveAnnotation 把注解的 "METHOD /valuation/admin/<path>" 反解回 (描述符, 操作)：
// 必须逐字等于某个描述符某个操作的派生路径（含 :param / {param} 名）。
func resolveAnnotation(descriptors []dictcrud.Descriptor, router string) (dictcrud.Descriptor, dictcrud.Op, bool) {
	for _, d := range descriptors {
		for _, op := range dictOps() {
			want := op.Method() + " " + adminSwaggerPrefix() + "/" + dictcrud.SwaggerPath(d, op)
			if router == want {
				return d, op, true
			}
		}
	}
	return dictcrud.Descriptor{}, 0, false
}

// phantomKey 幻影表的键。
func phantomKey(d dictcrud.Descriptor, op dictcrud.Op) string {
	return d.Name + "/" + strings.ToLower(op.Method())
}

// TestDictDispatchMatchesDescriptors 分派表 ↔ AllDescriptors()：声明了操作就必须有具名方法
// （注册期 panic 的同一判据），表里不得有孤儿。
func TestDictDispatchMatchesDescriptors(t *testing.T) {
	table := (&ConfigHandler{}).dictDispatch()
	descriptors := dictcrud.AllDescriptors()
	if len(table) != len(descriptors) {
		t.Fatalf("分派表条目 %d 条，AllDescriptors() %d 条 —— 必须一一对应", len(table), len(descriptors))
	}
	registered := 0
	for _, d := range descriptors {
		routes, ok := table[d.Name]
		if !ok {
			t.Fatalf("描述符 %q 在具名分派表里没有条目（dictcrud_dispatch.go 漏登记）", d.Name)
		}
		for _, op := range dictOps() {
			declared := dictDeclaresOp(d, op)
			fn := dictOpFunc(routes, op)
			if declared != (fn != nil) {
				t.Fatalf("描述符 %q 的 %s：描述符声明=%v，分派表具名方法=%v —— 二者必须一致",
					d.Name, op.Method(), declared, fn != nil)
			}
			if declared {
				registered++
			}
		}
	}
	// 13 个描述符：12 create + 7 update + 12 delete = 31（规格族 6 个无 Update）。
	if registered != 31 {
		t.Fatalf("分派表登记的路由数 = %d，期望 31（12 create + 7 update + 12 delete）", registered)
	}
	known := map[string]bool{}
	for _, d := range descriptors {
		known[d.Name] = true
	}
	for name := range table {
		if !known[name] {
			t.Fatalf("分派表登记了未声明的描述符 %q", name)
		}
	}
}

// TestDictAnnotationsMatchDescriptors 本票的主锁：37 条注解 == AllDescriptors() 派生集合。
func TestDictAnnotationsMatchDescriptors(t *testing.T) {
	docs := loadDictDocAnnotations(t)
	table := (&ConfigHandler{}).dictDispatch()
	descriptors := dictcrud.AllDescriptors()

	// 1) 执行体方向：分派表里每个具名方法都必须有注解块（注解面不许漏）。
	for _, d := range descriptors {
		for _, op := range dictOps() {
			if !dictDeclaresOp(d, op) {
				continue
			}
			host := funcName(dictOpFunc(table[d.Name], op))
			if _, ok := docs[host]; !ok {
				t.Fatalf("描述符 %q 的 %s 执行体 %s 在 dictcrud_docs.go 里没有注解块（swagger 会漏这条路由）",
					d.Name, op.Method(), host)
			}
		}
	}

	// 2) 逐条注解：反解 (描述符, 操作) → 路径/字段表必须等于派生值；并且与分派表指向同一个宿主。
	checked := 0
	for _, ann := range docs {
		if !strings.HasPrefix(ann.router, "POST "+adminSwaggerPrefix()+"/") &&
			!strings.HasPrefix(ann.router, "PUT "+adminSwaggerPrefix()+"/") &&
			!strings.HasPrefix(ann.router, "DELETE "+adminSwaggerPrefix()+"/") {
			continue
		}
		d, op, ok := resolveAnnotation(descriptors, ann.router)
		if !ok {
			t.Fatalf("%s：@Router %q 不对应任何描述符的任何操作（路径/参数名漂移？）", ann.method, ann.router)
		}
		// 已注册的操作：注解宿主必须就是分派表里的那个具名方法（注解宿主 == 执行体），
		// 且 @Success 字段表逐字段等于描述符派生值。
		if dictDeclaresOp(d, op) {
			host := funcName(dictOpFunc(table[d.Name], op))
			if host != ann.method {
				t.Fatalf("描述符 %q 的 %s：分派表执行体 = %s，注解宿主 = %s —— 必须同一个方法",
					d.Name, op.Method(), host, ann.method)
			}
			wantFields := dictcrud.ResponseFields(d, op)
			if !reflect.DeepEqual(ann.success, wantFields) {
				t.Fatalf("%s：@Success 字段表与描述符派生不一致\n  注解   = %s\n  描述符 = %s",
					ann.method, formatFields(ann.success), formatFields(wantFields))
			}
			checked++
			continue
		}
		// 未声明的操作：只允许在本票登记的幻影表里（多一条即红）。
		// 注意：幻影操作的 @Success 字段表**没有派生源**（描述符没有该操作），故这里只断言
		// 路径/方法/参数名（上面已由 resolveAnnotation 断过）与「字段都是该描述符声明过的字段」
		// —— 前者可派生，后者是不许凭空造字段的底线；完整字段表派生要等幻影处置（见 phantomAnnotations）。
		key := phantomKey(d, op)
		if _, ok := phantomAnnotations[key]; !ok {
			t.Fatalf("%s：描述符 %q 未声明 %s（路由不会注册），但注解与域声明表都在 —— "+
				"要么登记进分派表（补描述符 + 补 Update 声明），要么删注解并开票处置",
				ann.method, d.Name, op.Method())
		}
		if len(ann.success) == 0 {
			t.Fatalf("%s：幻影注解的 @Success 没有字段表", ann.method)
		}
		for _, field := range ann.success {
			if field.Name == "id" {
				continue
			}
			if _, ok := d.Field(field.Name); !ok {
				t.Fatalf("%s：幻影注解声明了描述符未声明的字段 %q", ann.method, field.Name)
			}
		}
		checked++
	}
	if checked != 37 {
		t.Fatalf("覆盖的注解条数 = %d，期望 37", checked)
	}
	if len(phantomAnnotations) != 6 {
		t.Fatalf("幻影注解表 %d 条，期望 6（#1100 实测：6 个规格族描述符无 Update）", len(phantomAnnotations))
	}

	// 3) 反向：幻影表不得过期——描述符一旦补上 Update，条目必须移出本表。
	for key := range phantomAnnotations {
		parts := strings.SplitN(key, "/", 2)
		var d dictcrud.Descriptor
		found := false
		for _, cand := range descriptors {
			if cand.Name == parts[0] {
				d, found = cand, true
				break
			}
		}
		if !found {
			t.Fatalf("幻影表条目 %q 指向不存在的描述符", key)
		}
		if dictDeclaresOp(d, dictcrud.OpUpdate) {
			t.Fatalf("幻影表条目 %q 已过期：描述符已声明 Update —— 请登记进分派表并删掉本条目", key)
		}
	}
}

// TestDictDispatchRegistrationFailClosed 缺具名方法在注册期 panic，不静默少注册
// （route 与注解分叉的第二个入口）。
func TestDictDispatchRegistrationFailClosed(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("描述符声明了 create 但分派表没有具名方法：期望 panic")
		}
	}()
	requireDictRoute(dictcrud.BrandDescriptor, dictcrud.OpCreate, nil)
}

func formatFields(fields []dictcrud.ResponseField) string {
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		parts = append(parts, f.Name+"="+f.Type)
	}
	return "{" + strings.Join(parts, ",") + "}"
}
