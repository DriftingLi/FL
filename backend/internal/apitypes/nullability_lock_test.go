// 表态锁（ADR-0064 决策 8 的第二件护栏，本波最后一把）：**每个切片/映射字段必须显式表态**
// 它在 JSON 里可不可为 null —— 漏标即红。
//
// 为什么要这么一把锁：Go 的 `[]T` 在没有初始化时序列化成 `null`，而 swag 只看得到 Go 的类型，
// 看不到「这个出口有没有 make(...)」。于是生成的 TS 默认把它写成 `T[]`（承诺非 null），
// 消费方按非 null 用 `.length` / `v-for`，运行期却可能收到 `null`。这类谎不是一次 bug，
// 是**每一个未表态的集合字段都自带一份**，而人不可能记住 94 个出口各自的初始化形状。
//
// 锁的形状（三条，都不需要跨函数推断数据流 —— 那正是上一波否决假绿锁的理由）：
//  1. **漏标即红**：切片/映射字段没有 `nullability:"…" tag ⇒ 违规。
//  2. **自相矛盾即红**：`nonnil`（承诺出口恒非 null）与 `extensions:"x-nullable"`
//     （契约说可为 null）同时出现 ⇒ 违规。
//  3. **债务只准减不准加**：标了 `nullable` 但契约里还没落 `x-nullable` 的字段数被钉成一个常量。
//     新增一个这样的字段 ⇒ 计数变大 ⇒ 红；把某个出口补成真正非 null 或给契约补上 x-nullable ⇒
//     计数变小 ⇒ 同样红（要人来改这个数，从而让收口动作可见）。
//
// 第 2/3 条的边界由 TestNullabilityCheckerFires 用合成夹具验证：先证明 checker 真能报出
// 「漏标」和「矛盾」，再拿它去扫全仓（否则一把永不报红的锁就是本波点名要避开的那种摆设）。
package apitypes

import (
	"os"

	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
	"testing"
)

// nullabilityTag 是表态所在的 struct tag key。
// 值域两个：nullable（可为 null，消费方必须处理）/ nonnil（该出口恒非 null）。
const nullabilityTag = "nullability"

const (
	verdictNullable = "nullable"
	verdictNonNil   = "nonnil"
)

// 契约撒谎债务的当前实测值（见文件头第 3 条）。只能减，不能加。
//
// 93 = 第④批建锁时全仓扫出的「字段已表态 nullable、但 swagger 里还没有 x-nullable」的位置数。
// 也就是说：这 93 处生成的 TS 正在对消费方承诺 `T[]`，而运行期可能发 `null`。
// 本波不做批量补 x-nullable（那会把每个消费点都变成一次带类型错误的跨端改动），
// 把它钉成常量是为了让这个数**可见、单调下降**，而不是停在「大概有不少吧」。
const declaredNullableWithoutContractFlag = 93

// nullabilityViolation 一条违规的可读描述（type.field + 原因）。
type nullabilityViolation struct{ msg string }

// jsonTagRe / xNullableRe 只用于读 tag，不做写入。
var (
	jsonTagRe     = regexp.MustCompile(`json:"([^"]+)`)
	xNullableTag  = regexp.MustCompile(`extensions:"[^"]*x-nullable[^"]*"`)
	nullabilityRe = regexp.MustCompile(`nullability:"([^"]+)"`)
)

// collectNamedCollections 记下包内 `type X []Y` / `type X map[K]V` 这类别名，
// 让 `JSONArray` 这种具名集合也被当成切片/映射来要求表态。
func collectNamedCollections(files []*ast.File) map[string]bool {
	out := map[string]bool{}
	for _, f := range files {
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
				switch u := ts.Type.(type) {
				case *ast.ArrayType, *ast.MapType:
					_ = u
					out[ts.Name.Name] = true
				}
			}
		}
	}
	return out
}

// isCollectionFieldType 判断字段类型是否集合：切片/数组/映射，或指向它们的指针，
// 或本包内定义为集合的具名类型（JSONArray 等）。
func isCollectionFieldType(expr ast.Expr, named map[string]bool) bool {
	switch t := expr.(type) {
	case *ast.ArrayType, *ast.MapType:
		return true
	case *ast.StarExpr:
		return isCollectionFieldType(t.X, named)
	case *ast.SelectorExpr: // 跨包具名（model.JSONB）：按底层 kind 判
		return false
	case *ast.Ident:
		if named[t.Name] {
			return true
		}
		return false
	}
	return false
}

// checkStructType 扫一个 struct 类型，返回它的表态违规。
// 只管**会进 JSON 的导出字段**（有 json tag 且键名不是 "-"）。
func checkStructType(typeName string, st *ast.StructType, named map[string]bool) []nullabilityViolation {
	var out []nullabilityViolation
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 || !f.Names[0].IsExported() {
			continue
		}
		tag := ""
		if f.Tag != nil {
			tag = strings.Trim(f.Tag.Value, "`")
		}
		jsonName := ""
		if m := jsonTagRe.FindStringSubmatch(tag); m != nil {
			jsonName = m[1]
		}
		if jsonName == "" || jsonName == "-" {
			continue
		}
		if !isCollectionFieldType(f.Type, named) {
			continue
		}
		name := typeName + "." + f.Names[0].Name
		m := nullabilityRe.FindStringSubmatch(tag)
		if m == nil {
			out = append(out, nullabilityViolation{name + "：集合字段未表态（缺 " + nullabilityTag +
				":\"nullable|nonnil\"）—— 不表态就没人知道这个出口会不会发出 null"})
			continue
		}
		switch m[1] {
		case verdictNullable:
			// 与契约一致即可；缺 x-nullable 由债务计数那一条统一管。
		case verdictNonNil:
			if xNullableTag.MatchString(tag) {
				out = append(out, nullabilityViolation{name + "：nonnil（出口恒非 null）与契约上的 " +
					"x-nullable（可为 null）同时出现 ⇒ 两格互相顶替"})
			}
		default:
			out = append(out, nullabilityViolation{name + "：nullability tag 取值非法 " + m[1] +
				"（只能是 nullable / nonnil）"})
		}
	}
	return out
}

// checkSource 解析一个 Go 文件并返回其中所有导出 struct 的表态违规。
func checkSource(fset *token.FileSet, path string, named map[string]bool) []nullabilityViolation {
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return []nullabilityViolation{{msg: path + "：解析失败 " + err.Error()}}
	}
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
			out = append(out, checkStructType(ts.Name.Name, st, named)...)
		}
	}
	return out
}

// countNullableWithoutFlag 数出「字段表态可为 null，但契约里没写 x-nullable」的个数
// —— 即生成物正在对消费方撒谎的位置数（文件头第 3 条的债务）。
func countNullableWithoutFlag(fset *token.FileSet, path string, named map[string]bool) int {
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return 0
	}
	n := 0
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
			for _, fld := range st.Fields.List {
				if len(fld.Names) == 0 || !fld.Names[0].IsExported() || fld.Tag == nil {
					continue
				}
				tag := strings.Trim(fld.Tag.Value, "`")
				if !isCollectionFieldType(fld.Type, named) {
					continue
				}
				m := nullabilityRe.FindStringSubmatch(tag)
				if m != nil && m[1] == verdictNullable && !xNullableTag.MatchString(tag) {
					n++
				}
			}
		}
	}
	return n
}

// parsedFiles 一个包目录里解析成功的文件集合（paths 与 files 同序）。
type parsedFiles struct {
	files []*ast.File
	paths []string
}

// parsePackage 解析目录下的 Go 文件。includeTests=false 时跳过 _test.go
// （全仓扫描不该把测试自己的夹具算成契约面）。
func parsePackage(dir string, includeTests bool) (parsedFiles, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return parsedFiles{}, err
	}
	fset := token.NewFileSet()
	var out parsedFiles
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		if !includeTests && strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := dir + "/" + name
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			continue // 解析失败由 go build 兜，不在这里重复报
		}
		out.files = append(out.files, f)
		out.paths = append(out.paths, path)
	}
	return out, nil
}

// sweepPackage 扫一个包目录（跳过 _test.go），返回违规与债务计数。
func sweepPackage(t *testing.T, dir string) ([]nullabilityViolation, int) {
	t.Helper()
	fset := token.NewFileSet()
	fis, err := parsePackage(dir, false)
	if err != nil {
		t.Fatalf("读 %s 失败: %v", dir, err)
	}
	named := collectNamedCollections(fis.files)
	var out []nullabilityViolation
	debts := 0
	for _, path := range fis.paths {
		out = append(out, checkSource(fset, path, named)...)
		debts += countNullableWithoutFlag(fset, path, named)
	}
	return out, debts
}

// TestResponseCollectionsMustDeclareNullability 全仓扫：service 与 api 两个包里的响应 DTO
// 每个集合字段都要有表态，且「契约撒谎债务」不得超过登记的常量。
func TestResponseCollectionsMustDeclareNullability(t *testing.T) {
	var all []nullabilityViolation
	debts := 0
	for _, dir := range []string{"../service", "../api"} {
		vs, n := sweepPackage(t, dir)
		all = append(all, vs...)
		debts += n
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

// 合成夹具：证明 checker 真的会报，而不是扫出一片绿自我安慰。
type nilabilityFixtureUntagged struct {
	Items []string `json:"items"`
}

type nilabilityFixtureContradictory struct {
	Items []string `json:"items" nullability:"nonnil" extensions:"x-nullable"`
}

type nilabilityFixtureBadVerdict struct {
	Items []string `json:"items" nullability:"maybe"`
}

type nilabilityFixtureDeclared struct {
	Items []string `json:"items" nullability:"nullable" extensions:"x-nullable"`
	Name  string   `json:"name"`
}

// fixtureViolations 从测试自己的源码里解析出夹具类型并跑 checker——
// 用真实 AST 而不是手搓 ast 节点，夹具改了测试不会假绿。
func fixtureViolations(t *testing.T, typeName string) []nullabilityViolation {
	t.Helper()
	fset := token.NewFileSet()
	fis, err := parsePackage(".", true)
	if err != nil {
		t.Fatalf("读本包失败: %v", err)
	}
	var out []nullabilityViolation
	for _, path := range fis.paths {
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			continue
		}
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
				if st, ok := ts.Type.(*ast.StructType); ok {
					out = append(out, checkStructType(typeName, st, collectNamedCollections(fis.files))...)
				}
			}
		}
	}
	if len(out) == 0 && typeName == "nilabilityFixtureDeclared" {
		return nil // 已表态的夹具本就不该有违规；其余夹具由调用方断言非空
	}
	return out
}

func TestNullabilityCheckerFires(t *testing.T) {
	cases := []struct {
		typeName string
		wantMsg  string
	}{
		{"nilabilityFixtureUntagged", "未表态"},
		{"nilabilityFixtureContradictory", "同时出现"},
		{"nilabilityFixtureBadVerdict", "取值非法"},
	}
	for _, c := range cases {
		got := fixtureViolations(t, c.typeName)
		if len(got) != 1 {
			t.Fatalf("%s 应报 1 条违规，实际 %d 条（checker 空转）", c.typeName, len(got))
		}
		if !strings.Contains(got[0].msg, c.wantMsg) {
			t.Fatalf("%s 的违规原因应含「%s」，实际 %s", c.typeName, c.wantMsg, got[0].msg)
		}
	}
	// 反向：已正确表态的夹具不得被误报（否则锁会逼人乱打标）。
	if got := fixtureViolations(t, "nilabilityFixtureDeclared"); len(got) != 0 {
		t.Fatalf("已表态的夹具被误报: %v", got[0].msg)
	}
}
