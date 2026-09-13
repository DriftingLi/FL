package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"forklift-training/internal/authz"
)

// 角色字面量锁（ADR-0047 §1 / spec #928 决策 3-4）：端点守卫只允许用 authz 角色常量，
// 不允许再出现 RoleRequired("admin") 这类字面量——否则「角色→能力」的单一声明会被绕过，
// 而 84 处字面量正是本次收敛要消灭的东西。
//
// 用 AST 而非正则：正则会被注释与字符串内容骗过（本仓库的注释里就出现过 "admin"）。
func TestAuthzLock_NoRawRoleLiteralInGuards(t *testing.T) {
	dirs := []string{".", filepath.Join("..", "valuation", "handler")}
	fset := token.NewFileSet()
	checked := 0
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("读取目录 %s 失败: %v", dir, err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("解析 %s 失败: %v", path, err)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if !isRoleGuardCall(call.Fun) {
					return true
				}
				checked++
				for _, arg := range call.Args {
					if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						t.Errorf("%s: 角色字面量 %s 应改用 authz 角色常量（RoleRequired 的参数只能是 authz.RoleX）", path, lit.Value)
					}
				}
				return true
			})
		}
	}
	if checked == 0 {
		t.Fatal("未找到任何角色守卫调用——锁测试失效（守卫被改名或移除？）")
	}
}

// isRoleGuardCall 判定调用是否为角色/能力守卫：middleware.RoleRequired 或 RoleRequired。
func isRoleGuardCall(fun ast.Expr) bool {
	switch f := fun.(type) {
	case *ast.SelectorExpr:
		return f.Sel.Name == "RoleRequired" || f.Sel.Name == "CapabilityRequired"
	case *ast.Ident:
		return f.Name == "RoleRequired" || f.Name == "CapabilityRequired"
	}
	return false
}

// 分层锁：authz 是被依赖方，不得 import service / api / security / middleware——
// security 硬编码 "recruiter" 的历史根因正是「security 不能 import service」。
func TestAuthzLock_NoUpwardImports(t *testing.T) {
	forbidden := []string{
		"forklift-training/internal/service",
		"forklift-training/internal/api",
		"forklift-training/internal/security",
		"forklift-training/internal/middleware",
	}
	entries, err := os.ReadDir(filepath.Join("..", "authz"))
	if err != nil {
		t.Fatalf("读取 authz 目录失败: %v", err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join("..", "authz", e.Name())
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("解析 %s 失败: %v", path, err)
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, "\"")
			for _, bad := range forbidden {
				if p == bad {
					t.Errorf("%s import 了 %s：authz 必须是最底层（否则 security/middleware 无法引用它）", path, p)
				}
			}
		}
	}
}

// 能力表覆盖锁：能力键必须形如「资源域.动作」（含且仅含一个点），且能力表非空。
func TestAuthzLock_CapabilityNaming(t *testing.T) {
	caps := authz.AllCapabilities()
	if len(caps) == 0 {
		t.Fatal("能力表不得为空")
	}
	for _, c := range caps {
		key := string(c)
		if strings.Count(key, ".") != 1 {
			t.Errorf("能力键 %q 不符合「资源域.动作」命名（应恰有一个点）", key)
		}
		if strings.HasPrefix(key, ".") || strings.HasSuffix(key, ".") {
			t.Errorf("能力键 %q 的资源域或动作不得为空", key)
		}
		if len(authz.RolesFor(c)) == 0 {
			t.Errorf("能力 %q 没有任何角色", key)
		}
	}
}
