package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// 端点授权覆盖锁（ADR-0047 §1 / spec #928 决策 6；代码审查补交）。
//
// 规则：每个蓝图注册函数在其函数体内必须显式出现能力守卫（middleware.CapabilityRequired），
// 否则登记进 allowlist 并写明理由——「新蓝图忘了挂授权」以前只能靠人记得，现在要么有守卫，
// 要么有一个被 review 看见的豁免决定。
//
// 为什么不做「路由 → 能力」的全枚举：gin 不在路由上暴露中间件链，运行期拿不到「这条路由需要什么能力」；
// 静态扫描注册函数是能落地的最近似形态，且与本仓既有 AST 锁（authz_lock_test.go）同一手法。
func TestBlueprintCapabilityCoverage(t *testing.T) {
	// allowlist：确实没有能力位、且理由成立的蓝图（新增一项 = 一次显式的豁免决定）。
	allow := map[string]string{
		"RegisterCaptchaRoutes":             "图形验证码：无需鉴权的公开端点",
		"RegisterEmailAuthRoutes":           "认证入口：此刻尚无角色，能力守卫无从判定",
		"RegisterPhoneAuthRoutes":           "认证入口：同上",
		"RegisterWechatAuthRoutes":          "认证入口：同上",
		"RegisterProfileBindRoutes":         "认证入口：手机号/邮箱绑定，属账号自身而非资源域",
		"RegisterCoursesRoutes":             "课程读面横跨学员/讲师/管理端（讲师与管理员读同一份章节详情），挂学员能力会误伤；能力位细化留待后续",
		"RegisterSearchRoutes":              "公开搜索端点（无 JWTAuth）",
		"RegisterDiagnosisRoutes":           "诊断只读代理（品牌/车型/故障码/手册），公开面",
		"RegisterNotificationRoutes":        "站内信按收件人鉴权（任何已登录角色都可能收到），不是资源域能力",
		"RegisterQuestionInteractionRoutes": "题目评论/笔记/考点为学员面，但讲师与管理端审核读同一份；能力位细化留待后续",
		"RegisterQuestionBankRoutes":        "题库蓝图混合学员读写与管理端审核：管理端路由逐条挂能力守卫，学员侧继承组级 JWTAuth",
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("读取 api 目录失败: %v", err)
	}
	fset := token.NewFileSet()
	checked, exempted := 0, 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, perr := parser.ParseFile(fset, name, nil, 0)
		if perr != nil {
			t.Fatalf("解析 %s 失败: %v", name, perr)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Register") || !strings.HasSuffix(fn.Name.Name, "Routes") {
				return true
			}
			checked++
			hasCapability := false
			ast.Inspect(fn.Body, func(inner ast.Node) bool {
				call, ok := inner.(*ast.CallExpr)
				if !ok {
					return true
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "CapabilityRequired" {
					hasCapability = true
				}
				return true
			})
			if hasCapability {
				return true
			}
			if reason, ok := allow[fn.Name.Name]; ok {
				exempted++
				t.Logf("豁免: %s —— %s", fn.Name.Name, reason)
				return true
			}
			t.Errorf("%s 既没有 middleware.CapabilityRequired，也不在豁免清单里：请挂能力守卫，或写明豁免理由（ADR-0047 §1）", fn.Name.Name)
			return true
		})
	}
	if checked == 0 {
		t.Fatal("未找到任何蓝图注册函数——锁失效（命名变了？）")
	}
	t.Logf("蓝图注册函数 %d 个（豁免 %d 个）", checked, exempted)
}
