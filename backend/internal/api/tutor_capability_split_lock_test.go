package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"forklift-training/internal/authz"
	"forklift-training/internal/testutil"
)

// 讲师域能力键拆分锁（#2595 / 规格块 A）。
//
// 阶段一只做一件事：把原本一键盖住「入口 + 课程读 + 章节写 + 附件增删 + 文件库」的
// tutor.access 拆成三把。**行为必须零变化**（规格 A6），所以本文件锁的不是「权限收紧了」，
// 而是两件互相牵制的事：
//
//  1. 结构锁：每条路由挂在哪个键上，逐条对上（拆错了键名、把写路由漏在读组里，都在这里红）；
//  2. 行为锁：讲师 token 打全部讲师面端点仍不被 403（三键全拿 ⇒ 全端点可达），
//     非讲师仍一律 403（入口键没被写组旁路掉）。
//
// 为什么结构锁用 AST 而不是跑起来问 gin：gin 不在路由上暴露中间件链，运行期拿不到
// 「这条路由要什么能力」；静态扫描注册函数是能落地的最近似形态（与 authz_coverage_lock_test
// 同一手法）。行为锁则用真实路由 + 真实 token，补上结构锁看不见的那一半。

// tutorRouteCapability 讲师域一条路由应挂的能力键。
type tutorRouteCapability struct {
	route string // "METHOD /path"（路径为蓝图内相对路径，不含 /api/tutor 前缀）
	want  authz.Capability
}

// wantTutorRoutes 讲师域 9 条路由的挂键账本（spec A1/A2）。
//
// 读三端点留 tutor.access；写六端点（上传附件、提交审核、图文插图、更新章节、删除附件、
// 批量删除附件）挂 course.author。
var wantTutorRoutes = []tutorRouteCapability{
	{"GET /courses", authz.CapTutorAccess},
	{"GET /course/:course_id/chapters", authz.CapTutorAccess},
	{"GET /chapter/:chapter_id", authz.CapTutorAccess},

	{"POST /chapter/:chapter_id/upload", authz.CapCourseAuthor},
	{"POST /chapter/:chapter_id/submit-review", authz.CapCourseAuthor},
	{"POST /upload-image", authz.CapCourseAuthor},
	{"PUT /chapter/:chapter_id", authz.CapCourseAuthor},
	{"DELETE /file/:file_id", authz.CapCourseAuthor},
	{"POST /files/batch-delete", authz.CapCourseAuthor},
}

// wantTutorFileRoutes 文件库整组 11 条路由全部挂 tutorfile.manage（spec A2）。
//
// 这一组不拆读/写：文件库的读写一律按归属导师收口（tutorfile/service.go 的 ErrForbidden），
// 能力键只回答「这个人能不能进文件库」，粒度再细就落进数据级谓词该管的地方了。
var wantTutorFileRoutes = []tutorRouteCapability{
	{"POST /files/upload", authz.CapTutorFileManage},
	{"GET /files/upload/:session_id", authz.CapTutorFileManage},
	{"PUT /files/upload/:session_id/chunk/:part_number", authz.CapTutorFileManage},
	{"POST /files/upload/:session_id/complete", authz.CapTutorFileManage},
	{"POST /files/upload/:session_id/abort", authz.CapTutorFileManage},
	{"GET /files", authz.CapTutorFileManage},
	{"POST /files/:file_id/attach", authz.CapTutorFileManage},
	{"POST /files/:file_id/detach", authz.CapTutorFileManage},
	{"POST /files/:file_id/rename", authz.CapTutorFileManage},
	{"POST /chapter/:chapter_id/files/swap", authz.CapTutorFileManage},
	{"DELETE /files/:file_id", authz.CapTutorFileManage},
}

// TestTutorDomainRouteCapabilityMapping 结构锁：注册函数里每条路由挂的键与账本逐条相等
// （两个方向都比：漏挂的红，账本没登记的新路由也红）。
func TestTutorDomainRouteCapabilityMapping(t *testing.T) {
	t.Parallel()
	cases := []struct {
		file     string
		fn       string
		expected []tutorRouteCapability
	}{
		{"internal/tutor/handler.go", "RegisterRoutes", wantTutorRoutes},
		{"internal/tutorfile/handler.go", "RegisterRoutes", wantTutorFileRoutes},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			got := parseRouteGates(t, tc.file, tc.fn)
			want := make(map[string]authz.Capability, len(tc.expected))
			for _, e := range tc.expected {
				want[e.route] = e.want
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
				t.Fatalf("%s 一条路由都没解析出来——锁失效（注册函数被改名或改写形态？）", tc.file)
			}
		})
	}
}

// TestTutorDomainReachabilityUnchanged 行为锁（spec A6）：讲师三键全拿 ⇒ 全部讲师面端点可达；
// 任何非讲师角色一律 403。
//
// 「可达」只判「不是 401/403」：下游 400（空 body）、404（对象不存在）、500 都不是授权问题，
// 本锁管的是守卫这一层。这既是「拆键没把讲师挡在门外」的回归网，也是「入口键没被写组旁路」
// 的反向网——一旦有一条写路由漏了守卫，学生那半边就会红。
func TestTutorDomainReachabilityUnchanged(t *testing.T) {
	r, _, db := newSlice6Env(t)
	tutorTok := tokenFor(t, db, "tutor")
	studentTok := tokenFor(t, db, "student")

	all := append(append([]tutorRouteCapability{}, wantTutorRoutes...), wantTutorFileRoutes...)
	for _, rc := range all {
		method, path, ok := strings.Cut(rc.route, " ")
		if !ok {
			t.Fatalf("账本项 %q 不是「METHOD /path」形态", rc.route)
		}
		reqPath := "/api/tutor" + fillPathParams(path)

		t.Run("tutor "+rc.route, func(t *testing.T) {
			rec := doAuthedRequest(r, method, reqPath, "Bearer "+tutorTok)
			if rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
				t.Fatalf("讲师应可达 %s %s（三键全拿），得到 %d: %s", method, reqPath, rec.Code, rec.Body.String())
			}
		})
		t.Run("student "+rc.route, func(t *testing.T) {
			rec := doAuthedRequest(r, method, reqPath, "Bearer "+studentTok)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("非讲师访问 %s %s 应 403，得到 %d: %s", method, reqPath, rec.Code, rec.Body.String())
			}
		})
	}
}

// parseRouteGates 解析一个注册函数体内的「路由 → 守卫能力键」映射。
//
// 认得出的写法只有一种（本仓讲师域就是这一种，也是 #2595 引入的分组形态）：
//
//	x := parent.Group(prefix[, middleware.CapabilityRequired(authz.CapX)])
//	x.GET(path, handler)
//
// 认不出的写法一律 Fatal 而不是静默当作「无守卫」——锁跟不上写法就等于锁没了。
func parseRouteGates(t *testing.T, modulePath, funcName string) map[string]authz.Capability {
	t.Helper()
	src := testutil.FindCode(t, testutil.ScanBackendCode(t), modulePath)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, modulePath, src.Src, 0)
	if err != nil {
		t.Fatalf("解析 %s 失败: %v", modulePath, err)
	}
	var fn *ast.FuncDecl
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == funcName {
			fn = fd
		}
	}
	if fn == nil {
		t.Fatalf("%s 里找不到函数 %s", modulePath, funcName)
	}

	// groupCap 路由组变量 → 守卫能力；known 记录见过的组变量（判定「路由挂在不认识的 receiver 上」）。
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
			if constName := guardConstInGroupCall(call); constName != "" {
				groupCap[lhs.Name] = capabilityFromConst(t, constName)
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
			if !isHTTPMethod(method) {
				return true
			}
			if len(node.Args) == 0 {
				return true
			}
			lit, ok := node.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			path, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			if !known[recv.Name] {
				t.Fatalf("%s: 路由 %s %s 挂在不认识的 receiver %q 上——本锁认不出这条路的守卫，请改写注册函数或扩展本锁",
					modulePath, method, path, recv.Name)
			}
			out[method+" "+path] = groupCap[recv.Name]
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

// capabilityFromConst 能力常量名 → 能力键。未登记的常量名即 Fatal：拆键换了挂键方式时，
// 这里必须被人看一眼再改，而不是静默回落成「无守卫」。
func capabilityFromConst(t *testing.T, name string) authz.Capability {
	t.Helper()
	switch name {
	case "CapTutorAccess":
		return authz.CapTutorAccess
	case "CapCourseAuthor":
		return authz.CapCourseAuthor
	case "CapTutorFileManage":
		return authz.CapTutorFileManage
	}
	t.Fatalf("未登记的能力常量 %q：请更新本锁的映射表（fail closed，不静默当作无守卫）", name)
	return ""
}

// isHTTPMethod gin 的路由注册方法名。
func isHTTPMethod(name string) bool {
	switch name {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
		return true
	}
	return false
}

// fillPathParams 把路径里的 :param 段替换成固定 id（只为拿到一条可寻址的真实路径）。
func fillPathParams(path string) string {
	parts := strings.Split(path, "/")
	for i, seg := range parts {
		if strings.HasPrefix(seg, ":") {
			parts[i] = "1"
		}
	}
	return strings.Join(parts, "/")
}

// doAuthedRequest 打一发带 Authorization 头的请求。
func doAuthedRequest(r http.Handler, method, path, authHeader string) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(method, path, nil)
	req.Header.Set("Authorization", authHeader)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}
