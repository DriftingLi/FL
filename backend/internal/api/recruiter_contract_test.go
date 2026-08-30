// 契约测试 #370：企业招聘者账号体系（第四角色 + cookie 隔离）。
//
// 守的三条硬边界：
//   - 独立表 recruiter_users，不进 hrwai_users；
//   - 登录走共享骨架，仅新增查表分支与 status 校验；
//   - cookie 必须与学员侧隔离：recruiter_token host-only，hrwai_token 父域共享。
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

// recruiterCreateResp 管理端创建招聘者响应（只取关心的字段）。
type recruiterCreateResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		ID            int    `json:"id"`
		Username      string `json:"username"`
		CompanyName   string `json:"company_name"`
		CreditCode    string `json:"credit_code"`
		BusinessScope string `json:"business_scope"`
		ContactName   string `json:"contact_name"`
		ContactPhone  string `json:"contact_phone"`
		ContactEmail  string `json:"contact_email"`
		Status        int16  `json:"status"`
	} `json:"data"`
}

// loginResp 登录响应（LoginResult）。
type loginResp struct {
	Code    int                 `json:"code"`
	Message string              `json:"message"`
	Data    service.LoginResult `json:"data"`
}

func TestRecruiterContract_FullFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	adminPwd, _ := service.HashPassword("admin123")
	admin := testutil.SeedAdmin(t, db, "admin1", adminPwd)
	studentPwd, _ := service.HashPassword("student123")
	_ = testutil.SeedStudent(t, db, "stu1", studentPwd)

	cfg := &config.Config{
		JWTSecretKey:          "contract-test-secret",
		JWTExpiresHours:       2,
		JWTRefreshExpiresDays: 7,
		AuthCookie:            config.AuthCookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: false},
		RecruiterCookie:       config.RecruiterCookieConfig{Name: "recruiter_token", Domain: "", Secure: false},
	}
	r := NewRouter(newContractDeps(t, db, cfg))

	adminSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name, Domain: cfg.AuthCookie.Domain, Secure: cfg.AuthCookie.Secure})
	adminToken, _ := adminSess.Issue(admin.AdminID, admin.Username, "admin")

	// 1. 缺字段应被拒
	missingBody := map[string]any{
		"username":    "recruit1",
		"password":    "recruit123",
		"credit_code": "91110000MA12345678", "business_scope": "叉车维修", "contact_name": "张三", "contact_phone": "13800000001", "contact_email": "zhang@example.com",
	}
	rec := doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", missingBody)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("缺字段建号应 400, 实际 %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "企业名称") {
		t.Fatalf("缺字段错误文案应提及企业名称, 实际 %s", rec.Body.String())
	}

	// 正常创建
	goodBody := map[string]any{
		"username": "recruit1", "password": "recruit123", "company_name": "测试企业", "credit_code": "91110000MA12345678", "business_scope": "叉车维修", "contact_name": "张三", "contact_phone": "13800000001", "contact_email": "zhang@example.com",
	}
	rec = doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", goodBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("建号应 201, 实际 %d body=%s", rec.Code, rec.Body.String())
	}
	var created recruiterCreateResp
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("解析建号响应失败: %v", err)
	}
	if created.Data.Username != "recruit1" || created.Data.CompanyName != "测试企业" {
		t.Fatalf("建号返回数据异常: %+v", created.Data)
	}
	recruiterID := created.Data.ID

	var hrwaiCount int64
	db.Model(&model.HrwaiUser{}).Where("account = ? OR username = ?", "recruit1", "recruit1").Count(&hrwaiCount)
	if hrwaiCount != 0 {
		t.Fatalf("招聘者不应出现在 hrwai_users, count=%d", hrwaiCount)
	}
	var recCount int64
	db.Model(&model.RecruiterUser{}).Where("username = ?", "recruit1").Count(&recCount)
	if recCount != 1 {
		t.Fatalf("招聘者应在 recruiter_users 中, count=%d", recCount)
	}

	// 2. 招聘者登录
	loginBody := map[string]any{"username": "recruit1", "password": "recruit123"}
	rec = doJSON(t, r, http.MethodPost, "/api/auth/recruiter-login", loginBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("招聘者登录应 200, 实际 %d body=%s", rec.Code, rec.Body.String())
	}
	var lresp loginResp
	if err := json.Unmarshal(rec.Body.Bytes(), &lresp); err != nil {
		t.Fatalf("解析登录响应失败: %v", err)
	}
	if lresp.Data.Role != "recruiter" {
		t.Fatalf("招聘者登录角色应为 recruiter, 实际 %q", lresp.Data.Role)
	}
	recruiterToken := lresp.Data.Token
	cookies := rec.Result().Cookies()
	var foundRecruiter, foundHrwai bool
	for _, ck := range cookies {
		if ck.Name == "recruiter_token" {
			foundRecruiter = true
			if ck.Domain != "" {
				t.Fatalf("recruiter_token 应 host-only (Domain == \"\"), 实际 Domain=%q", ck.Domain)
			}
			if !ck.HttpOnly {
				t.Fatalf("recruiter_token 必须 HttpOnly")
			}
		}
		if ck.Name == "hrwai_token" {
			foundHrwai = true
		}
	}
	if !foundRecruiter {
		t.Fatalf("招聘者登录应写 recruiter_token cookie, 实际 cookies=%v", cookies)
	}
	if foundHrwai {
		t.Fatalf("招聘者登录不应写 hrwai_token cookie")
	}

	// 学员登录应写 hrwai_token
	hrwaiLoginBody := map[string]any{"username": "acct_stu1", "password": "student123"}
	recHr := doJSON(t, r, http.MethodPost, "/api/auth/login", hrwaiLoginBody)
	if recHr.Code != http.StatusOK {
		t.Fatalf("学员登录应 200, 实际 %d body=%s", recHr.Code, recHr.Body.String())
	}
	var hrLogin loginResp
	if err := json.Unmarshal(recHr.Body.Bytes(), &hrLogin); err != nil {
		t.Fatalf("解析学员登录失败: %v", err)
	}
	hrCookies := recHr.Result().Cookies()
	var hrFound bool
	for _, ck := range hrCookies {
		if ck.Name == "hrwai_token" {
			hrFound = true
			if ck.Domain != "example.com" {
				t.Fatalf("hrwai_token Domain 应为 example.com, 实际 %q", ck.Domain)
			}
		}
		if ck.Name == "recruiter_token" {
			t.Fatalf("学员登录不应写 recruiter_token")
		}
	}
	if !hrFound {
		t.Fatalf("学员登录应写 hrwai_token")
	}
	studentToken := hrLogin.Data.Token

	// 3. 角色隔离
	rec = doWithToken(t, r, recruiterToken, http.MethodGet, "/api/forum/topics", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("招聘者访问学员侧论坛应 403, 实际 %d body=%s", rec.Code, rec.Body.String())
	}
	rec = doWithToken(t, r, studentToken, http.MethodGet, "/api/recruit/resumes", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("学员访问招聘侧应 403, 实际 %d body=%s", rec.Code, rec.Body.String())
	}
	rec = doWithToken(t, r, recruiterToken, http.MethodGet, "/api/recruit/resumes", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("招聘者访问招聘侧应 200, 实际 %d body=%s", rec.Code, rec.Body.String())
	}
	var recruitList struct {
		Code int `json:"code"`
		Data struct {
			Items []any `json:"items"`
			Total int   `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &recruitList); err != nil {
		t.Fatalf("解析招聘列表失败: %v", err)
	}
	if len(recruitList.Data.Items) != 0 || recruitList.Data.Total != 0 {
		t.Fatalf("招聘端空列表应为空, 实际 %+v", recruitList.Data)
	}
	rec = doWithoutToken(t, r, http.MethodGet, "/api/recruit/resumes")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("未登录访问招聘侧应 401, 实际 %d", rec.Code)
	}

	// 4. 禁用后登录被拒
	toggleRec := doWithToken(t, r, adminToken, http.MethodPut, "/api/admin/recruiters/"+strconv.Itoa(recruiterID)+"/status", nil)
	if toggleRec.Code != http.StatusOK {
		t.Fatalf("禁用应 200, 实际 %d body=%s", toggleRec.Code, toggleRec.Body.String())
	}
	rec = doJSON(t, r, http.MethodPost, "/api/auth/recruiter-login", loginBody)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("禁用后登录应 400, 实际 %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "禁用") {
		t.Fatalf("禁用后错误文案应提及禁用, 实际 %s", rec.Body.String())
	}
}

func TestRecruiterCookieIsolation_HostOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey:    "test-secret",
		AuthCookie:      config.AuthCookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: false},
		RecruiterCookie: config.RecruiterCookieConfig{Name: "recruiter_token", Domain: "", Secure: false},
	}
	r := NewRouter(newContractDeps(t, db, cfg))
	adminPwd, _ := service.HashPassword("admin123")
	admin := testutil.SeedAdmin(t, db, "adminA", adminPwd)
	adminSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name, Domain: cfg.AuthCookie.Domain, Secure: false})
	adminToken, _ := adminSess.Issue(admin.AdminID, admin.Username, "admin")
	body := map[string]any{
		"username": "recCookie", "password": "pass1234", "company_name": "C", "credit_code": "CODE1", "business_scope": "BS", "contact_name": "N", "contact_phone": "13800000002", "contact_email": "c@example.com",
	}
	rec := doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("建号失败 %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, r, http.MethodPost, "/api/auth/recruiter-login", map[string]any{"username": "recCookie", "password": "pass1234"})
	if rec.Code != http.StatusOK {
		t.Fatalf("login fail %d", rec.Code)
	}
	var found bool
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == "recruiter_token" {
			found = true
			if ck.Domain != "" {
				t.Fatalf("recruiter cookie should be host-only, got Domain %q", ck.Domain)
			}
		}
	}
	if !found {
		t.Fatalf("recruiter_token not found")
	}
}

func doWithToken(t *testing.T, r *gin.Engine, token, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		req, _ = http.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, path, nil)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func doJSON(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func doWithoutToken(t *testing.T, r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
