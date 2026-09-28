// 5xx 不外发驱动原文（ADR-0064 决策 9）的**执行面**契约。
//
// 背景：规则本体原住在 internal/api 的端点渲染单点（clientErrorText），但**不经端点表**的裸
// handler（文件流 / 上传 / 导出 / 列表直连）另抄了一份「response.ServerError(c, "前缀: "+err.Error())」
// —— 20 处（internal/api 18 + valuation/handler 2），驱动原文照旧外发。
// 本批把规则提到 pkg/response.ClientErrorText，并给裸 handler 一格
// response.ServerErrorCause（固定文案 + c.Error 记账），20 处全部改用。
//
// 本文件验**行为面**：删表注入后响应体不得出现驱动/ORM 原文，且文案是固定文案（不带表名）。
// 形状面（其余同族端点必须走 ServerErrorCause）由 server_error_text_guard_test.go 的 AST 守卫守。
package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

// serverErrorEnv 真路由 + 三种角色 token（学员 / 招聘者 / 管理员）。
type serverErrorEnv struct {
	db                       *gorm.DB
	r                        *gin.Engine
	studentTok, recruiterTok string
	adminTok                 string
}

func newServerErrorEnv(t *testing.T) *serverErrorEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey: "server-error-text-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := NewRouter(newContractDeps(t, db, cfg))

	hashed, err := service.HashPassword("seedpass123")
	if err != nil {
		t.Fatalf("哈希种子口令失败: %v", err)
	}
	stu := testutil.SeedStudent(t, db, "srv_err_stu", hashed)
	rec := testutil.SeedRecruiter(t, db, "srv_err_rec", hashed)
	adm := testutil.SeedAdmin(t, db, "srv_err_admin", hashed)

	issue := func(id int, account, role string) string {
		tok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name}).Issue(id, account, role)
		if err != nil {
			t.Fatalf("签发 token 失败: %v", err)
		}
		return tok
	}
	return &serverErrorEnv{
		db:           db,
		r:            r,
		studentTok:   issue(stu.ID, stu.Account, "hrwai_user"),
		recruiterTok: issue(rec.ID, rec.Username, service.RecruiterRole),
		adminTok:     issue(adm.AdminID, adm.Username, "admin"),
	}
}

// TestServerErrorTextNoDriverLeak 删表注入 ⇒ 500 且响应体里没有驱动原文、没有表名。
func TestServerErrorTextNoDriverLeak(t *testing.T) {
	cases := []struct {
		name   string
		drop   string
		method string
		path   string
		tok    func(e *serverErrorEnv) string
	}{
		{
			name: "GET /resume/contact-requests（学员侧，contact.go ListForStudent）",
			drop: "contact_requests", method: http.MethodGet, path: "/api/resume/contact-requests",
			tok: func(e *serverErrorEnv) string { return e.studentTok },
		},
		{
			name: "GET /recruit/contact-requests（招聘侧，contact.go ListForRecruiter）",
			drop: "contact_requests", method: http.MethodGet, path: "/api/recruit/contact-requests",
			tok: func(e *serverErrorEnv) string { return e.recruiterTok },
		},
		{
			name: "GET /admin/recruiters（admin_recruiter.go List）",
			drop: "recruiter_users", method: http.MethodGet, path: "/api/admin/recruiters",
			tok: func(e *serverErrorEnv) string { return e.adminTok },
		},
		{
			name: "GET /resume/view-stats（resume_view.go StudentViewStats）",
			drop: "recruit_resume_views", method: http.MethodGet, path: "/api/resume/view-stats",
			tok: func(e *serverErrorEnv) string { return e.studentTok },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newServerErrorEnv(t)
			if err := env.db.Exec("DROP TABLE " + tc.drop).Error; err != nil {
				t.Fatalf("注入故障（删 %s 表）失败: %v", tc.drop, err)
			}
			rec := doWithToken(t, env.r, tc.tok(env), tc.method, tc.path, nil)
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("删表后应回 500，实得 %d，body=%s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if leakyDriverText(body) {
				t.Fatalf("驱动原文外发（ADR-0064 决策 9）：%s", body)
			}
			if strings.Contains(body, tc.drop) {
				t.Fatalf("响应体带表名（%s）：%s", tc.drop, body)
			}
		})
	}
}
