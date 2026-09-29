// 静态守卫（ADR-0064 决策 9 的执行面）：**5xx 文案不得由 err.Error() 拼出来**。
//
// 为什么是 Go 测试而不是 scripts/ 守卫：与 gorm_seam_guard_test.go 同因（包内 seam 断言、
// 不新增 CI 步骤），且判据是 AST 而非文本 grep —— 注释里提到旧形状不误报。
//
// 判据形状：调用 response.ServerError(c, X) 且 X 的表达式树里出现无参 <x>.Error() 调用 ⇒ 红。
// 正解是 response.ServerErrorCause(c, "前缀: ", err)（规则本体在 pkg/response.ClientErrorText：
// 保留前缀、丢掉尾巴，并 c.Error 记账）；显式固定文案（response.ServerError(c, "服务器内部错误")）
// 仍然允许 —— 「说什么」是显式决定，禁的是 err.Error() 的默认漏出。
//
// 扫描面：internal/ + pkg/ 的**非测试**源码。本批实测 20 处（internal/api 18 +
// internal/valuation/handler 2）已全部改用 ServerErrorCause，存量清零、零豁免。
package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// scanServerErrorLeaks 扫一份源码，返回「5xx 文案由 err.Error() 拼出」的行号。
func scanServerErrorLeaks(t *testing.T, filename, src string) []int {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		t.Fatalf("解析 %s 失败: %v", filename, err)
	}
	var lines []int
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "ServerError" {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "response" {
			return true
		}
		if containsErrErrorCall(call.Args[1]) {
			lines = append(lines, fset.Position(call.Pos()).Line)
		}
		return true
	})
	return lines
}

// containsErrErrorCall 子树里是否有无参 <x>.Error() 调用（err.Error() / errors.Unwrap(err).Error() 都算）。
func containsErrErrorCall(node ast.Node) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == "Error" && len(call.Args) == 0 {
			found = true
		}
		return true
	})
	return found
}

// TestServerErrorTextGuard 全仓 5xx 文案不得拼 err.Error()（存量 0，零豁免）。
func TestServerErrorTextGuard(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	scanRoots := []string{filepath.Join(root, "internal"), filepath.Join(root, "pkg")}
	var offenders []string
	for _, scanRoot := range scanRoots {
		err := filepath.WalkDir(scanRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := d.Name()
			if d.IsDir() {
				if name == "testdata" || name == "node_modules" {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				return nil
			}
			src, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			rel, _ := filepath.Rel(root, path)
			for _, line := range scanServerErrorLeaks(t, name, string(src)) {
				offenders = append(offenders, filepath.ToSlash(rel)+":"+strconv.Itoa(line))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("扫描 %s 失败: %v", scanRoot, err)
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("5xx 文案拼了 err.Error()（ADR-0064 决策 9：驱动原文不得外发），改用 response.ServerErrorCause：\n  %s",
			strings.Join(offenders, "\n  "))
	}
}

// TestServerErrorTextGuardSelfCheck 判据本身有效：旧形状必红（裸的与带前缀的两种），正解不红。
func TestServerErrorTextGuardSelfCheck(t *testing.T) {
	t.Parallel()
	stale := `package x

func a(c *gin.Context, err error) {
	response.ServerError(c, err.Error())
	response.ServerError(c, "导出失败: "+err.Error())
}
`
	if got := scanServerErrorLeaks(t, "stale.go", stale); len(got) != 2 {
		t.Fatalf("旧形状应两处命中，实得 %v", got)
	}
	fixed := `package x

func a(c *gin.Context, err error) {
	response.ServerErrorCause(c, "导出失败: ", err)
	response.ServerError(c, "服务器内部错误")
	log.Error("x", zap.Error(err))
}
`
	if got := scanServerErrorLeaks(t, "fixed.go", fixed); len(got) != 0 {
		t.Fatalf("正解与固定文案不应命中，实得 %v", got)
	}
}
