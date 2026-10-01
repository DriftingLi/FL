// 静态守卫（ADR-0056 §3 / #1097）：api 层不得出现 *gorm.DB —— 唯一白名单是装配根 deps.go。
//
// 为什么是 Go 测试而不是 scripts/ 守卫：本票明令不新增 CI 步骤、不动 scripts/；且这是包内 seam
// 断言（api 只能经 service 读库），与包同生命周期。扫描面是 AST 而非文本 grep —— 注释里提到
// *gorm.DB 不误报，gorm.ErrRecordNotFound（api 层合法的错误判定）不误报。
//
// 测试文件不在扫描面内：用例经 testutil.NewMemoryDB 造库、RouterDeps.DB 装配路由，属测试脚手架，
// 不是 API 的数据访问面。
package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"testing"

	"forklift-training/internal/testutil"
)

// gormDBWhitelist 允许出现 *gorm.DB 的文件（**模块根相对路径** → 理由；逐条登记，防「顺手放宽」）。
// 键从文件名改成路径：拆包后同名文件可能落在新目录，文件名不再唯一指认。
var gormDBWhitelist = map[string]string{
	"internal/api/deps.go": "装配根：RouterDeps.DB / Deps.DB 字段与 NewDeps 入参",
}

// scanGormDBRefs 返回源码中 *gorm.DB 类型引用的行号（AST：只认类型表达式）。
func scanGormDBRefs(t *testing.T, filename, src string) []int {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		t.Fatalf("解析 %s 失败: %v", filename, err)
	}
	var lines []int
	ast.Inspect(file, func(n ast.Node) bool {
		star, ok := n.(*ast.StarExpr)
		if !ok {
			return true
		}
		sel, ok := star.X.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "gorm" || sel.Sel.Name != "DB" {
			return true
		}
		lines = append(lines, fset.Position(star.Pos()).Line)
		return true
	})
	return lines
}

// TestAPILayerHasNoGormDB 守卫：HTTP 面的非测试源码里 *gorm.DB 只允许出现在白名单文件内。
// 射程由 testutil.HTTPSurface 定义（拆包后各域包的端点/路由文件自动进射程），不再假定
// 「本测试文件所在目录就是全部射程」。
func TestAPILayerHasNoGormDB(t *testing.T) {
	t.Parallel()
	var offenders []string
	for _, f := range testutil.ScanBackendCode(t) {
		if !testutil.HTTPSurface(f) {
			continue
		}
		if _, ok := gormDBWhitelist[f.Path]; ok {
			continue
		}
		for _, line := range scanGormDBRefs(t, f.Name, f.Src) {
			offenders = append(offenders, f.Path+":"+strconv.Itoa(line))
		}
	}
	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Fatalf("HTTP 面不得持 *gorm.DB（ADR-0056 §3）：读库只能经 service；违例 %v", offenders)
	}
}

// TestGormDBWhitelistIsLive 白名单必须仍然有理由：deps.go 不再持 *gorm.DB 时就该删掉这一条，
// 否则守卫会悄悄放宽（白名单腐化）。
func TestGormDBWhitelistIsLive(t *testing.T) {
	t.Parallel()
	files := testutil.ScanBackendCode(t)
	for path, reason := range gormDBWhitelist {
		if reason == "" {
			t.Errorf("白名单 %s 缺理由", path)
		}
		f := testutil.FindCode(t, files, path)
		if refs := scanGormDBRefs(t, f.Name, f.Src); len(refs) == 0 {
			t.Errorf("白名单 %s 已不再出现 *gorm.DB，应从 gormDBWhitelist 移除", path)
		}
	}
}

// TestGormDBGuardDetectsOffender 判定面自测：守卫不是空转 —— 合成违例必须被报出，
// 合法的 gorm 错误判定（非 *gorm.DB 类型）不得误报。
func TestGormDBGuardDetectsOffender(t *testing.T) {
	t.Parallel()
	offender := `package api

import "gorm.io/gorm"

func register(db *gorm.DB) {}
`
	if refs := scanGormDBRefs(t, "synthetic.go", offender); len(refs) != 1 {
		t.Fatalf("合成违例未被报出（守卫空转）：refs = %v", refs)
	}
	legal := `package api

import (
	"errors"

	"gorm.io/gorm"

)

func isMissing(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
`
	if refs := scanGormDBRefs(t, "synthetic_legal.go", legal); len(refs) != 0 {
		t.Fatalf("gorm 错误判定被误报为 *gorm.DB：refs = %v", refs)
	}
}
