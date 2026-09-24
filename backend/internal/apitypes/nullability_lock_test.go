// 表态锁（ADR-0064 决策 8 的第二件护栏，本波最后一把）：**响应里每一个切片/映射字段都必须
// 显式表态**它在 JSON 里可不可为 null —— 漏标即红。
//
// 为什么要这么一把锁：Go 的 `[]T` 在没有初始化时序列化成 `null`，而 swag 只看得到 Go 的类型，
// 看不到「这个出口有没有 make(...)」。于是生成的 TS 默认把它写成 `T[]`（承诺非 null），
// 消费方按非 null 用 `.length` / `v-for`，运行期却可能收到 `null`。这类谎不需要写错代码，
// 只需要**没人表态**，而人不可能记住几十个出口各自的初始化形状。
//
// 射程按 spec 的说法只管**响应**：范围取自 swagger 里所有 2xx 响应的类型闭包
// （service./api./model./repository. 都算，因为消费方看到的就是这份闭包）。
// 请求体上的同名字段**不在射程内**——入参的「没给」与「给了空数组」是另一件事，
// 用同一个 tag 表态会把两种语义混成一格。
// 也不靠类型名猜（「以 DTO 结尾」那种判据会漏会误伤）。
//
// 三条判据，都不需要跨函数推断数据流（那正是上一波否决假绿锁的理由）：
//  1. **漏标即红**：闭包内的集合字段没有 `nullability:"…"` tag。
//  2. **自相矛盾即红**：`nonnil`（承诺出口恒非 null）与契约上的 `x-nullable`（说可为 null）同时出现。
//  3. **债务只准减不准加**：标了 `nullable` 而契约里还没落 `x-nullable` 的位置数被钉成常量。
//     新增 ⇒ 计数变大 ⇒ 红；真的修掉了 ⇒ 计数变小 ⇒ 也红（要人来改这个常量，
//     于是收口动作留下痕迹，而不是悄悄漂过去）。
//
// 锁自己先被验：TestNullabilityCheckerFires 用合成夹具证明三条判据真的会报、且不误报，
// 再拿同一个 checker 去扫全仓——一把永不报红的锁就是本波点名要避开的那种摆设。
package apitypes

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// nullabilityTag 是表态所在的 struct tag key；值域两个（可为 null / 出口恒非 null）。
const (
	nullabilityTag  = "nullability"
	verdictNullable = "nullable"
	verdictNonNil   = "nonnil"
)

// declaredNullableWithoutContractFlag 契约撒谎债务的实测值（文件头第 3 条）：
// 91 处响应集合字段自己承认「可能为 null」，而 swagger/TS 还在对消费方承诺非 null。
// （104 处表态 = 99 nullable + 5 nonnil；其中 8 处早已带 x-nullable，故债务是 91 而不是 99。）
// 本波不做批量补 x-nullable（那会把每个消费点变成一次带类型错误的跨端改动）——
// 这笔账交给下一波，常量的作用是让它只能变短。
const declaredNullableWithoutContractFlag = 91

// sweptDirs 扫哪些包目录。键 = 包名（swagger 的 definition 名前缀），值 = 目录。
// 加新包就加一行；不在表里的包里的 DTO 不会被扫（因此也不会被误报）。
var sweptDirs = map[string]string{
	"service":    "../service",
	"api":        "../api",
	"model":      "../model",
	"repository": "../valuation/repository",
}

var (
	jsonTagRe     = regexp.MustCompile(`json:"([^"]+)`)
	xNullableTag  = regexp.MustCompile(`extensions:"[^"]*x-nullable[^"]*"`)
	nullabilityRe = regexp.MustCompile(`nullability:"([^"]+)"`)
)

// nullabilityViolation 一条违规的可读描述（类型.字段 + 原因）。
type nullabilityViolation struct{ msg string }

// responseDefinitions 取 swagger 里所有 2xx 响应 schema 引用的类型闭包。
// 闭包用 codegen 自己那套（collect / collectRefs），不另写第二份 $ref 遍历。
func responseDefinitions(t *testing.T) map[string]bool {
	t.Helper()
	// 射程来自 swagger 产物；定位失败即 Fatal（codegen_test.go 的 specPath 就是这条），
	// **不 Skip**——一把因为找不到产物就静默通过的锁，等于没有锁。
	spec, err := LoadSpec(specPath(t))
	if err != nil {
		t.Fatalf("解析 swagger.json 失败: %v", err)
	}
	roots := []string{}
	for _, item := range spec.Paths {
		ops, ok := item.(map[string]any)
		if !ok {
			continue
		}
		for _, raw := range ops {
			b, err := json.Marshal(raw)
			if err != nil {
				continue
			}
			var op operation
			if err := json.Unmarshal(b, &op); err != nil {
				continue
			}
			for code, resp := range op.Responses {
				if !strings.HasPrefix(code, "2") {
					continue
				}
				refs := []string{}
				collectRefs(resp.Schema, &refs)
				roots = append(roots, refs...)
			}
		}
	}
	names, err := collect(spec, roots)
	if err != nil {
		t.Fatalf("响应类型闭包不闭合: %v", err)
	}
	out := make(map[string]bool, len(names))
	for _, n := range names {
		out[n] = true
	}
	return out
}

// namedCollections 收集「具名集合类型」（type X []Y / map[K]V），键为 包名.X，
// 这样 `JSONArray`（service 包内）与 `model.JSONB`（跨包）都能被认成集合。
func namedCollections(pkgs map[string]parsedPackage) map[string]bool {
	out := map[string]bool{}
	for pkgName, p := range pkgs {
		for _, f := range p.files {
			for _, d := range f.Decls {
				gd, ok := d.(*ast.GenDecl)
				if !ok || gd.Tok != token.TYPE {
					continue
				}
				for _, spec := range gd.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					switch ts.Type.(type) {
					case *ast.ArrayType, *ast.MapType:
						out[pkgName+"."+ts.Name.Name] = true
					}
				}
			}
		}
	}
	return out
}

// isCollectionFieldType 判断字段类型是否是集合：切片/数组/映射、指向它们的指针、
// 或（本包或跨包）定义为集合的具名类型。
func isCollectionFieldType(expr ast.Expr, curPkg string, named map[string]bool) bool {
	switch t := expr.(type) {
	case *ast.ArrayType, *ast.MapType:
		return true
	case *ast.StarExpr:
		return isCollectionFieldType(t.X, curPkg, named)
	case *ast.Ident:
		return named[curPkg+"."+t.Name]
	case *ast.SelectorExpr:
		pkgIdent, ok := t.X.(*ast.Ident)
		if !ok {
			return false
		}
		return named[pkgIdent.Name+"."+t.Sel.Name]
	}
	return false
}

// checkStructType 扫一个 struct：只问「会进 JSON 的导出集合字段」有没有诚实表态。
// inClosure=false 时整个类型跳过（它不出现在任何 2xx 响应里，就没有对消费方撒谎这件事）。
func checkStructType(typeName string, inClosure bool, st *ast.StructType, curPkg string, named map[string]bool) []nullabilityViolation {
	if !inClosure {
		return nil
	}
	var out []nullabilityViolation
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 || !f.Names[0].IsExported() || f.Tag == nil {
			continue
		}
		tag := strings.Trim(f.Tag.Value, "`")
		m := jsonTagRe.FindStringSubmatch(tag)
		if m == nil || m[1] == "" || m[1] == "-" {
			continue
		}
		if !isCollectionFieldType(f.Type, curPkg, named) {
			continue
		}
		name := typeName + "." + f.Names[0].Name
		v := nullabilityRe.FindStringSubmatch(tag)
		if v == nil {
			out = append(out, nullabilityViolation{name + "：集合字段未表态（缺 " + nullabilityTag +
				":\"nullable|nonnil\"）—— 不表态就没人知道这个出口会不会发出 null"})
			continue
		}
		switch v[1] {
		case verdictNullable:
			// 与契约是否一致，由债务计数那条统一管（文件头第 3 条）。
		case verdictNonNil:
			if xNullableTag.MatchString(tag) {
				out = append(out, nullabilityViolation{name + "：nonnil（出口恒非 null）与契约上的 " +
					"x-nullable（可为 null）同时出现 ⇒ 两格互相顶替"})
			}
		default:
			out = append(out, nullabilityViolation{name + "：nullability tag 取值非法 " + v[1] +
				"（只能是 nullable / nonnil）"})
		}
	}
	return out
}

// checkFile 扫一个已解析文件里的所有导出 struct。
func checkFile(f *ast.File, curPkg string, named map[string]bool, closure map[string]bool) []nullabilityViolation {
	var out []nullabilityViolation
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || !ts.Name.IsExported() {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			out = append(out, checkStructType(ts.Name.Name, closure[curPkg+"."+ts.Name.Name], st, curPkg, named)...)
		}
	}
	return out
}

// countNullableWithoutFlag 数「字段表态可为 null、契约里却没写 x-nullable」的位置数。
func countNullableWithoutFlag(f *ast.File, curPkg string, named map[string]bool, closure map[string]bool) int {
	n := 0
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || !ts.Name.IsExported() || !closure[curPkg+"."+ts.Name.Name] {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, fld := range st.Fields.List {
				if len(fld.Names) == 0 || !fld.Names[0].IsExported() || fld.Tag == nil {
					continue
				}
				tag := strings.Trim(fld.Tag.Value, "`")
				if !isCollectionFieldType(fld.Type, curPkg, named) {
					continue
				}
				v := nullabilityRe.FindStringSubmatch(tag)
				if v != nil && v[1] == verdictNullable && !xNullableTag.MatchString(tag) {
					n++
				}
			}
		}
	}
	return n
}

// parsedPackage 一个目录里解析成功的文件（含其测试文件，供合成夹具用）。
type parsedPackage struct {
	files []*ast.File
}

// parsePackage 解析目录下的 .go 文件；解析失败的文件直接跳过
// （编译不过的事由 go build 报，这里不重复报，也不留一条永远走不到的分支）。
func parsePackage(dir string, includeTests bool) (parsedPackage, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return parsedPackage{}, err
	}
	fset := token.NewFileSet()
	var out parsedPackage
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		if !includeTests && strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments)
		if err != nil {
			continue
		}
		out.files = append(out.files, f)
	}
	return out, nil
}

// TestResponseCollectionsMustDeclareNullability 按 swagger 响应闭包全仓扫一遍。
func TestResponseCollectionsMustDeclareNullability(t *testing.T) {
	closure := responseDefinitions(t)
	pkgs := map[string]parsedPackage{}
	for pkgName, dir := range sweptDirs {
		p, err := parsePackage(dir, false)
		if err != nil {
			t.Fatalf("读 %s 失败: %v", dir, err)
		}
		pkgs[pkgName] = p
	}
	named := namedCollections(pkgs)

	var all []nullabilityViolation
	debts := 0
	for pkgName, p := range pkgs {
		for _, f := range p.files {
			all = append(all, checkFile(f, pkgName, named, closure)...)
			debts += countNullableWithoutFlag(f, pkgName, named, closure)
		}
	}
	for _, v := range all {
		t.Errorf("表态锁: %s", v.msg)
	}
	if debts > declaredNullableWithoutContractFlag {
		t.Errorf("契约撒谎债务从 %d 涨到 %d：新加的 `nullable` 字段没有同时在契约上落 x-nullable"+
			"（生成的 TS 会继续承诺非 null）。要么把出口 make 成非 null 并标 nonnil，"+
			"要么给字段补 extensions:\"x-nullable\" 并同步消费端。",
			declaredNullableWithoutContractFlag, debts)
	}
	if debts < declaredNullableWithoutContractFlag {
		t.Errorf("债务降了（%d < %d）：这是好消息，把常量改成实测值，别让它停在虚高的数上。",
			debts, declaredNullableWithoutContractFlag)
	}
}

// 合成夹具：证明三条判据真的会报、且不误报。
//
// 这些类型只被 AST 按名字查找时，Go 的死代码检查会把它们报成 unused（CI backend-lint 实测），
// 而「按名字查找」本身也是个可被漂移的软连接。⇒ 做成一张表：类型在这里被真实引用，
// 表里的名字再用 reflect 反查核对，**类型改名与表里的字符串必须一起改**，否则红。
var nullabilityFixtures = []struct {
	fixture any
	wantMsg string // 空串 = 这组是反向例：已正确表态，不得被误报
}{
	{nullabilityFixtureUntagged{}, "未表态"},
	{nullabilityFixtureContradictory{}, "同时出现"},
	{nullabilityFixtureBadVerdict{}, "取值非法"},
	// 具名集合别名（type X []string）这一组钉的是「按底层 kind 判」那条：
	// 早先版本对跨包具名直接放过，于是 model.JSONB 那类字段成了活漏口。
	{nullabilityFixtureNamedCollection{}, "未表态"},
	{nullabilityFixtureDeclared{}, ""},
}

type nullabilityFixtureUntagged struct {
	Items []string `json:"items"`
}

type nullabilityFixtureContradictory struct {
	Items []string `json:"items" nullability:"nonnil" extensions:"x-nullable"`
}

type nullabilityFixtureBadVerdict struct {
	Items []string `json:"items" nullability:"maybe"`
}

type nullabilityFixtureDeclared struct {
	Items []string `json:"items" nullability:"nullable" extensions:"x-nullable"`
	Name  string   `json:"name"`
}

type nullabilityFixtureNamedCollection struct {
	Items nullabilityFixtureAlias `json:"items"`
}

type nullabilityFixtureAlias []string

// fixtureViolations 从测试自己的源码里解析出该类型再跑 checker——
// 用真实 AST 而不是手搓节点；类型被改名或删掉时这里 Fatalf，而不是静默当成「无违规」。
func fixtureViolations(t *testing.T, typeName string) []nullabilityViolation {
	t.Helper()
	self, err := parsePackage(".", true)
	if err != nil {
		t.Fatalf("读本包失败: %v", err)
	}
	named := map[string]bool{"apitypes.nullabilityFixtureAlias": true}
	found := false
	var out []nullabilityViolation
	for _, f := range self.files {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Name.Name != typeName {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					t.Fatalf("夹具 %s 不是 struct，夹具被改过", typeName)
				}
				found = true
				// inClosure 一律传 true：夹具验的是 checker 的判据，不是射程。
				out = append(out, checkStructType(typeName, true, st, "apitypes", named)...)
			}
		}
	}
	if !found {
		t.Fatalf("夹具类型 %s 不在本包里——被删了还是改名了？（找不到夹具时不得当作「无违规」）", typeName)
	}
	return out
}

func TestNullabilityCheckerFires(t *testing.T) {
	for _, c := range nullabilityFixtures {
		typeName := reflect.TypeOf(c.fixture).Name()
		if typeName == "" {
			t.Fatalf("夹具表里放了非具名类型：%v", c.fixture)
		}
		t.Run(typeName, func(t *testing.T) {
			got := fixtureViolations(t, typeName)
			if c.wantMsg == "" {
				if len(got) != 0 {
					t.Fatalf("已正确表态的夹具被误报: %s", got[0].msg)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("%s 应报 1 条违规，实际 %d 条（checker 空转？）", typeName, len(got))
			}
			if !strings.Contains(got[0].msg, c.wantMsg) {
				t.Fatalf("%s 的违规原因应含「%s」，实际 %s", typeName, c.wantMsg, got[0].msg)
			}
		})
	}
}
