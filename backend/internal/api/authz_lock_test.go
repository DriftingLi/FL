package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"forklift-training/internal/authz"
	"forklift-training/internal/testutil"
)

// 角色字面量锁（ADR-0047 §1 / spec #928 决策 3-4）：端点守卫只允许用 authz 角色常量，
// 不允许再出现 RoleRequired("admin") 这类字面量——否则「角色→能力」的单一声明会被绕过，
// 而 84 处字面量正是本次收敛要消灭的东西。
//
// 用 AST 而非正则：正则会被注释与字符串内容骗过（本仓库的注释里就出现过 "admin"）。
func TestAuthzLock_NoRawRoleLiteralInGuards(t *testing.T) {
	t.Parallel()
	// 射程 = HTTP 面（testutil.HTTPSurface）：今天 internal/api + internal/valuation/handler，
	// 拆包（P2）后由那一处定义跟着走；不再靠「本文件所在目录 + 一条 ../valuation/handler」。
	fset := token.NewFileSet()
	checked := 0
	for _, src := range testutil.ScanBackendCode(t) {
		if !testutil.HTTPSurface(src) {
			continue
		}
		f, err := parser.ParseFile(fset, src.Path, src.Src, 0)
		if err != nil {
			t.Fatalf("解析 %s 失败: %v", src.Path, err)
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
					t.Errorf("%s: 角色字面量 %s 应改用 authz 角色常量（RoleRequired 的参数只能是 authz.RoleX）", src.Path, lit.Value)
				}
			}
			return true
		})
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
	t.Parallel()
	forbidden := []string{
		"forklift-training/internal/core",
		"forklift-training/internal/api",
		"forklift-training/internal/security",
		"forklift-training/internal/middleware",
	}
	// 射程按**模块根相对目录**认（internal/authz），不再用 ../authz：测试文件随域包搬家后，
	// cwd 相对路径会静默指向别处。
	fset := token.NewFileSet()
	for _, src := range testutil.ScanBackendCode(t) {
		if src.Test || src.Dir != "internal/authz" {
			continue
		}
		path := src.Path
		f, err := parser.ParseFile(fset, path, src.Src, parser.ImportsOnly)
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
	t.Parallel()
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
		// 空行 = 该能力的可达面由数据层回答（受保护管理角色持有，#1618 段1）。
		// 「空行 ⇔ 在受保护清单里」由 internal/authz 的 dynamic_admin_test.go 双向锁定，
		// 这里只确认它不是没人认领的孤儿键。
		if len(authz.RolesFor(c)) == 0 && !isProtectedAdminCapability(c) {
			t.Errorf("能力 %q 没有任何角色，也不在受保护管理角色能力全集里（孤儿键）", key)
		}
	}
}

// 收敛锁（ADR-0047 §1 / spec #928 决策 1）：迁移完成后守卫只有一个形态——CapabilityRequired。
// RoleRequired 若复活即为回归——它是 pass-through（21 行查表），删掉后复杂度只会散回 34 个蓝图；
// 「角色 → 可达面」的事实源只能是 authz 能力表。
func TestAuthzLock_RoleGuardRetired(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	var offenders []string
	err := filepath.WalkDir(filepath.Join(".."), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "RoleRequired" {
				offenders = append(offenders, path)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}
	if len(offenders) > 0 {
		t.Fatalf("守卫必须统一为 CapabilityRequired，以下文件仍在用 RoleRequired: %v", offenders)
	}
}

// isProtectedAdminCapability 该能力是否由受保护（超级管理员）角色持有 —— 即静态表对它没有回答
// （#1618 段1 起管理端能力改为数据层回答）。判据来自 authz 自己的声明，不在这里另抄一份清单。
func isProtectedAdminCapability(c authz.Capability) bool {
	for _, p := range authz.ProtectedAdminCapabilities() {
		if p == c {
			return true
		}
	}
	return false
}
