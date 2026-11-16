package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"path"
	"strconv"
	"strings"
	"testing"

	"forklift-training/internal/authz"
	"forklift-training/internal/testutil"
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
	t.Parallel()
	// allowlist：确实没有能力位、且理由成立的蓝图（新增一项 = 一次显式的豁免决定）。
	allow := map[string]string{
		"auth.RegisterCaptchaRoutes":         "图形验证码：无需鉴权的公开端点",
		"auth.RegisterEmailAuthRoutes":       "认证入口：此刻尚无角色，能力守卫无从判定",
		"auth.RegisterPhoneAuthRoutes":       "认证入口：同上",
		"auth.RegisterWechatAuthRoutes":      "认证入口：同上",
		"auth.RegisterProfileBindRoutes":     "认证入口：手机号/邮箱绑定，属账号自身而非资源域",
		"auth.RegisterRoutes":                "公开登录/刷新/登出面 + 本人资料与注销（任何已登录角色都是自己的资料），无资源域能力位",
		"course.RegisterRoutes":              "课程读面横跨学员/讲师/管理端（讲师与管理员读同一份章节详情），挂学员能力会误伤；能力位细化留待后续",
		"training.RegisterRoutes":            "培训目录读面同样横跨学员/管理端（目录树、等级、标签、岗位、证件都是同一份），挂学员能力会误伤；能力位细化留待后续",
		"search.RegisterRoutes":              "公开搜索端点（无 JWTAuth）",
		"notification.RegisterRoutes":        "站内信按收件人鉴权（学员与讲师各有一格信箱，按 JWT role 分派到 user_id/tutor_id），不是资源域能力；admin/recruiter 无信箱由 handler 显式 403（ADR-0095 决策三）",
		"aiassistant.RegisterAdminRoutes":    "AI 配置管理面：挂 internal/admin/handler.go:43 的 /admin 组（组级 JWTAuth + CapabilityRequired(authz.CapAdminAccess)），函数体内不再重复守卫",
		"questioninteraction.RegisterRoutes": "题目评论/笔记/考点为学员面，但讲师与管理端审核读同一份；能力位细化留待后续",
		"questionbank.RegisterRoutes":        "题库蓝图混合学员读写与管理端审核：管理端路由逐条挂能力守卫，学员侧继承组级 JWTAuth",
		"note.RegisterRoutes":                "学员笔记：纯用户私有数据（读写一律以 user_id 收口，越权按「不存在」处理），门禁与既有题目笔记端点一致——只要求登录；同一资源的两条路径挂两套门才是真不一致（ADR-0055）",
	}
	// 射程 = HTTP 面（testutil.HTTPSurface）：拆包后蓝图注册函数跟着域包走，这一处定义自动覆盖。
	fset := token.NewFileSet()
	checked, exempted := 0, 0
	for _, src := range testutil.ScanBackendCode(t) {
		if !testutil.HTTPSurface(src) {
			continue
		}
		name := src.Path
		f, perr := parser.ParseFile(fset, name, src.Src, 0)
		if perr != nil {
			t.Fatalf("解析 %s 失败: %v", name, perr)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Register") || !strings.HasSuffix(fn.Name.Name, "Routes") {
				return true
			}
			checked++
			// 键 = 包名.函数名（域包）或裸名（internal/api）：域包拆出去之后 faq/handler.go 与
			// notification/handler.go 会各有一个 RegisterRoutes，裸名会让两条蓝图共用一份豁免
			// ——豁免是逐蓝图的一次决定，键撞了就等于其中一条没人看过（ADR-0070）。
			key := fn.Name.Name
			if src.Dir != "internal/api" {
				key = path.Base(src.Dir) + "." + key
			}
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
			if reason, ok := allow[key]; ok {
				exempted++
				t.Logf("豁免: %s —— %s", key, reason)
				return true
			}
			t.Errorf("%s 既没有 middleware.CapabilityRequired，也不在豁免清单里：请挂能力守卫，或写明豁免理由（ADR-0047 §1）", key)
			return true
		})
	}
	if checked == 0 {
		t.Fatal("未找到任何蓝图注册函数——锁失效（命名变了？）")
	}
	t.Logf("蓝图注册函数 %d 个（豁免 %d 个）", checked, exempted)
}

// tutorRouteGate 讲师域一条路由的挂键账本项（spec #2595 A1/A2）。
type tutorRouteGate struct {
	method string
	path   string // 蓝图内相对路径（不含 /api/tutor 前缀）
	want   authz.Capability
}

// wantTutorRoutes 讲师域 9 条路由的挂键账本（spec A2）：读三端点留 tutor.access，写六端点挂
// course.author。写面含附件上传删除与图文传图——它们改的都是章节内容。
var wantTutorRoutes = []tutorRouteGate{
	{http.MethodGet, "/courses", authz.CapTutorAccess},
	{http.MethodGet, "/course/:course_id/chapters", authz.CapTutorAccess},
	{http.MethodGet, "/chapter/:chapter_id", authz.CapTutorAccess},

	{http.MethodPost, "/chapter/:chapter_id/upload", authz.CapCourseAuthor},
	{http.MethodPost, "/chapter/:chapter_id/submit-review", authz.CapCourseAuthor},
	{http.MethodPost, "/upload-image", authz.CapCourseAuthor},
	{http.MethodPut, "/chapter/:chapter_id", authz.CapCourseAuthor},
	{http.MethodDelete, "/file/:file_id", authz.CapCourseAuthor},
	{http.MethodPost, "/files/batch-delete", authz.CapCourseAuthor},
}

// wantTutorFileRoutes 文件库蓝图的路由账本（spec A2 + #2595 裁定）：除章节侧换挂外全部挂
// tutorfile.manage。
//
// 这一组不拆读/写：文件库的读写一律按归属导师收口（tutorfile/service.go 的 ErrForbidden），
// 能力键只回答「这个人能不能进文件库」，粒度再细就落进数据级谓词该管的地方了。
//
// swap 是唯一例外：它长在本蓝图里但改的是**章节**的附件挂接（并 MarkChapterDraft），
// 故挂 course.author —— 键跟着它改变的资源走。
var wantTutorFileRoutes = []tutorRouteGate{
	{http.MethodPost, "/files/upload", authz.CapTutorFileManage},
	{http.MethodGet, "/files/upload/:session_id", authz.CapTutorFileManage},
	{http.MethodPut, "/files/upload/:session_id/chunk/:part_number", authz.CapTutorFileManage},
	{http.MethodPost, "/files/upload/:session_id/complete", authz.CapTutorFileManage},
	{http.MethodPost, "/files/upload/:session_id/abort", authz.CapTutorFileManage},
	{http.MethodGet, "/files", authz.CapTutorFileManage},
	{http.MethodPost, "/files/:file_id/attach", authz.CapTutorFileManage},
	{http.MethodPost, "/files/:file_id/detach", authz.CapTutorFileManage},
	{http.MethodPost, "/files/:file_id/rename", authz.CapTutorFileManage},
	{http.MethodPost, "/chapter/:chapter_id/files/swap", authz.CapCourseAuthor},
	{http.MethodDelete, "/files/:file_id", authz.CapTutorFileManage},
}

// TestTutorDomainRouteCapabilityMapping 讲师域逐路由挂键（spec #2595 A2 / Testing Decisions 阶段一
// 「路由守卫存在性」）。
//
// 上一条测试只判「注册函数体里出现过 CapabilityRequired」——讲师域拆成读组/写组之后，那个判据
// 就分辨不出「9 条路由全挂在入口键上」和「写组挂了 course.author」，于是拆键可以静默失效。本测试
// 把账本与代码逐条对齐（两个方向都比：挂错的红，账本没登记的新路由也红）。
func TestTutorDomainRouteCapabilityMapping(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		file string
		want []tutorRouteGate
	}{
		{"internal/tutor/handler.go", wantTutorRoutes},
		{"internal/tutorfile/handler.go", wantTutorFileRoutes},
	} {
		t.Run(tc.file, func(t *testing.T) {
			got := parseRouteGates(t, tc.file, "RegisterRoutes")
			want := make(map[string]authz.Capability, len(tc.want))
			for _, g := range tc.want {
				want[g.method+" "+g.path] = g.want
			}
			for route, wantCap := range want {
				gotCap, ok := got[route]
				if !ok {
					t.Errorf("%s 里找不到路由 %q（账本登记了但代码没有？）", tc.file, route)
					continue
				}
				if gotCap != wantCap {
					t.Errorf("路由 %q 挂在 %q 上，应为 %q", route, gotCap, wantCap)
				}
			}
			for route, gotCap := range got {
				if _, ok := want[route]; !ok {
					t.Errorf("%s 新增了账本未登记的路由 %q（挂在 %q）——请确认它该挂哪个键并更新账本", tc.file, route, gotCap)
				}
			}
			if len(got) == 0 {
				t.Fatalf("%s 一条路由都没解析出来——锁失效（注册函数被改名或改写成认不出的形态？）", tc.file)
			}
		})
	}
}

// TestTutorDomainReachabilityUnchanged 讲师域可达面（spec #2595 A6 零行为变化 + Testing Decisions
// 阶段一「行为不变断言」）。
//
// 三件事：
//  1. 讲师三键全拿 ⇒ 全部 20 条路由不被 401/403（拆键没把讲师挡在门外）；
//  2. 学生 ⇒ 一律 403（写组没被入口键旁路，也没有哪条路由漏挂守卫）；
//  3. 管理员 ⇒ 讲师面一律 403（A1：两个新键不授超管，管理员可达面与今天逐字相同）。
//
// 第 1 条只判「不是 401/403」会让「路由被删」也算通过（404）——所以先拿 gin 的路由表核对每条
// 路由确实注册着，再打请求。少了这一步，整条断言在路由消失时仍绿（假绿）。
func TestTutorDomainReachabilityUnchanged(t *testing.T) {
	// 不 t.Parallel()：三种身份各自 seed 固定账号名（学员走 seedStudent 的计数器与用户名），
	// 与别的并行用例同时 seed 会撞唯一键。本用例只打 60 发本机请求，串行代价可忽略。
	r, cfg, db := newSlice6Env(t)
	tutorTok := tokenFor(t, db, "tutor")
	studentTok := tokenFor(t, db, "student")
	adminTok := adminTokenWithAccount(t, cfg, db, "tutor_split_admin")

	registered := map[string]bool{}
	for _, rt := range r.Routes() {
		registered[rt.Method+" "+rt.Path] = true
	}

	all := append(append([]tutorRouteGate{}, wantTutorRoutes...), wantTutorFileRoutes...)
	for _, g := range all {
		// 请求打填充过的路径；查路由表用**原始**路径——gin 的 Routes() 返回注册时的 :param 形态。
		pattern := "/api/tutor" + g.path
		full := "/api/tutor" + fillPathParams(g.path)

		t.Run("registered "+g.method+" "+g.path, func(t *testing.T) {
			if !registered[g.method+" "+pattern] {
				t.Fatalf("路由 %s %s 不在 gin 路由表里——账本登记的路由已经不存在了", g.method, pattern)
			}
		})
		t.Run("tutor "+g.method+" "+g.path, func(t *testing.T) {
			if rec := catalogRequest(t, r, tutorTok, g.method, full, ""); rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
				t.Fatalf("讲师应可达 %s %s（三键全拿），得到 %d: %s", g.method, full, rec.Code, rec.Body.String())
			}
		})
		t.Run("student "+g.method+" "+g.path, func(t *testing.T) {
			if rec := catalogRequest(t, r, studentTok, g.method, full, ""); rec.Code != http.StatusForbidden {
				t.Fatalf("非讲师访问 %s %s 应 403，得到 %d: %s", g.method, full, rec.Code, rec.Body.String())
			}
		})
		t.Run("admin "+g.method+" "+g.path, func(t *testing.T) {
			if rec := catalogRequest(t, r, adminTok, g.method, full, ""); rec.Code != http.StatusForbidden {
				t.Fatalf("管理员访问讲师面 %s %s 应 403（新键不授超管），得到 %d: %s", g.method, full, rec.Code, rec.Body.String())
			}
		})
	}
}

// tutorGuardConsts 讲师域守卫允许出现的能力常量（fail closed：不在表里即 Fatal，不静默当作无守卫）。
var tutorGuardConsts = map[string]authz.Capability{
	"CapTutorAccess":     authz.CapTutorAccess,
	"CapCourseAuthor":    authz.CapCourseAuthor,
	"CapTutorFileManage": authz.CapTutorFileManage,
}

// parseRouteGates 解析一个注册函数体内的「METHOD /path → 守卫能力键」映射。
//
// 认得出的写法只有一种（讲师域就是这一种，也是 #2595 引入的读组/写组形态）：
//
//	x := parent.Group(prefix[, middleware.CapabilityRequired(authz.CapX)])
//	x.GET(path, handler)
//
// 认不出的一律 Fatal 而不是记成「无守卫」：锁跟不上写法就等于锁没了。
func parseRouteGates(t *testing.T, filePath, funcName string) map[string]authz.Capability {
	t.Helper()
	src := testutil.FindCode(t, testutil.ScanBackendCode(t), filePath)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, src.Src, 0)
	if err != nil {
		t.Fatalf("解析 %s 失败: %v", filePath, err)
	}
	var fn *ast.FuncDecl
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == funcName {
			fn = fd
		}
	}
	if fn == nil {
		t.Fatalf("%s 里找不到函数 %s", filePath, funcName)
	}

	// groupCap 路由组变量 → 守卫能力；known 记录见过的组变量。
	// 「组存在但没有守卫能力」是合法的（JWTAuth 组本身不挂能力），但**在它上面注册路由**不合法。
	groupCap := map[string]authz.Capability{}
	known := map[string]bool{}
	out := map[string]authz.Capability{}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			if len(node.Lhs) != 1 || len(node.Rhs) != 1 {
				return true
			}
			lhs, ok := node.Lhs[0].(*ast.Ident)
			if !ok {
				return true
			}
			call, ok := node.Rhs[0].(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Group" {
				return true
			}
			known[lhs.Name] = true
			if name := guardConstInGroupCall(call); name != "" {
				groupCap[lhs.Name] = tutorGuardConst(t, name)
			}
		case *ast.CallExpr:
			sel, ok := node.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			recv, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			method := sel.Sel.Name
			if !isHTTPMethod(method) || len(node.Args) == 0 {
				return true
			}
			lit, ok := node.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			routePath, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			if !known[recv.Name] {
				t.Fatalf("%s: 路由 %s %s 挂在不认识的 receiver %q 上——本锁认不出这条路的守卫，请扩展本锁",
					filePath, method, routePath, recv.Name)
			}
			// 逐路由中间件（第三个及以后的参数）不在本锁的认法里：静默忽略会把它当成
			// 「只有组守卫」，而实际可能更严也可能更松——宁可 Fatal 让人来看一眼。
			if len(node.Args) > 2 {
				t.Fatalf("%s: 路由 %s %s 挂了逐路由中间件——本锁只认组级守卫，请扩展本锁",
					filePath, method, routePath)
			}
			cap, ok := groupCap[recv.Name]
			if !ok {
				t.Fatalf("%s: 路由 %s %s 挂在组 %q 上，而该组没有能力守卫——漏挂守卫或本锁认不出",
					filePath, method, routePath, recv.Name)
			}
			out[method+" "+routePath] = cap
		}
		return true
	})
	return out
}

// guardConstInGroupCall 取 Group(...) 调用里 CapabilityRequired(authz.CapX) 的常量名；没有守卫返回 ""。
func guardConstInGroupCall(call *ast.CallExpr) string {
	for _, arg := range call.Args {
		inner, ok := arg.(*ast.CallExpr)
		if !ok {
			continue
		}
		sel, ok := inner.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "CapabilityRequired" || len(inner.Args) == 0 {
			continue
		}
		if capSel, ok := inner.Args[0].(*ast.SelectorExpr); ok {
			return capSel.Sel.Name
		}
	}
	return ""
}

// tutorGuardConst 能力常量名 → 能力键；未登记即 Fatal（fail closed）。
func tutorGuardConst(t *testing.T, name string) authz.Capability {
	t.Helper()
	c, ok := tutorGuardConsts[name]
	if !ok {
		t.Fatalf("未登记的能力常量 %q：请更新 tutorGuardConsts（fail closed，不静默当作无守卫）", name)
	}
	return c
}

// isHTTPMethod gin 的路由注册方法名。
func isHTTPMethod(name string) bool {
	switch name {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

// fillPathParams 把路径里的 :param 段替换成固定 id（只为拿到一条可寻址的真实路径）。
func fillPathParams(routePath string) string {
	parts := strings.Split(routePath, "/")
	for i, seg := range parts {
		if strings.HasPrefix(seg, ":") {
			parts[i] = "1"
		}
	}
	return strings.Join(parts, "/")
}
