// 族判定（#1376 跨端评审 · 移动端 ADR-0030 ② 第 2 条与 ④ 第 1 项）：
// 两族 refresh cookie 可以在同一个 host 上并存，「读哪一族」必须由 **access** 定，
// 判据与鉴权中间件对「你是谁」的回答同源（Bearer 头优先，其次 access cookie）。
//
// 本文件锁三件事，形状各不同：
//
//	① 表驱动的双向：两族并存时头/cookie 指向招聘者 ⇒ 取招聘者那一族；指向学员 ⇒ 取学员那一族；
//	② 负锁：两族并存而**没有任何族线索** ⇒ 返回空串（端点因此回退请求体，两条都缺即 401）。
//	   这一行就是「不许按 Cookie 名序任选一族」的牙齿——旧实现在这里会返回主站那一族（rt-main），
//	   于是招聘者面的续期静默轮换掉学员的活跃会话；
//	③ role 解析不验签、也不许被认证判定调用（它只回答「客户端自认哪一族」，不回答「你是谁可信」）。
package security

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// familySession 两族都配齐的会话：主站父域 + 招聘者 host-only（ADR-0022 的招牌形状）。
func familySession() *Session {
	return NewSessionWithRecruiterCookie(testSecret, time.Hour, 7*24*time.Hour,
		CookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: true},
		CookieConfig{Name: "recruiter_token", Domain: "", Secure: true},
		newInmemoryBlacklistStore())
}

// familyRequest 造一枚打到 /api/auth/refresh 的请求。族线索（access 头 / access cookie）由
// 用例决定，传空串表示「这条线索不存在」；families 决定带哪几枚 refresh cookie，
// 传 nil 表示**两族并存**（本缺陷成立的先决条件）。
func familyRequest(t *testing.T, sess *Session, headerRole, accessCookieRole string, families []string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, RefreshCookiePath, nil)
	if headerRole != "" {
		access, _, err := sess.IssuePair(1, "u1", headerRole)
		if err != nil {
			t.Fatalf("签发 %s 族 access 失败: %v", headerRole, err)
		}
		req.Header.Set("Authorization", "Bearer "+access)
	}
	if accessCookieRole != "" {
		access, _, err := sess.IssuePair(1, "u1", accessCookieRole)
		if err != nil {
			t.Fatalf("签发 %s 族 access 失败: %v", accessCookieRole, err)
		}
		name := sess.CookieName()
		if accessCookieRole == roleRecruiter {
			name = sess.RecruiterCookieName()
		}
		req.AddCookie(&http.Cookie{Name: name, Value: access})
	}
	if families == nil {
		families = []string{"main", "rec"}
	}
	for _, f := range families {
		switch f {
		case "main":
			req.AddCookie(&http.Cookie{Name: sess.RefreshCookieName(), Value: "rt-main"})
		case "rec":
			req.AddCookie(&http.Cookie{Name: sess.RecruiterRefreshCookieName(), Value: "rt-rec"})
		default:
			t.Fatalf("夹具里的族名写错了：%q（只认 main / rec）", f)
		}
	}
	return req
}

func TestRefreshFamily_族由access定(t *testing.T) {
	sess := familySession()

	cases := []struct {
		name             string
		headerRole       string   // Bearer 头里的 role
		accessCookieRole string   // access cookie 里的 role
		families         []string // 带哪几枚 refresh cookie（nil = 两族并存）
		want             string
	}{
		// ① 双向：并存时谁被轮换，取决于 access 说你是谁
		{name: "头说招聘者", headerRole: roleRecruiter, want: "rt-rec"},
		{name: "头说学员", headerRole: "hrwai_user", want: "rt-main"},
		{name: "无头时按 access cookie 定族（招聘者）", accessCookieRole: roleRecruiter, want: "rt-rec"},
		{name: "无头时按 access cookie 定族（学员）", accessCookieRole: "hrwai_user", want: "rt-main"},
		// 头优先于 access cookie——与 ExtractToken/鉴权中间件的顺序逐字同口径
		{name: "头与 access cookie 相反时头赢", headerRole: roleRecruiter, accessCookieRole: "hrwai_user", want: "rt-rec"},
		// 只读本族：本族没带 Cookie 就当作「Cookie 通道不可用」，不许跨族去拿另一枚
		{name: "族线索=招聘者但只带了主站 Cookie", headerRole: roleRecruiter, families: []string{"main"}, want: ""},
		// ② 负锁：无线索 ⇒ 一律不读，绝不回退名序（这一行就是本票的缺陷本体）
		{name: "两族并存而无线索 ⇒ 不回退名序", want: ""},
	}
	for _, c := range cases {
		req := familyRequest(t, sess, c.headerRole, c.accessCookieRole, c.families)
		if got := sess.RefreshCookieForRequest(req); got != c.want {
			t.Errorf("%s: RefreshCookieForRequest = %q, want %q", c.name, got, c.want)
		}
	}
}

// 单族形状的行为逐字不变（防回归）：只带主站 refresh、只带招聘者 refresh 的两族各自单族请求。
func TestRefreshFamily_单族行为不变(t *testing.T) {
	sess := familySession()

	t.Run("只有主站一族", func(t *testing.T) {
		access, _, _ := sess.IssuePair(1, "u1", "hrwai_user")
		req := httptest.NewRequest(http.MethodPost, RefreshCookiePath, nil)
		req.Header.Set("Authorization", "Bearer "+access)
		req.AddCookie(&http.Cookie{Name: sess.RefreshCookieName(), Value: "rt-main"})
		if got := sess.RefreshCookieForRequest(req); got != "rt-main" {
			t.Errorf("单族主站 = %q, want rt-main", got)
		}
	})

	t.Run("只有招聘者一族", func(t *testing.T) {
		access, _, _ := sess.IssuePair(9, "hr001", roleRecruiter)
		req := httptest.NewRequest(http.MethodPost, RefreshCookiePath, nil)
		req.Header.Set("Authorization", "Bearer "+access)
		req.AddCookie(&http.Cookie{Name: sess.RecruiterRefreshCookieName(), Value: "rt-rec"})
		if got := sess.RefreshCookieForRequest(req); got != "rt-rec" {
			t.Errorf("单族招聘者 = %q, want rt-rec", got)
		}
	})
}

// role 解析**不是认证面**（口径 ③ 的行为半边）：一支用别的密钥签的令牌，在这里照样读得出 role
// ——它只回答「客户端自认哪一族」；同一支令牌在 VerifyAccess 那里必须被拒。
// 反过来，若哪天有人给它加上验签/过期校验，这里会红：族线索会在 access 自然过期后消失，
// 浏览器侧就再也续不了期（本函数存在的意义恰恰是「过期也能读」）。
func TestRefreshFamily_Role解析不是认证面(t *testing.T) {
	sess := familySession()
	other := NewSessionWithRecruiterCookie("another-secret", time.Hour, 7*24*time.Hour,
		CookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: true},
		CookieConfig{Name: "recruiter_token", Domain: "", Secure: true},
		newInmemoryBlacklistStore())

	foreign, _, err := other.IssuePair(9, "hr001", roleRecruiter)
	if err != nil {
		t.Fatalf("别的密钥签发令牌失败: %v", err)
	}
	if got := accessRoleOf(foreign); got != roleRecruiter {
		t.Errorf("未验签的 role 解析应只读载荷，实际 = %q", got)
	}
	if _, err := sess.VerifyAccess(foreign); err == nil {
		t.Error("同一支令牌在认证面必须被拒——否则这里就是在做认证，ADR-0067 的族判定就成了提权面")
	}

	for _, junk := range []string{"", "not-a-jwt", "a.b", "a.!!!.c"} {
		if got := accessRoleOf(junk); got != "" {
			t.Errorf("accessRoleOf(%q) = %q, want 空串（解不出即「没有族线索」）", junk, got)
		}
	}
}

// 口径 ③ 的静态半边：`accessRoleOf` 在整个 security 包里**只**允许被 RefreshFamilyRole 调用。
// 用 AST 而不是文本计数：文本会数到注释与函数声明本身（前几波的护栏就在这上面空转过）。
// 射程说明：本函数非导出，跨包不可达；认证判定入口（VerifyAccess/ValidateRefresh/verify）
// 都在同一个文件里，所以「包内 AST 白名单」就是这条判据的完整射程。
func TestRefreshFamily_Role解析不被认证判定调用(t *testing.T) {
	const target = "accessRoleOf"
	allowed := map[string]bool{"RefreshFamilyRole": true}

	entry := token.NewFileSet()
	// ParseDir 的 filter 返回 true 表示**收录**，所以这里要取反（把测试文件排除：
	// 本文件自己就直调 accessRoleOf 做行为断言，收进来会让白名单假红）。
	pkgs, err := parser.ParseDir(entry, ".", func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("解析 security 包失败: %v", err)
	}
	if len(pkgs) == 0 {
		t.Fatal("解析到 0 个包：判据没找到输入，必须报错而不是空跑通过")
	}

	var callers []string
	for _, pkg := range pkgs {
		for file := range pkg.Files {
			ast.Inspect(pkg.Files[file], func(n ast.Node) bool {
				fn, ok := n.(*ast.FuncDecl)
				if !ok {
					return true
				}
				ast.Inspect(fn.Body, func(inner ast.Node) bool {
					call, ok := inner.(*ast.CallExpr)
					if !ok {
						return true
					}
					if id, ok := call.Fun.(*ast.Ident); ok && id.Name == target {
						callers = append(callers, fn.Name.Name)
					}
					return true
				})
				return true
			})
		}
	}
	if len(callers) == 0 {
		t.Fatalf("没找到任何 %s 调用点：判据失去输入，空跑不算绿", target)
	}
	for _, caller := range callers {
		if !allowed[caller] {
			t.Errorf("%s 被 %s 调用了：role 解析只能用于选族，不许进入任何认证判定", target, caller)
		}
	}
}
